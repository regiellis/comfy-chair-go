package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/regiellis/comfyui-chair-go/internal"
)

// NOTE: includedDirs is now sourced from comfy-installs.json (per environment) via main.go, not from .env.
func reloadComfyUI(watchDir string, debounceSeconds int, exts []string, includedDirs []string) {
	logFile := appPaths.LogFile
	if logFile == "" {
		fmt.Println(internal.ErrorStyle.Render("Log file path is not set."))
		return
	}

	// Start tailing the log file in a goroutine
	tailDone := make(chan struct{})
	go func() {
		file, err := os.Open(logFile)
		if err != nil {
			fmt.Println(internal.ErrorStyle.Render(fmt.Sprintf("Failed to open log file: %v", err)))
			return
		}
		defer file.Close()
		// Seek to the end of the file initially to only show new logs
		_, err = file.Seek(0, io.SeekEnd)
		if err != nil {
			fmt.Println(internal.ErrorStyle.Render(fmt.Sprintf("Failed to seek to end of log file: %v", err)))
			return
		}
		reader := bufio.NewReader(file)
		for {
			select {
			case <-tailDone:
				return
			default:
				line, err := reader.ReadString('\n')
				if err != nil {
					if err == io.EOF {
						time.Sleep(500 * time.Millisecond)
						continue
					}
					return
				}
				fmt.Print(line)
			}
		}
	}()

	pid, _ := readPID()
	if !isProcessRunning(pid) {
		fmt.Println(internal.SuccessStyle.Render("Starting ComfyUI..."))
		startComfyUI(true)
		// Give it a moment to start
		time.Sleep(2 * time.Second)
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(internal.ErrorStyle.Render(fmt.Sprintf("Failed to create watcher: %v", err)))
	}
	defer watcher.Close()

	// Add only includedDirs to the watcher (opt-in)
	for _, dir := range includedDirs {
		watchPath := filepath.Join(watchDir, dir)
		info, err := os.Lstat(watchPath)
		if err != nil {
			continue
		}
		// If it's a symlink, resolve it and ensure it's a directory
		if info.Mode()&os.ModeSymlink != 0 {
			realPath, err := filepath.EvalSymlinks(watchPath)
			if err != nil {
				fmt.Println(internal.WarningStyle.Render(fmt.Sprintf("Failed to resolve symlink %s: %v", watchPath, err)))
				continue
			}
			info, err = os.Stat(realPath)
			if err != nil || !info.IsDir() {
				fmt.Println(internal.WarningStyle.Render(fmt.Sprintf("Symlink %s does not resolve to a directory.", watchPath)))
				continue
			}
			watchPath = realPath
		}
		if !info.IsDir() {
			continue
		}
		err = watcher.Add(watchPath)
		if err != nil {
			fmt.Println(internal.WarningStyle.Render(fmt.Sprintf("Failed to watch directory %s: %v", watchPath, err)))
		}
	}

	fmt.Println(internal.SuccessStyle.Render(fmt.Sprintf("Watching %s for changes...", watchDir)))
	lastRestartTime := time.Now()
	debounceDuration := time.Duration(debounceSeconds) * time.Second

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	
	// Create a done channel for proper shutdown coordination
	done := make(chan struct{})

	go func() {
		<-sigs
		fmt.Println("\nReceived signal, exiting reload watcher...")
		close(tailDone)
		close(done)
	}()

	defer func() {
		watcher.Close()
		fmt.Println(internal.InfoStyle.Render("File watcher cleanup completed"))
	}()

	for {
		select {
		case <-done:
			fmt.Println(internal.InfoStyle.Render("Shutdown signal received, stopping file watcher..."))
			return
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			fmt.Printf("[DEBUG] Event: %s, Op: %s\n", event.Name, event.Op)
			// Only trigger reload if the event is in an includedDir
			inIncluded := false
			for _, dir := range includedDirs {
				if strings.Contains(filepath.ToSlash(event.Name), "/"+dir+"/") || strings.HasPrefix(filepath.ToSlash(event.Name), filepath.ToSlash(filepath.Join(watchDir, dir))) {
					inIncluded = true
					break
				}
			}
			if !inIncluded {
				fmt.Printf("[DEBUG] Ignored (not in includedDirs): %s\n", event.Name)
				continue
			}
			matched := matchesExtension(event.Name, exts)
			fmt.Printf("[DEBUG] matchesExtension(%s, %v) = %v\n", event.Name, exts, matched)
			if (event.Op.Has(fsnotify.Write) || event.Op.Has(fsnotify.Create)) && matched {
				if time.Since(lastRestartTime) > debounceDuration {
					fmt.Println(internal.WarningStyle.Render(fmt.Sprintf("Changes detected in %s. Restarting ComfyUI...", event.Name)))
					restartComfyUIProcess()
					lastRestartTime = time.Now()
				} else {
					fmt.Println(internal.InfoStyle.Render(fmt.Sprintf("Change detected in %s, but debouncing...", event.Name)))
				}
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			log.Println(internal.ErrorStyle.Render(fmt.Sprintf("Watcher error: %v", err)))
		}
	}
}

func matchesExtension(filePath string, exts []string) bool {
	for _, ext := range exts {
		if strings.HasSuffix(strings.ToLower(filePath), strings.ToLower(ext)) {
			return true
		}
	}
	return false
}

func restartComfyUIProcess() {
	pid, isRunning := getRunningPID()
	if isRunning {
		process, err := os.FindProcess(pid)
		if err == nil {
			// Try graceful stop first (SIGTERM)
			process.Signal(syscall.SIGTERM)
			waited := 0
			for waited < 20 { // Wait up to 2 seconds (20 * 100ms)
				time.Sleep(100 * time.Millisecond)
				if !isProcessRunning(pid) {
					break
				}
				waited++
			}
			if isProcessRunning(pid) {
				if isWindows() {
					process.Kill()
				} else {
					process.Signal(syscall.SIGKILL)
				}
				fmt.Println(internal.WarningStyle.Render(fmt.Sprintf("ComfyUI (PID: %d) force killed for reload.", pid)))
				// Short delay to ensure process is fully terminated
				for i := 0; i < 10; i++ {
					time.Sleep(100 * time.Millisecond)
					if !isProcessRunning(pid) {
						break
					}
				}
			} else {
				fmt.Println(internal.InfoStyle.Render(fmt.Sprintf("ComfyUI (PID: %d) stopped gracefully.", pid)))
			}
			// Wait for ComfyUI to fully stop before continuing
			waitForComfyUIStop(pid)
		} else {
			fmt.Println(internal.WarningStyle.Render(fmt.Sprintf("Could not find process to kill (PID: %d): %v", pid, err)))
		}
		cleanupPIDFile()
	} else if pid != 0 { // Stale PID file
		fmt.Println(internal.InfoStyle.Render(fmt.Sprintf("Removing stale PID file for PID %d.", pid)))
		cleanupPIDFile()
	}
	startComfyUI(true)
}

func isWindows() bool {
	return strings.Contains(strings.ToLower(runtime.GOOS), "windows")
}
