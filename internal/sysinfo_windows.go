//go:build windows

package internal

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	modkernel32              = windows.NewLazySystemDLL("kernel32.dll")
	procGetProcessMemoryInfo = modkernel32.NewProc("K32GetProcessMemoryInfo")
)

// processMemoryCounters mirrors PROCESS_MEMORY_COUNTERS from psapi.h.
type processMemoryCounters struct {
	cb                         uint32
	PageFaultCount             uint32
	PeakWorkingSetSize         uintptr
	WorkingSetSize             uintptr
	QuotaPeakPagedPoolUsage    uintptr
	QuotaPagedPoolUsage        uintptr
	QuotaPeakNonPagedPoolUsage uintptr
	QuotaNonPagedPoolUsage     uintptr
	PagefileUsage              uintptr
	PeakPagefileUsage          uintptr
}

// getDiskUsage returns the used percentage of the volume containing path,
// or -1 if it cannot be determined.
func getDiskUsage(path string) float64 {
	p, err := windows.UTF16PtrFromString(ExpandUserPath(path))
	if err != nil {
		return -1
	}
	var freeForCaller, total, totalFree uint64
	if err := windows.GetDiskFreeSpaceEx(p, &freeForCaller, &total, &totalFree); err != nil || total == 0 {
		return -1
	}
	return float64(total-totalFree) / float64(total) * 100
}

// getProcessMemoryMB returns the working set size of pid in MB.
func getProcessMemoryMB(pid int) (float64, error) {
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return 0, fmt.Errorf("could not open process %d: %w", pid, err)
	}
	defer windows.CloseHandle(h)
	var pmc processMemoryCounters
	pmc.cb = uint32(unsafe.Sizeof(pmc))
	r1, _, callErr := procGetProcessMemoryInfo.Call(uintptr(h), uintptr(unsafe.Pointer(&pmc)), uintptr(pmc.cb))
	if r1 == 0 {
		return 0, fmt.Errorf("could not query memory for process %d: %v", pid, callErr)
	}
	return float64(pmc.WorkingSetSize) / (1024 * 1024), nil
}
