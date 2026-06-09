//go:build !windows

package internal

import (
	"os"
	"runtime"
	"testing"
)

func TestGetDiskUsage(t *testing.T) {
	usage := getDiskUsage(os.TempDir())
	if usage < 0 || usage > 100 {
		t.Errorf("getDiskUsage(%q) = %f, want a percentage in [0, 100]", os.TempDir(), usage)
	}
}

func TestGetProcessMemoryMB(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("reads /proc, linux only")
	}
	mb, err := getProcessMemoryMB(os.Getpid())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mb <= 0 {
		t.Errorf("getProcessMemoryMB(self) = %f, want > 0", mb)
	}
}

func TestGetAvailableMemoryMB(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("reads /proc/meminfo, linux only")
	}
	mb, err := getAvailableMemoryMB()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mb <= 0 {
		t.Errorf("getAvailableMemoryMB() = %f, want > 0", mb)
	}
}
