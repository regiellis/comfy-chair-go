package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/regiellis/comfyui-chair-go/internal"
)

// Common patterns to ignore during file watching
var commonIgnorePatterns = []*regexp.Regexp{
	regexp.MustCompile(`__pycache__`),
	regexp.MustCompile(`\.pyc$`),
	regexp.MustCompile(`\.pyo$`),
	regexp.MustCompile(`\.pyd$`),
	regexp.MustCompile(`\.so$`),
	regexp.MustCompile(`\.egg-info`),
	regexp.MustCompile(`\.git`),
	regexp.MustCompile(`\.DS_Store$`),
	regexp.MustCompile(`Thumbs\.db$`),
	regexp.MustCompile(`\.swp$`),
	regexp.MustCompile(`\.tmp$`),
	regexp.MustCompile(`\.log$`),
	regexp.MustCompile(`\.pid$`),
	regexp.MustCompile(`node_modules`),
	regexp.MustCompile(`\.vscode`),
	regexp.MustCompile(`\.idea`),
	regexp.MustCompile(`dist/`),
	regexp.MustCompile(`build/`),
}

// shouldIgnorePath checks if a path should be ignored based on common patterns and .gitignore files
func shouldIgnorePath(path string, basePath string) bool {
	// Get relative path for pattern matching
	relPath, err := filepath.Rel(basePath, path)
	if err != nil {
		relPath = path
	}
	
	// Check against common ignore patterns
	for _, pattern := range commonIgnorePatterns {
		if pattern.MatchString(relPath) || pattern.MatchString(filepath.Base(path)) {
			return true
		}
	}
	
	// Check for .gitignore in the directory and parent directories
	return checkGitignore(path, basePath)
}

// checkGitignore reads .gitignore files and checks if path should be ignored
func checkGitignore(targetPath string, basePath string) bool {
	// Start from the target path directory and walk up to basePath
	currentDir := filepath.Dir(targetPath)
	for {
		gitignorePath := filepath.Join(currentDir, ".gitignore")
		if _, err := os.Stat(gitignorePath); err == nil {
			if isIgnoredByGitignore(targetPath, gitignorePath, currentDir) {
				return true
			}
		}
		
		// Stop if we've reached the base path or root
		if currentDir == basePath || currentDir == filepath.Dir(currentDir) {
			break
		}
		currentDir = filepath.Dir(currentDir)
	}
	return false
}

// isIgnoredByGitignore checks if a path matches patterns in a .gitignore file
func isIgnoredByGitignore(targetPath string, gitignorePath string, gitignoreDir string) bool {
	file, err := os.Open(gitignorePath)
	if err != nil {
		return false
	}
	defer file.Close()
	
	// Get relative path from gitignore directory
	relPath, err := filepath.Rel(gitignoreDir, targetPath)
	if err != nil {
		return false
	}
	
	// Convert to forward slashes for pattern matching
	relPath = filepath.ToSlash(relPath)
	
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		
		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		
		// Handle negation patterns (starting with !)
		isNegation := strings.HasPrefix(line, "!")
		if isNegation {
			line = line[1:]
		}
		
		// Convert gitignore pattern to regex
		pattern := gitignorePatternToRegex(line)
		matched, err := regexp.MatchString(pattern, relPath)
		if err == nil && matched {
			// If it's a negation pattern and matches, don't ignore
			if isNegation {
				return false
			}
			// Otherwise, ignore it
			return true
		}
	}
	return false
}

// gitignorePatternToRegex converts a gitignore pattern to a regular expression
func gitignorePatternToRegex(pattern string) string {
	// Escape special regex characters except * and ?
	pattern = regexp.MustCompile(`[.+^${}()|\[\]\\]`).ReplaceAllStringFunc(pattern, func(s string) string {
		return "\\" + s
	})
	
	// Convert gitignore wildcards to regex
	pattern = strings.ReplaceAll(pattern, "*", "[^/]*")
	pattern = strings.ReplaceAll(pattern, "?", "[^/]")
	
	// Handle directory patterns (ending with /)
	if strings.HasSuffix(pattern, "/") {
		pattern = pattern[:len(pattern)-1] + "(/.*)?$"
	} else {
		pattern = "^" + pattern + "$"
	}
	
	return pattern
}

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
	if !internal.IsProcessRunning(pid) {
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

	// Add only includedDirs to the watcher (opt-in), recursively
	dirsWatched := 0
	for _, dir := range includedDirs {
		watchPath := filepath.Join(watchDir, dir)
		info, err := os.Lstat(watchPath)
		if err != nil {
			fmt.Println(internal.WarningStyle.Render(fmt.Sprintf("Cannot access directory %s: %v", watchPath, err)))
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
			fmt.Println(internal.WarningStyle.Render(fmt.Sprintf("Path %s is not a directory, skipping", watchPath)))
			continue
		}
		
		// Recursively add all subdirectories to the watcher, respecting .gitignore
		err = filepath.Walk(watchPath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				fmt.Println(internal.WarningStyle.Render(fmt.Sprintf("Error walking directory %s: %v", path, err)))
				return nil // Continue walking despite errors
			}
			
			// Check if this path should be ignored
			if shouldIgnorePath(path, watchDir) {
				fmt.Println(internal.InfoStyle.Render(fmt.Sprintf("Skipping ignored path: %s", strings.TrimPrefix(path, watchDir+string(filepath.Separator)))))
				if info.IsDir() {
					return filepath.SkipDir // Skip entire directory
				}
				return nil
			}
			
			if info.IsDir() {
				err = watcher.Add(path)
				if err != nil {
					fmt.Println(internal.WarningStyle.Render(fmt.Sprintf("Failed to watch directory %s: %v", path, err)))
				} else {
					dirsWatched++
					fmt.Println(internal.InfoStyle.Render(fmt.Sprintf("Watching directory: %s", strings.TrimPrefix(path, watchDir+string(filepath.Separator)))))
				}
			}
			return nil
		})
		
		if err != nil {
			fmt.Println(internal.ErrorStyle.Render(fmt.Sprintf("Failed to setup recursive watching for %s: %v", watchPath, err)))
		}
	}
	
	fmt.Println(internal.SuccessStyle.Render(fmt.Sprintf("Total directories being watched: %d", dirsWatched)))

	fmt.Println(internal.SuccessStyle.Render(fmt.Sprintf("Watching %s for changes...", watchDir)))
	fmt.Println(internal.InfoStyle.Render(fmt.Sprintf("Watching file extensions: %v", exts)))
	fmt.Println(internal.InfoStyle.Render(fmt.Sprintf("Watching custom node directories: %v", includedDirs)))
	fmt.Println(internal.InfoStyle.Render(fmt.Sprintf("Debounce period: %d seconds", debounceSeconds)))
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
			
			// Check if this file should be ignored
			if shouldIgnorePath(event.Name, watchDir) {
				// Silently ignore files that match ignore patterns
				continue
			}
			
			// Log all file events for debugging
			fileName := filepath.Base(event.Name)
			relativePath := strings.TrimPrefix(event.Name, watchDir+string(filepath.Separator))
			fmt.Println(internal.InfoStyle.Render(fmt.Sprintf("File event: %s on %s (op: %s)", event.Op, relativePath, event.Op)))
			
			// Only trigger reload if the event is in an includedDir
			inIncluded := false
			for _, dir := range includedDirs {
				if strings.Contains(filepath.ToSlash(event.Name), "/"+dir+"/") || strings.HasPrefix(filepath.ToSlash(event.Name), filepath.ToSlash(filepath.Join(watchDir, dir))) {
					inIncluded = true
					break
				}
			}
			
			if !inIncluded {
				fmt.Println(internal.InfoStyle.Render(fmt.Sprintf("File change ignored - %s not in watched custom node directories", relativePath)))
				continue
			}
			
			matched := matchesExtension(event.Name, exts)
			fmt.Println(internal.InfoStyle.Render(fmt.Sprintf("Extension check: %s matches %v = %t", fileName, exts, matched)))
			
			if (event.Op.Has(fsnotify.Write) || event.Op.Has(fsnotify.Create)) && matched {
				if time.Since(lastRestartTime) > debounceDuration {
					fmt.Println(internal.WarningStyle.Render(fmt.Sprintf("Changes detected in %s. Restarting ComfyUI...", relativePath)))
					restartComfyUIProcess()
					lastRestartTime = time.Now()
				} else {
					fmt.Println(internal.InfoStyle.Render(fmt.Sprintf("Change detected in %s, but debouncing... (%.1fs remaining)", relativePath, debounceDuration.Seconds()-time.Since(lastRestartTime).Seconds())))
				}
			} else if !matched {
				fmt.Println(internal.InfoStyle.Render(fmt.Sprintf("File change ignored - %s does not match watched extensions %v", fileName, exts)))
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
				if !internal.IsProcessRunning(pid) {
					break
				}
				waited++
			}
			if internal.IsProcessRunning(pid) {
				if isWindows() {
					process.Kill()
				} else {
					process.Signal(syscall.SIGKILL)
				}
				fmt.Println(internal.WarningStyle.Render(fmt.Sprintf("ComfyUI (PID: %d) force killed for reload.", pid)))
				// Short delay to ensure process is fully terminated
				for i := 0; i < 10; i++ {
					time.Sleep(100 * time.Millisecond)
					if !internal.IsProcessRunning(pid) {
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
