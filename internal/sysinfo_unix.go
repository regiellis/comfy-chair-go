//go:build !windows

package internal

import (
	"fmt"
	"os"
	"strings"
	"syscall"
)

// getDiskUsage returns the used percentage of the filesystem containing path,
// or -1 if it cannot be determined.
func getDiskUsage(path string) float64 {
	var st syscall.Statfs_t
	if err := syscall.Statfs(ExpandUserPath(path), &st); err != nil || st.Blocks == 0 {
		return -1
	}
	return float64(st.Blocks-st.Bfree) / float64(st.Blocks) * 100
}

// getProcessMemoryMB returns the resident set size of pid in MB, read from
// /proc (Linux). On platforms without /proc it returns an error.
func getProcessMemoryMB(pid int) (float64, error) {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/status", pid))
	if err != nil {
		return 0, err
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "VmRSS:") {
			var memKB float64
			fmt.Sscanf(line, "VmRSS: %f kB", &memKB)
			return memKB / 1024, nil
		}
	}
	return 0, fmt.Errorf("could not parse memory usage from /proc/%d/status", pid)
}
