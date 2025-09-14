package internal

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// PIDFile provides unified PID file operations
type PIDFile struct {
	path string
}

// NewPIDFile creates a new PID file handler
func NewPIDFile(path string) *PIDFile {
	return &PIDFile{path: path}
}

// Read reads the process ID from the PID file
func (p *PIDFile) Read() (int, error) {
	expandedPath := ExpandUserPath(p.path)
	if expandedPath == "" {
		return 0, fmt.Errorf("invalid PID file path")
	}
	
	data, err := os.ReadFile(expandedPath)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, os.ErrNotExist
		}
		return 0, fmt.Errorf("failed to read PID file: %w", err)
	}
	
	if len(data) == 0 {
		return 0, fmt.Errorf("PID file is empty")
	}
	
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0, fmt.Errorf("invalid PID format: %w", err)
	}
	
	if pid <= 0 {
		return 0, fmt.Errorf("invalid PID value: %d", pid)
	}
	
	return pid, nil
}

// Write writes the process ID to the PID file
func (p *PIDFile) Write(pid int) error {
	if pid <= 0 {
		return fmt.Errorf("invalid PID value: %d", pid)
	}
	
	expandedPath := ExpandUserPath(p.path)
	if expandedPath == "" {
		return fmt.Errorf("invalid PID file path")
	}
	
	return DryRunExecute("Write PID %d to file %s", func() error {
		data := fmt.Sprintf("%d", pid)
		return os.WriteFile(expandedPath, []byte(data), 0644)
	}, pid, p.path)
}

// Remove removes the PID file
func (p *PIDFile) Remove() error {
	expandedPath := ExpandUserPath(p.path)
	if expandedPath == "" {
		return fmt.Errorf("invalid PID file path")
	}
	
	return DryRunExecute("Remove PID file %s", func() error {
		err := os.Remove(expandedPath)
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}, p.path)
}

// GetRunningPID reads PID and checks if the process is running
func (p *PIDFile) GetRunningPID() (pid int, isRunning bool) {
	pid, err := p.Read()
	if err != nil {
		// os.ErrNotExist is normal if ComfyUI not started via this tool
		if !os.IsNotExist(err) {
			Log.Warning("Could not read PID file %s: %v", p.path, err)
		}
		return 0, false
	}
	
	if IsProcessRunning(pid) {
		return pid, true
	}
	
	return pid, false // PID read, but process not running (stale)
}