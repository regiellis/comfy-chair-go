package internal

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/huh"
)

// PerformanceMetric represents a single performance measurement
type PerformanceMetric struct {
	Timestamp       time.Time     `json:"timestamp"`
	Environment     string        `json:"environment"`
	StartupTime     time.Duration `json:"startup_time_ms"`
	MemoryUsageMB   float64       `json:"memory_usage_mb"`
	CPUPercent      float64       `json:"cpu_percent"`
	ProcessID       int           `json:"process_id"`
	Port            int           `json:"port"`
	LogSizeMB       float64       `json:"log_size_mb"`
	CustomNodeCount int           `json:"custom_node_count"`
}

// PerformanceHistory stores historical performance data
type PerformanceHistory struct {
	Metrics []PerformanceMetric `json:"metrics"`
}

// PerformanceSummary provides aggregated statistics
type PerformanceSummary struct {
	AverageStartupTime time.Duration
	MinStartupTime     time.Duration
	MaxStartupTime     time.Duration
	AverageMemoryMB    float64
	MaxMemoryMB        float64
	TotalSessions      int
	LastWeekSessions   int
	MostUsedEnv        string
}

const (
	PerformanceHistoryFile = "comfy-performance-history.json"
	MaxHistoryEntries      = 1000 // Keep last 1000 entries
)

// GetPerformanceHistoryPath returns the path to the performance history file
func GetPerformanceHistoryPath() (string, error) {
	exePath, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(exePath), PerformanceHistoryFile), nil
}

// LoadPerformanceHistory loads the performance history from disk
func LoadPerformanceHistory() (*PerformanceHistory, error) {
	path, err := GetPerformanceHistoryPath()
	if err != nil {
		return nil, err
	}

	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &PerformanceHistory{Metrics: []PerformanceMetric{}}, nil
		}
		return nil, err
	}
	defer f.Close()

	var history PerformanceHistory
	decoder := json.NewDecoder(f)
	if err := decoder.Decode(&history); err != nil {
		return nil, err
	}

	return &history, nil
}

// SavePerformanceHistory saves the performance history to disk
func SavePerformanceHistory(history *PerformanceHistory) error {
	path, err := GetPerformanceHistoryPath()
	if err != nil {
		return err
	}

	// Limit history size
	if len(history.Metrics) > MaxHistoryEntries {
		history.Metrics = history.Metrics[len(history.Metrics)-MaxHistoryEntries:]
	}

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	encoder := json.NewEncoder(f)
	encoder.SetIndent("", "  ")
	return encoder.Encode(history)
}

// RecordPerformanceMetric adds a new performance metric to history
func RecordPerformanceMetric(metric PerformanceMetric) error {
	history, err := LoadPerformanceHistory()
	if err != nil {
		return err
	}

	history.Metrics = append(history.Metrics, metric)
	return SavePerformanceHistory(history)
}

// MeasureStartupTime measures how long ComfyUI takes to start up
func MeasureStartupTime(envType string, startProcess func() (*os.Process, error)) (time.Duration, *os.Process, error) {
	startTime := time.Now()
	process, err := startProcess()
	if err != nil {
		return 0, nil, err
	}

	// Wait for ComfyUI to be ready (check for successful connection on port)
	// This is a simple approach - in production you might want to check the actual API endpoint
	timeout := time.After(60 * time.Second) // 60 second timeout
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			return time.Since(startTime), process, fmt.Errorf("startup timeout after 60 seconds")
		case <-ticker.C:
			// Check if process is still running and ComfyUI might be ready
			if !IsProcessRunning(process.Pid) {
				return time.Since(startTime), process, fmt.Errorf("process died during startup")
			}
			
			// Simple heuristic: if it's been running for more than 3 seconds, assume it's ready
			// In a real implementation, you'd want to check the actual ComfyUI API endpoint
			if time.Since(startTime) > 3*time.Second {
				return time.Since(startTime), process, nil
			}
		}
	}
}

// GetCurrentMemoryUsage returns the current memory usage of a process in MB
func GetCurrentMemoryUsage(pid int) (float64, error) {
	if !IsProcessRunning(pid) {
		return 0, fmt.Errorf("process %d is not running", pid)
	}

	// Platform-specific memory measurement
	if runtime.GOOS == "windows" {
		return getWindowsMemoryUsage(pid)
	}
	return getUnixMemoryUsage(pid)
}

// getUnixMemoryUsage gets memory usage on Unix-like systems
func getUnixMemoryUsage(pid int) (float64, error) {
	// Read from /proc/{pid}/status for RSS (Resident Set Size)
	statusPath := fmt.Sprintf("/proc/%d/status", pid)
	data, err := os.ReadFile(statusPath)
	if err != nil {
		return 0, err
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "VmRSS:") {
			var memKB float64
			fmt.Sscanf(line, "VmRSS: %f kB", &memKB)
			return memKB / 1024, nil // Convert KB to MB
		}
	}
	return 0, fmt.Errorf("could not parse memory usage from /proc/%d/status", pid)
}

// getWindowsMemoryUsage gets memory usage on Windows (simplified implementation)
func getWindowsMemoryUsage(pid int) (float64, error) {
	// This is a simplified implementation
	// In a real implementation, you'd use Windows API calls
	return 0, fmt.Errorf("Windows memory monitoring not yet implemented")
}

// GetLogSize returns the size of the ComfyUI log file in MB
func GetLogSize(logPath string) float64 {
	if logPath == "" {
		return 0
	}

	info, err := os.Stat(ExpandUserPath(logPath))
	if err != nil {
		return 0
	}

	return float64(info.Size()) / (1024 * 1024) // Convert bytes to MB
}

// CountCustomNodes counts the number of custom nodes in the environment
func CountCustomNodes(customNodesPath string) int {
	entries, err := os.ReadDir(ExpandUserPath(customNodesPath))
	if err != nil {
		return 0
	}

	count := 0
	for _, entry := range entries {
		if entry.IsDir() && entry.Name() != "venv" && entry.Name() != ".venv" {
			count++
		}
	}
	return count
}

// CalculatePerformanceSummary generates summary statistics from performance history
func CalculatePerformanceSummary(history *PerformanceHistory) *PerformanceSummary {
	if len(history.Metrics) == 0 {
		return &PerformanceSummary{}
	}

	var totalStartup time.Duration
	var totalMemory float64
	var maxMemory float64
	minStartup := history.Metrics[0].StartupTime
	maxStartup := history.Metrics[0].StartupTime
	
	envCounts := make(map[string]int)
	weekAgo := time.Now().AddDate(0, 0, -7)
	lastWeekCount := 0

	for _, metric := range history.Metrics {
		totalStartup += metric.StartupTime
		totalMemory += metric.MemoryUsageMB

		if metric.StartupTime < minStartup {
			minStartup = metric.StartupTime
		}
		if metric.StartupTime > maxStartup {
			maxStartup = metric.StartupTime
		}
		if metric.MemoryUsageMB > maxMemory {
			maxMemory = metric.MemoryUsageMB
		}

		envCounts[metric.Environment]++
		
		if metric.Timestamp.After(weekAgo) {
			lastWeekCount++
		}
	}

	// Find most used environment
	mostUsedEnv := ""
	maxCount := 0
	for env, count := range envCounts {
		if count > maxCount {
			maxCount = count
			mostUsedEnv = env
		}
	}

	return &PerformanceSummary{
		AverageStartupTime: totalStartup / time.Duration(len(history.Metrics)),
		MinStartupTime:     minStartup,
		MaxStartupTime:     maxStartup,
		AverageMemoryMB:    totalMemory / float64(len(history.Metrics)),
		MaxMemoryMB:        maxMemory,
		TotalSessions:      len(history.Metrics),
		LastWeekSessions:   lastWeekCount,
		MostUsedEnv:        mostUsedEnv,
	}
}

// ShowPerformanceReport displays a comprehensive performance report
func ShowPerformanceReport() {
	fmt.Println(TitleStyle.Render("📊 ComfyUI Performance Report"))

	history, err := LoadPerformanceHistory()
	if err != nil {
		fmt.Println(ErrorStyle.Render(fmt.Sprintf("Failed to load performance history: %v", err)))
		PromptReturnToMenu()
		return
	}

	if len(history.Metrics) == 0 {
		fmt.Println(InfoStyle.Render("No performance data available yet. Start ComfyUI a few times to generate performance metrics."))
		PromptReturnToMenu()
		return
	}

	summary := CalculatePerformanceSummary(history)

	// Overall Statistics
	fmt.Println(SuccessStyle.Render("\n📈 Overall Statistics"))
	fmt.Printf("  Total Sessions: %d\n", summary.TotalSessions)
	fmt.Printf("  Sessions This Week: %d\n", summary.LastWeekSessions)
	fmt.Printf("  Most Used Environment: %s\n", summary.MostUsedEnv)

	// Startup Performance
	fmt.Println(SuccessStyle.Render("\n🚀 Startup Performance"))
	fmt.Printf("  Average Startup Time: %v\n", summary.AverageStartupTime.Round(time.Millisecond))
	fmt.Printf("  Fastest Startup: %v\n", summary.MinStartupTime.Round(time.Millisecond))
	fmt.Printf("  Slowest Startup: %v\n", summary.MaxStartupTime.Round(time.Millisecond))

	// Memory Usage
	fmt.Println(SuccessStyle.Render("\n💾 Memory Usage"))
	fmt.Printf("  Average Memory: %.1f MB\n", summary.AverageMemoryMB)
	fmt.Printf("  Peak Memory: %.1f MB\n", summary.MaxMemoryMB)

	// Recent Activity (last 10 sessions)
	fmt.Println(SuccessStyle.Render("\n📅 Recent Activity (Last 10 Sessions)"))
	recent := history.Metrics
	if len(recent) > 10 {
		recent = recent[len(recent)-10:]
	}

	for i := len(recent) - 1; i >= 0; i-- {
		metric := recent[i]
		fmt.Printf("  %s [%s] - Startup: %v, Memory: %.1f MB\n",
			metric.Timestamp.Format("01/02 15:04"),
			metric.Environment,
			metric.StartupTime.Round(time.Millisecond),
			metric.MemoryUsageMB)
	}

	// Environment Breakdown
	envCounts := make(map[string]int)
	for _, metric := range history.Metrics {
		envCounts[metric.Environment]++
	}

	if len(envCounts) > 1 {
		fmt.Println(SuccessStyle.Render("\n🏠 Environment Usage"))
		for env, count := range envCounts {
			percentage := float64(count) / float64(len(history.Metrics)) * 100
			fmt.Printf("  %s: %d sessions (%.1f%%)\n", env, count, percentage)
		}
	}

	PromptReturnToMenu()
}

// ShowPerformanceMenu displays performance monitoring options
func ShowPerformanceMenu() {
	for {
		var choice string
		form := huh.NewForm(huh.NewGroup(
			huh.NewSelect[string]().
				Title("Performance Monitoring").
				Options(
					huh.NewOption("View Performance Report", "report"),
					huh.NewOption("Clear Performance History", "clear"),
					huh.NewOption("Export Performance Data", "export"),
					huh.NewOption("Main Menu", "back"),
				).
				Value(&choice),
		)).WithTheme(huh.ThemeCharm())
		
		if err := form.Run(); err != nil || choice == "back" || choice == "" {
			return
		}

		switch choice {
		case "report":
			ShowPerformanceReport()
		case "clear":
			confirmClearHistory()
		case "export":
			exportPerformanceData()
		}
	}
}

// confirmClearHistory asks for confirmation before clearing performance history
func confirmClearHistory() {
	var confirm bool
	form := huh.NewForm(huh.NewGroup(
		huh.NewConfirm().
			Title("Clear Performance History").
			Description("This will permanently delete all performance data. Are you sure?").
			Value(&confirm),
	)).WithTheme(huh.ThemeCharm())

	if err := form.Run(); err != nil || !confirm {
		fmt.Println(InfoStyle.Render("Operation cancelled."))
		return
	}

	path, err := GetPerformanceHistoryPath()
	if err != nil {
		fmt.Println(ErrorStyle.Render(fmt.Sprintf("Failed to get history path: %v", err)))
		return
	}

	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		fmt.Println(ErrorStyle.Render(fmt.Sprintf("Failed to clear history: %v", err)))
		return
	}

	fmt.Println(SuccessStyle.Render("Performance history cleared successfully."))
}

// exportPerformanceData exports performance data to a JSON file
func exportPerformanceData() {
	history, err := LoadPerformanceHistory()
	if err != nil {
		fmt.Println(ErrorStyle.Render(fmt.Sprintf("Failed to load performance history: %v", err)))
		return
	}

	if len(history.Metrics) == 0 {
		fmt.Println(InfoStyle.Render("No performance data to export."))
		return
	}

	// Sort by timestamp for better readability
	sort.Slice(history.Metrics, func(i, j int) bool {
		return history.Metrics[i].Timestamp.Before(history.Metrics[j].Timestamp)
	})

	timestamp := time.Now().Format("2006-01-02_15-04-05")
	filename := fmt.Sprintf("comfy-performance-export_%s.json", timestamp)

	f, err := os.Create(filename)
	if err != nil {
		fmt.Println(ErrorStyle.Render(fmt.Sprintf("Failed to create export file: %v", err)))
		return
	}
	defer f.Close()

	encoder := json.NewEncoder(f)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(history); err != nil {
		fmt.Println(ErrorStyle.Render(fmt.Sprintf("Failed to write export data: %v", err)))
		return
	}

	fmt.Println(SuccessStyle.Render(fmt.Sprintf("Performance data exported to: %s", filename)))
}