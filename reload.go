package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"
)

func reloadComfyUI(watchDir string, debounceSeconds int, exts []string) {
	logFile := appPaths.logFile
	if logFile == "" {
		fmt.Println(errorStyle.Render("Log file path is not set."))
		return
	}

	// Start tailing the log file in a goroutine
	tailDone := make(chan struct{})
	go func() {
		file, err := os.Open(logFile)
		if err != nil {
			fmt.Println(errorStyle.Render(fmt.Sprintf("Failed to open log file: %v", err)))
			return
		}
		defer file.Close()
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
		fmt.Println(successStyle.Render("Starting ComfyUI..."))
		startComfyUI(true)
		// Give it a moment to start
		time.Sleep(2 * time.Second)
	}

	fmt.Println(successStyle.Render(fmt.Sprintf("Watching %s for changes...", watchDir)))
	lastRestart := time.Now().Add(-time.Duration(debounceSeconds) * time.Second)

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigs
		fmt.Println("\nReceived signal, exiting reload watcher...")
		close(tailDone)
		os.Exit(0)
	}()

	for {
		var latestMod time.Time
		var changedFile string
		filepath.Walk(watchDir, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}
			for _, ext := range exts {
				if strings.HasSuffix(path, ext) {
					if info.ModTime().After(latestMod) {
						latestMod = info.ModTime()
						changedFile = path
					}
				}
			}
			return nil
		})
		if !latestMod.IsZero() && latestMod.After(lastRestart) && time.Since(latestMod) > 0 {
			fmt.Println(warningStyle.Render(fmt.Sprintf("Changes detected in %s. Restarting ComfyUI...", changedFile)))
			pid, isRunning := getRunningPID()
			if isRunning {
				process, err := os.FindProcess(pid)
				if err == nil {
					// Try graceful stop first (SIGTERM)
					process.Signal(syscall.SIGTERM)
					waited := 0
					for waited < 20 {
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
						fmt.Println(warningStyle.Render(fmt.Sprintf("ComfyUI (PID: %d) force killed for reload.", pid)))
						for i := 0; i < 10; i++ {
							time.Sleep(100 * time.Millisecond)
							if !isProcessRunning(pid) {
								break
							}
						}
					} else {
						fmt.Println(infoStyle.Render(fmt.Sprintf("ComfyUI (PID: %d) stopped gracefully.", pid)))
					}
				} else {
					fmt.Println(warningStyle.Render(fmt.Sprintf("Could not find process to kill (PID: %d): %v", pid, err)))
				}
				cleanupPIDFile()
			} else if pid != 0 {
				cleanupPIDFile()
			}
			startComfyUI(true)
			lastRestart = time.Now()
		}
		time.Sleep(1 * time.Second)
	}
}

func isWindows() bool {
	return strings.Contains(strings.ToLower(runtime.GOOS), "windows")
}
