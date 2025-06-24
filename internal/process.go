package internal

import (
	"errors"
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

// processStatus represents cached process status information
type processStatus struct {
	isRunning bool
	lastCheck time.Time
}

// processCache manages cached process status to avoid frequent system calls
type processCache struct {
	cache       map[int]processStatus
	mu          sync.RWMutex
	lastCleanup time.Time
}

// Global process cache instance
var procCache = &processCache{
	cache: make(map[int]processStatus),
}

// ExecuteCommand creates and returns an exec.Command with proper stdout/stderr handling
func ExecuteCommand(commandName string, args []string, workDir string, logFilePath string, inBackground bool) (*os.Process, error) {
	// Basic validation: command name should not be empty and should not contain path separators unless it's an absolute path
	if commandName == "" {
		return nil, fmt.Errorf("command name cannot be empty")
	}
	
	// Validate working directory if provided
	if workDir != "" {
		cleanWorkDir := filepath.Clean(ExpandUserPath(workDir))
		// Ensure the work directory exists or is creatable
		if _, err := os.Stat(cleanWorkDir); err != nil && !os.IsNotExist(err) {
			return nil, fmt.Errorf("working directory validation failed: %w", err)
		}
	}

	cmd := exec.Command(commandName, args...)
	if workDir != "" {
		cmd.Dir = ExpandUserPath(workDir)
	}

	// Handle logging
	if logFilePath != "" && inBackground {
		logFile, err := os.OpenFile(ExpandUserPath(logFilePath), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			return nil, fmt.Errorf("failed to open log file %s: %w", logFilePath, err)
		}
		cmd.Stdout = logFile
		cmd.Stderr = logFile
		cmd.ExtraFiles = []*os.File{logFile}
	} else {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("command '%s %s' execution failed: %w", commandName, strings.Join(args, " "), err)
	}
	return cmd.Process, nil
}

// ReadPID reads the process ID from the pidFile.
func ReadPID(pidFile string) (int, error) {
	if _, err := os.Stat(ExpandUserPath(pidFile)); os.IsNotExist(err) {
		return 0, os.ErrNotExist // Return specific error
	}
	data, err := os.ReadFile(ExpandUserPath(pidFile))
	if err != nil {
		return 0, err
	}
	if len(data) == 0 {
		return 0, fmt.Errorf("pid file is empty: %s", pidFile)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0, fmt.Errorf("invalid PID in file %s: %w", pidFile, err)
	}
	return pid, nil
}

// CleanupPIDFile removes the pidFile.
func CleanupPIDFile(pidFile string) {
	if err := os.Remove(ExpandUserPath(pidFile)); err != nil && !os.IsNotExist(err) {
		fmt.Println(WarningStyle.Render(fmt.Sprintf("Warning: Failed to remove PID file %s: %v", pidFile, err)))
	}
}

// cleanupStaleEntries removes stale cache entries
func (pc *processCache) cleanupStaleEntries() {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	
	now := time.Now()
	// Clean up every 5 minutes
	if time.Since(pc.lastCleanup) < 5*time.Minute {
		return
	}
	
	// Remove entries older than 30 seconds
	for pid, status := range pc.cache {
		if time.Since(status.lastCheck) > 30*time.Second {
			delete(pc.cache, pid)
		}
	}
	pc.lastCleanup = now
}

// getCachedStatus retrieves cached process status
func (pc *processCache) getCachedStatus(pid int) (processStatus, bool) {
	pc.mu.RLock()
	defer pc.mu.RUnlock()
	
	status, exists := pc.cache[pid]
	return status, exists
}

// updateCache updates the process status cache
func (pc *processCache) updateCache(pid int, isRunning bool) {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	
	pc.cache[pid] = processStatus{
		isRunning: isRunning,
		lastCheck: time.Now(),
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
		cmd := exec.Command("tasklist", "/FI", fmt.Sprintf("PID eq %d", pid), "/NH") // No Header
		output, err := cmd.Output()
		if err != nil { // tasklist command failed or process not found (often an error)
			return false
		}
		return strings.Contains(strings.ToLower(string(output)), strings.ToLower(strconv.Itoa(pid))) // Case-insensitive check for PID
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
	procCache.cleanupStaleEntries()

	// Check cache first
	if status, exists := procCache.getCachedStatus(pid); exists {
		// Use cached result if it's fresh (within 5 seconds)
		if time.Since(status.lastCheck) < 5*time.Second {
			return status.isRunning
		}
	}

	// Cache miss or stale - check for real and update cache
	isRunning := isProcessRunningReal(pid)
	procCache.updateCache(pid, isRunning)
	return isRunning
}

// GetRunningPID reads PID from file and checks if the process is running.
func GetRunningPID(pidFile string) (pid int, isRunning bool) {
	pidRead, err := ReadPID(pidFile)
	if err != nil {
		// os.ErrNotExist is normal if ComfyUI not started via this tool's background mode.
		// Other errors (permission, corrupted file) are warnings.
		if !errors.Is(err, os.ErrNotExist) {
			fmt.Println(WarningStyle.Render(fmt.Sprintf("Warning: Could not read PID file %s: %v", pidFile, err)))
		}
		return 0, false
	}
	if IsProcessRunning(pidRead) {
		return pidRead, true
	}
	return pidRead, false // PID read, but process not running (stale PID)
}

// GetRunningPIDForEnv reads PID from a given pidFile and checks if the process is running.
func GetRunningPIDForEnv(pidFile string) (pid int, isRunning bool) {
	pid, _ = ReadPIDForEnv(pidFile)
	isRunning = IsProcessRunning(pid)
	return
}

// ReadPIDForEnv reads the PID from a given pidFile.
func ReadPIDForEnv(pidFile string) (int, error) {
	f, err := os.Open(ExpandUserPath(pidFile))
	if err != nil {
		return 0, err
	}
	defer f.Close()
	var pid int
	fmt.Fscanf(f, "%d", &pid)
	return pid, nil
}

// WritePIDForEnv writes the PID to a given pidFile.
func WritePIDForEnv(pid int, pidFile string) error {
	f, err := os.Create(ExpandUserPath(pidFile))
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = fmt.Fprintf(f, "%d", pid)
	return err
}