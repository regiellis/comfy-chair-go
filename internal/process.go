package internal

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

// ProcessStatus represents cached process status information
type ProcessStatus struct {
	IsRunning bool
	LastCheck time.Time
}

// ProcessCache manages cached process status to avoid frequent system calls
type ProcessCache struct {
	cache       map[int]ProcessStatus
	mu          sync.RWMutex
	lastCleanup time.Time
}

// Global process cache instance
var ProcCache = &ProcessCache{
	cache: make(map[int]ProcessStatus),
}

// ExecuteCommand creates and returns an exec.Command with proper stdout/stderr handling
// For foreground processes, it waits for completion. For background processes, it starts and returns immediately.
func ExecuteCommand(commandName string, args []string, workDir string, logFilePath string, inBackground bool) (*os.Process, error) {
	// Check if in dry-run mode
	if IsDryRun() {
		mode := "foreground"
		if inBackground {
			mode = "background"
		}
		DryRunLog("Would execute command: %s %s (in %s mode, workdir: %s)", commandName, strings.Join(args, " "), mode, workDir)
		return nil, nil
	}
	// Basic validation: command name should not be empty and should not contain path separators unless it's an absolute path
	if commandName == "" {
		return nil, fmt.Errorf("command name cannot be empty")
	}
	
	// Validate command name to prevent command injection
	if err := validateCommand(commandName); err != nil {
		return nil, fmt.Errorf("invalid command: %w", err)
	}
	
	// Validate all arguments
	for i, arg := range args {
		if err := validateArgument(arg); err != nil {
			return nil, fmt.Errorf("invalid argument at position %d: %w", i, err)
		}
	}
	
	// Validate working directory if provided
	if workDir != "" {
		cleanWorkDir := filepath.Clean(ExpandUserPath(workDir))
		if cleanWorkDir == "" {
			return nil, fmt.Errorf("invalid working directory path")
		}
		// Ensure the work directory exists or is creatable
		if _, err := os.Stat(cleanWorkDir); err != nil && !os.IsNotExist(err) {
			return nil, fmt.Errorf("working directory validation failed: %w", err)
		}
	}

	cmd := exec.Command(commandName, args...)
	if workDir != "" {
		cmd.Dir = ExpandUserPath(workDir)
	}

	// Handle logging and I/O redirection
	if logFilePath != "" && inBackground {
		expandedLogPath := ExpandUserPath(logFilePath)
		if expandedLogPath == "" {
			return nil, fmt.Errorf("invalid log file path")
		}
		logFile, err := os.OpenFile(expandedLogPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return nil, fmt.Errorf("failed to open log file %s: %w", logFilePath, err)
		}
		cmd.Stdout = logFile
		cmd.Stderr = logFile
		cmd.ExtraFiles = []*os.File{logFile}
	} else {
		// For foreground processes, connect to stdin to allow interaction
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}

	if inBackground {
		// Background mode: start process and return immediately
		if err := cmd.Start(); err != nil {
			return nil, fmt.Errorf("command '%s %s' execution failed: %w", commandName, strings.Join(args, " "), err)
		}
		return cmd.Process, nil
	} else {
		// Foreground mode: start process and wait for completion
		if err := cmd.Start(); err != nil {
			return nil, fmt.Errorf("command '%s %s' execution failed: %w", commandName, strings.Join(args, " "), err)
		}
		
		// Wait for the process to complete (this allows Ctrl+C to work properly)
		waitErr := cmd.Wait()
		// Return the process even if Wait failed - the process did run
		return cmd.Process, waitErr
	}
}

// ReadPID reads the process ID from the pidFile.
func ReadPID(pidFile string) (int, error) {
	pf := NewPIDFile(pidFile)
	return pf.Read()
}

// CleanupPIDFile removes the pidFile.
func CleanupPIDFile(pidFile string) {
	pf := NewPIDFile(pidFile)
	if err := pf.Remove(); err != nil {
		Log.Warning("Failed to remove PID file %s: %v", pidFile, err)
	}
}

// cleanupStaleEntries removes stale cache entries
func (pc *ProcessCache) cleanupStaleEntries() {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	
	now := time.Now()
	// Clean up every 5 minutes
	if time.Since(pc.lastCleanup) < 5*time.Minute {
		return
	}
	
	// Remove entries older than 30 seconds
	for pid, status := range pc.cache {
		if time.Since(status.LastCheck) > 30*time.Second {
			delete(pc.cache, pid)
		}
	}
	pc.lastCleanup = now
}

// getCachedStatus retrieves cached process status
func (pc *ProcessCache) getCachedStatus(pid int) (ProcessStatus, bool) {
	pc.mu.RLock()
	defer pc.mu.RUnlock()
	
	status, exists := pc.cache[pid]
	return status, exists
}

// updateCache updates the process status cache
func (pc *ProcessCache) updateCache(pid int, isRunning bool) {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	
	pc.cache[pid] = ProcessStatus{
		IsRunning: isRunning,
		LastCheck: time.Now(),
	}
}

// isProcessRunningReal performs the actual system check without caching
func isProcessRunningReal(pid int) bool {
	if pid == 0 {
		return false
	}
	process, err := os.FindProcess(pid)
	if err != nil { // Should not happen on POSIX if pid != 0, can happen on Windows.
		return false
	}

	if runtime.GOOS == "windows" {
		// On Windows, FindProcess always returns a Process object.
		// Sending signal 0 doesn't work. tasklist is more reliable.
		cmd := exec.Command("tasklist", "/FI", fmt.Sprintf("PID eq %d", pid), "/NH", "/FO", "CSV") // No Header, CSV format
		output, err := cmd.Output()
		if err != nil { // tasklist command failed or process not found (often an error)
			return false
		}
		
		// Parse CSV output to check for exact PID match
		// CSV format: "process.exe","PID","Session Name","Session#","Mem Usage"
		lines := strings.Split(string(output), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			
			// Split CSV fields
			fields := strings.Split(line, ",")
			if len(fields) >= 2 {
				// Extract PID field (second field) and remove quotes
				pidField := strings.Trim(fields[1], "\"")
				pidField = strings.TrimSpace(pidField)
				
				// Check for exact match
				if pidField == strconv.Itoa(pid) {
					return true
				}
			}
		}
		return false
	}

	// For POSIX systems, send signal 0 to check if the process exists and is owned by us.
	if err := process.Signal(syscall.Signal(0)); err != nil {
		return false // Process doesn't exist or we don't have permission
	}
	return true
}

// IsProcessRunning checks if a process with the given PID is currently running (with caching).
func IsProcessRunning(pid int) bool {
	if pid == 0 {
		return false
	}

	// Clean up stale entries periodically
	ProcCache.cleanupStaleEntries()

	// Check cache first
	if status, exists := ProcCache.getCachedStatus(pid); exists {
		// Use cached result if it's fresh (within 5 seconds)
		if time.Since(status.LastCheck) < 5*time.Second {
			return status.IsRunning
		}
	}

	// Cache miss or stale - check for real and update cache
	isRunning := isProcessRunningReal(pid)
	ProcCache.updateCache(pid, isRunning)
	return isRunning
}

// GetRunningPID reads PID from file and checks if the process is running.
func GetRunningPID(pidFile string) (pid int, isRunning bool) {
	pf := NewPIDFile(pidFile)
	return pf.GetRunningPID()
}

// WritePID writes the PID to a pidFile
func WritePID(pid int, pidFile string) error {
	pf := NewPIDFile(pidFile)
	return pf.Write(pid)
}

// validateCommand validates a command name to prevent command injection
func validateCommand(cmd string) error {
	// Allow absolute paths or simple command names
	if filepath.IsAbs(cmd) {
		// For absolute paths, verify the file exists and is executable
		info, err := os.Stat(cmd)
		if err != nil {
			return fmt.Errorf("command not found: %s", cmd)
		}
		if info.IsDir() {
			return fmt.Errorf("command path is a directory: %s", cmd)
		}
		// Check if executable (Unix-style permissions check)
		if runtime.GOOS != "windows" && info.Mode().Perm()&0111 == 0 {
			return fmt.Errorf("file is not executable: %s", cmd)
		}
	} else {
		// For relative commands, ensure they don't contain dangerous characters
		// that could be used for command injection
		dangerousChars := []string{";", "&", "|", "`", "$", "(", ")", "{", "}", "<", ">", "\n", "\r"}
		for _, char := range dangerousChars {
			if strings.Contains(cmd, char) {
				return fmt.Errorf("command contains dangerous character: %s", char)
			}
		}
		
		// Don't allow path traversal in command names
		if strings.Contains(cmd, "..") {
			return fmt.Errorf("command contains path traversal sequence")
		}
	}
	
	return nil
}

// validateArgument validates command arguments to prevent injection
func validateArgument(arg string) error {
	// Arguments can contain most characters, but we should prevent
	// null bytes and newlines that could break command parsing
	if strings.Contains(arg, "\x00") {
		return fmt.Errorf("argument contains null byte")
	}
	
	// Very long arguments could indicate an attack
	if len(arg) > 8192 {
		return fmt.Errorf("argument too long (max 8192 characters)")
	}
	
	return nil
}