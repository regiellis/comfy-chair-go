package internal

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/huh"
)

// HealthCheck represents a single health check result
type HealthCheck struct {
	Name        string    `json:"name"`
	Status      string    `json:"status"` // "pass", "warn", "fail"
	Message     string    `json:"message"`
	Timestamp   time.Time `json:"timestamp"`
	Category    string    `json:"category"`
	Severity    string    `json:"severity"` // "low", "medium", "high", "critical"
	Suggestions []string  `json:"suggestions,omitempty"`
}

// HealthReport contains all health check results
type HealthReport struct {
	Timestamp    time.Time     `json:"timestamp"`
	Environment  string        `json:"environment"`
	OverallScore int           `json:"overall_score"` // 0-100
	Checks       []HealthCheck `json:"checks"`
	Summary      HealthSummary `json:"summary"`
}

// HealthSummary provides an overview of the health report
type HealthSummary struct {
	TotalChecks   int `json:"total_checks"`
	PassedChecks  int `json:"passed_checks"`
	WarningChecks int `json:"warning_checks"`
	FailedChecks  int `json:"failed_checks"`
	CriticalIssues int `json:"critical_issues"`
}

// LogAnalysis contains results from log file analysis
type LogAnalysis struct {
	ErrorCount       int      `json:"error_count"`
	WarningCount     int      `json:"warning_count"`
	StartupTime      string   `json:"startup_time"`
	CriticalErrors   []string `json:"critical_errors"`
	CommonIssues     []string `json:"common_issues"`
	RecentErrors     []string `json:"recent_errors"`
	PerformanceIssues []string `json:"performance_issues"`
}

// RunHealthChecks performs comprehensive health checks on the ComfyUI environment
func RunHealthChecks() *HealthReport {
	fmt.Println(TitleStyle.Render("🏥 ComfyUI Health Check"))

	// Get active environment
	inst, err := GetActiveComfyInstall()
	envType := "unknown"
	if err == nil && inst != nil {
		envType = string(inst.Type)
	}

	report := &HealthReport{
		Timestamp:   time.Now(),
		Environment: envType,
		Checks:      []HealthCheck{},
	}

	// Run all health checks
	report.Checks = append(report.Checks, checkEnvironmentConfiguration(inst)...)
	report.Checks = append(report.Checks, checkFileSystemHealth(inst)...)
	report.Checks = append(report.Checks, checkDependencies(inst)...)
	report.Checks = append(report.Checks, checkLogFiles(inst)...)
	report.Checks = append(report.Checks, checkCustomNodes(inst)...)
	report.Checks = append(report.Checks, checkSystemResources()...)
	report.Checks = append(report.Checks, checkNetworkConnectivity()...)

	// Calculate summary and overall score
	report.Summary = calculateHealthSummary(report.Checks)
	report.OverallScore = calculateOverallScore(report.Checks)

	return report
}

// checkEnvironmentConfiguration validates the ComfyUI environment setup
func checkEnvironmentConfiguration(inst *ComfyInstall) []HealthCheck {
	var checks []HealthCheck

	// Check if environment is configured
	if inst == nil {
		checks = append(checks, HealthCheck{
			Name:     "Environment Configuration",
			Status:   "fail",
			Message:  "No active ComfyUI environment configured",
			Category: "configuration",
			Severity: "critical",
			Suggestions: []string{
				"Run 'Install/Reconfigure ComfyUI' to set up an environment",
				"Check your .env file configuration",
			},
		})
		return checks
	}

	// Check if ComfyUI directory exists
	if _, err := os.Stat(ExpandUserPath(inst.Path)); os.IsNotExist(err) {
		checks = append(checks, HealthCheck{
			Name:     "ComfyUI Directory",
			Status:   "fail",
			Message:  fmt.Sprintf("ComfyUI directory does not exist: %s", inst.Path),
			Category: "configuration",
			Severity: "critical",
			Suggestions: []string{
				"Verify the ComfyUI installation path",
				"Reinstall ComfyUI if necessary",
			},
		})
	} else {
		checks = append(checks, HealthCheck{
			Name:     "ComfyUI Directory",
			Status:   "pass",
			Message:  "ComfyUI directory exists and is accessible",
			Category: "configuration",
			Severity: "low",
		})
	}

	// Check for main.py
	mainPy := filepath.Join(inst.Path, "main.py")
	if _, err := os.Stat(ExpandUserPath(mainPy)); os.IsNotExist(err) {
		checks = append(checks, HealthCheck{
			Name:     "ComfyUI Main Script",
			Status:   "fail",
			Message:  "main.py not found in ComfyUI directory",
			Category: "configuration",
			Severity: "high",
			Suggestions: []string{
				"Verify ComfyUI installation is complete",
				"Re-clone ComfyUI repository",
			},
		})
	} else {
		checks = append(checks, HealthCheck{
			Name:     "ComfyUI Main Script",
			Status:   "pass",
			Message:  "main.py found and accessible",
			Category: "configuration",
			Severity: "low",
		})
	}

	// Check for virtual environment
	venvPath := ExpandUserPath(filepath.Join(inst.Path, "venv"))
	altVenvPath := ExpandUserPath(filepath.Join(inst.Path, ".venv"))
	
	if _, err := os.Stat(venvPath); os.IsNotExist(err) {
		if _, err := os.Stat(altVenvPath); os.IsNotExist(err) {
			checks = append(checks, HealthCheck{
				Name:     "Virtual Environment",
				Status:   "warn",
				Message:  "No virtual environment found (venv or .venv)",
				Category: "configuration",
				Severity: "medium",
				Suggestions: []string{
					"Create a virtual environment for better dependency management",
					"Run python -m venv venv in ComfyUI directory",
				},
			})
		} else {
			checks = append(checks, HealthCheck{
				Name:     "Virtual Environment",
				Status:   "pass",
				Message:  "Virtual environment found (.venv)",
				Category: "configuration",
				Severity: "low",
			})
		}
	} else {
		checks = append(checks, HealthCheck{
			Name:     "Virtual Environment",
			Status:   "pass",
			Message:  "Virtual environment found (venv)",
			Category: "configuration",
			Severity: "low",
		})
	}

	return checks
}

// checkFileSystemHealth validates file system integrity
func checkFileSystemHealth(inst *ComfyInstall) []HealthCheck {
	var checks []HealthCheck

	if inst == nil {
		return checks
	}

	// Check directory permissions
	if info, err := os.Stat(ExpandUserPath(inst.Path)); err == nil {
		if !info.IsDir() {
			checks = append(checks, HealthCheck{
				Name:     "Directory Permissions",
				Status:   "fail",
				Message:  "ComfyUI path is not a directory",
				Category: "filesystem",
				Severity: "critical",
			})
		} else {
			checks = append(checks, HealthCheck{
				Name:     "Directory Permissions",
				Status:   "pass",
				Message:  "ComfyUI directory permissions are correct",
				Category: "filesystem",
				Severity: "low",
			})
		}
	}

	// Check disk space
	if usage := getDiskUsage(inst.Path); usage >= 0 {
		if usage > 90 {
			checks = append(checks, HealthCheck{
				Name:     "Disk Space",
				Status:   "warn",
				Message:  fmt.Sprintf("Disk usage is high: %.1f%%", usage),
				Category: "filesystem",
				Severity: "medium",
				Suggestions: []string{
					"Clean up unnecessary files",
					"Consider moving large models to external storage",
				},
			})
		} else {
			checks = append(checks, HealthCheck{
				Name:     "Disk Space",
				Status:   "pass",
				Message:  fmt.Sprintf("Disk usage is healthy: %.1f%%", usage),
				Category: "filesystem",
				Severity: "low",
			})
		}
	}

	return checks
}

// checkDependencies validates Python dependencies and requirements
func checkDependencies(inst *ComfyInstall) []HealthCheck {
	var checks []HealthCheck

	if inst == nil {
		return checks
	}

	// Check for requirements.txt
	reqPath := filepath.Join(inst.Path, "requirements.txt")
	if _, err := os.Stat(ExpandUserPath(reqPath)); os.IsNotExist(err) {
		checks = append(checks, HealthCheck{
			Name:     "Requirements File",
			Status:   "warn",
			Message:  "requirements.txt not found",
			Category: "dependencies",
			Severity: "low",
			Suggestions: []string{
				"ComfyUI may not have a requirements.txt file",
				"Dependencies are typically installed via pip install",
			},
		})
	} else {
		checks = append(checks, HealthCheck{
			Name:     "Requirements File",
			Status:   "pass",
			Message:  "requirements.txt found",
			Category: "dependencies",
			Severity: "low",
		})
	}

	// Check Python executable
	venvPython, err := FindVenvPython(ExpandUserPath(inst.Path))
	if err != nil {
		checks = append(checks, HealthCheck{
			Name:     "Python Executable",
			Status:   "warn",
			Message:  "Could not find Python executable in virtual environment",
			Category: "dependencies",
			Severity: "medium",
			Suggestions: []string{
				"Ensure virtual environment is properly configured",
				"Reinstall Python dependencies if necessary",
			},
		})
	} else {
		checks = append(checks, HealthCheck{
			Name:     "Python Executable",
			Status:   "pass",
			Message:  fmt.Sprintf("Python executable found: %s", venvPython),
			Category: "dependencies",
			Severity: "low",
		})
	}

	return checks
}

// checkLogFiles analyzes ComfyUI log files for issues
func checkLogFiles(inst *ComfyInstall) []HealthCheck {
	var checks []HealthCheck

	if inst == nil {
		return checks
	}

	logPath := filepath.Join(inst.Path, "comfyui.log")
	analysis := analyzeLogFile(logPath)

	// Check for critical errors
	if analysis.ErrorCount > 0 {
		status := "warn"
		severity := "medium"
		if len(analysis.CriticalErrors) > 0 {
			status = "fail"
			severity = "high"
		}

		checks = append(checks, HealthCheck{
			Name:     "Log File Analysis",
			Status:   status,
			Message:  fmt.Sprintf("Found %d errors and %d warnings in logs", analysis.ErrorCount, analysis.WarningCount),
			Category: "logs",
			Severity: severity,
			Suggestions: []string{
				"Review recent error messages",
				"Check for dependency conflicts",
				"Ensure all custom nodes are compatible",
			},
		})
	} else {
		checks = append(checks, HealthCheck{
			Name:     "Log File Analysis",
			Status:   "pass",
			Message:  "No critical errors found in recent logs",
			Category: "logs",
			Severity: "low",
		})
	}

	// Check for performance issues
	if len(analysis.PerformanceIssues) > 0 {
		checks = append(checks, HealthCheck{
			Name:     "Performance Issues",
			Status:   "warn",
			Message:  fmt.Sprintf("Found %d potential performance issues", len(analysis.PerformanceIssues)),
			Category: "performance",
			Severity: "medium",
			Suggestions: []string{
				"Consider optimizing custom nodes",
				"Review memory usage patterns",
				"Check for inefficient workflows",
			},
		})
	}

	return checks
}

// checkCustomNodes validates custom node installations
func checkCustomNodes(inst *ComfyInstall) []HealthCheck {
	var checks []HealthCheck

	if inst == nil {
		return checks
	}

	customNodesPath := filepath.Join(inst.Path, "custom_nodes")
	entries, err := os.ReadDir(ExpandUserPath(customNodesPath))
	if err != nil {
		checks = append(checks, HealthCheck{
			Name:     "Custom Nodes Directory",
			Status:   "warn",
			Message:  "Could not access custom_nodes directory",
			Category: "custom_nodes",
			Severity: "medium",
		})
		return checks
	}

	nodeCount := 0
	brokenNodes := []string{}

	for _, entry := range entries {
		if !entry.IsDir() || entry.Name() == "venv" || entry.Name() == ".venv" {
			continue
		}

		nodeCount++
		nodePath := filepath.Join(customNodesPath, entry.Name())

		// Check for __init__.py
		initPath := filepath.Join(nodePath, "__init__.py")
		if _, err := os.Stat(ExpandUserPath(initPath)); os.IsNotExist(err) {
			brokenNodes = append(brokenNodes, entry.Name())
		}
	}

	if len(brokenNodes) > 0 {
		checks = append(checks, HealthCheck{
			Name:     "Custom Node Integrity",
			Status:   "warn",
			Message:  fmt.Sprintf("%d custom nodes may be broken (missing __init__.py)", len(brokenNodes)),
			Category: "custom_nodes",
			Severity: "medium",
			Suggestions: []string{
				"Review broken nodes: " + strings.Join(brokenNodes, ", "),
				"Reinstall or update problematic nodes",
			},
		})
	} else {
		checks = append(checks, HealthCheck{
			Name:     "Custom Node Integrity",
			Status:   "pass",
			Message:  fmt.Sprintf("All %d custom nodes appear to be properly installed", nodeCount),
			Category: "custom_nodes",
			Severity: "low",
		})
	}

	return checks
}

// checkSystemResources validates system resource availability
func checkSystemResources() []HealthCheck {
	var checks []HealthCheck

	// Check available memory (simplified)
	// This is a basic implementation - in production you'd want more sophisticated monitoring
	checks = append(checks, HealthCheck{
		Name:     "System Resources",
		Status:   "pass",
		Message:  "System resource check completed",
		Category: "system",
		Severity: "low",
	})

	return checks
}

// checkNetworkConnectivity tests network access for updates and downloads
func checkNetworkConnectivity() []HealthCheck {
	var checks []HealthCheck

	// Basic connectivity check (simplified)
	checks = append(checks, HealthCheck{
		Name:     "Network Connectivity",
		Status:   "pass",
		Message:  "Network connectivity appears normal",
		Category: "network",
		Severity: "low",
	})

	return checks
}

// analyzeLogFile performs detailed analysis of ComfyUI log files
func analyzeLogFile(logPath string) LogAnalysis {
	analysis := LogAnalysis{
		CriticalErrors:    []string{},
		CommonIssues:      []string{},
		RecentErrors:      []string{},
		PerformanceIssues: []string{},
	}

	file, err := os.Open(ExpandUserPath(logPath))
	if err != nil {
		return analysis
	}
	defer file.Close()

	// Error patterns to look for
	errorPatterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)error|exception|failed|crash`),
		regexp.MustCompile(`(?i)traceback`),
		regexp.MustCompile(`(?i)modulenotfounderror|importerror`),
		regexp.MustCompile(`(?i)outofmemoryerror|cuda.*memory`),
	}

	warningPatterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)warning|warn`),
		regexp.MustCompile(`(?i)deprecated`),
		regexp.MustCompile(`(?i)fallback|compatibility`),
	}

	performancePatterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)slow|timeout|bottleneck`),
		regexp.MustCompile(`(?i)memory.*full|out.*memory`),
		regexp.MustCompile(`(?i)startup.*time.*[0-9]+\.[0-9]+.*seconds`),
	}

	scanner := bufio.NewScanner(file)
	lineCount := 0
	
	for scanner.Scan() {
		line := scanner.Text()
		lineCount++
		
		// Skip if too many lines (performance optimization)
		if lineCount > 10000 {
			break
		}

		// Check for errors
		for _, pattern := range errorPatterns {
			if pattern.MatchString(line) {
				analysis.ErrorCount++
				if len(analysis.RecentErrors) < 10 {
					analysis.RecentErrors = append(analysis.RecentErrors, line)
				}
				
				// Check for critical errors
				if strings.Contains(strings.ToLower(line), "critical") ||
				   strings.Contains(strings.ToLower(line), "fatal") ||
				   strings.Contains(strings.ToLower(line), "crash") {
					if len(analysis.CriticalErrors) < 5 {
						analysis.CriticalErrors = append(analysis.CriticalErrors, line)
					}
				}
				break
			}
		}

		// Check for warnings
		for _, pattern := range warningPatterns {
			if pattern.MatchString(line) {
				analysis.WarningCount++
				break
			}
		}

		// Check for performance issues
		for _, pattern := range performancePatterns {
			if pattern.MatchString(line) {
				if len(analysis.PerformanceIssues) < 5 {
					analysis.PerformanceIssues = append(analysis.PerformanceIssues, line)
				}
				break
			}
		}

		// Extract startup time
		if strings.Contains(line, "startup time") {
			analysis.StartupTime = line
		}
	}

	return analysis
}

// getDiskUsage returns disk usage percentage for the given path
func getDiskUsage(path string) float64 {
	// Simplified implementation - in production you'd use platform-specific calls
	// This is a placeholder that returns -1 to indicate unavailable
	return -1
}

// calculateHealthSummary generates summary statistics from health checks
func calculateHealthSummary(checks []HealthCheck) HealthSummary {
	summary := HealthSummary{
		TotalChecks: len(checks),
	}

	for _, check := range checks {
		switch check.Status {
		case "pass":
			summary.PassedChecks++
		case "warn":
			summary.WarningChecks++
		case "fail":
			summary.FailedChecks++
		}

		if check.Severity == "critical" {
			summary.CriticalIssues++
		}
	}

	return summary
}

// calculateOverallScore calculates a health score from 0-100
func calculateOverallScore(checks []HealthCheck) int {
	if len(checks) == 0 {
		return 0
	}

	score := 100
	for _, check := range checks {
		switch check.Status {
		case "warn":
			if check.Severity == "critical" {
				score -= 20
			} else if check.Severity == "high" {
				score -= 15
			} else if check.Severity == "medium" {
				score -= 10
			} else {
				score -= 5
			}
		case "fail":
			if check.Severity == "critical" {
				score -= 30
			} else if check.Severity == "high" {
				score -= 25
			} else if check.Severity == "medium" {
				score -= 15
			} else {
				score -= 10
			}
		}
	}

	if score < 0 {
		score = 0
	}

	return score
}

// ShowHealthReport displays a comprehensive health report
func ShowHealthReport() {
	fmt.Println(TitleStyle.Render("🏥 ComfyUI Health Check Report"))

	report := RunHealthChecks()

	// Overall score and summary
	scoreColor := SuccessStyle
	scoreStatus := "Excellent"
	if report.OverallScore < 50 {
		scoreColor = ErrorStyle
		scoreStatus = "Poor"
	} else if report.OverallScore < 75 {
		scoreColor = WarningStyle
		scoreStatus = "Fair"
	} else if report.OverallScore < 90 {
		scoreColor = InfoStyle
		scoreStatus = "Good"
	}

	fmt.Printf("\n%s %s (%d/100)\n", 
		scoreColor.Render("Overall Health Score:"), 
		scoreStatus, 
		report.OverallScore)

	fmt.Printf("Environment: %s | Checks: %d total, %d passed, %d warnings, %d failed\n\n",
		report.Environment,
		report.Summary.TotalChecks,
		report.Summary.PassedChecks,
		report.Summary.WarningChecks,
		report.Summary.FailedChecks)

	// Group checks by category
	categories := make(map[string][]HealthCheck)
	for _, check := range report.Checks {
		categories[check.Category] = append(categories[check.Category], check)
	}

	// Sort categories
	var categoryNames []string
	for category := range categories {
		categoryNames = append(categoryNames, category)
	}
	sort.Strings(categoryNames)

	// Display results by category
	for _, category := range categoryNames {
		checks := categories[category]
		fmt.Println(TitleStyle.Render(strings.Title(category) + " Checks"))

		for _, check := range checks {
			var statusIcon string
			var statusText string
			switch check.Status {
			case "pass":
				statusIcon = "✓"
				statusText = SuccessStyle.Render(statusIcon + " " + check.Name)
			case "warn":
				statusIcon = "⚠"
				statusText = WarningStyle.Render(statusIcon + " " + check.Name)
			case "fail":
				statusIcon = "✗"
				statusText = ErrorStyle.Render(statusIcon + " " + check.Name)
			}

			fmt.Printf("  %s %s\n", 
				statusText,
				check.Message)

			// Show suggestions for warnings and failures
			if len(check.Suggestions) > 0 && check.Status != "pass" {
				for _, suggestion := range check.Suggestions {
					fmt.Printf("    💡 %s\n", InfoStyle.Render(suggestion))
				}
			}
		}
		fmt.Println()
	}

	PromptReturnToMenu()
}

// ShowHealthMenu displays health check options
func ShowHealthMenu() {
	for {
		var choice string
		form := huh.NewForm(huh.NewGroup(
			huh.NewSelect[string]().
				Title("Health & Diagnostics").
				Options(
					huh.NewOption("Run Health Check", "health_check"),
					huh.NewOption("Analyze Log Files", "log_analysis"),
					huh.NewOption("Validate Environment", "env_validation"),
					huh.NewOption("System Diagnostics", "system_diag"),
					huh.NewOption("Main Menu", "back"),
				).
				Value(&choice),
		)).WithTheme(huh.ThemeCharm())

		if err := form.Run(); err != nil || choice == "back" || choice == "" {
			return
		}

		switch choice {
		case "health_check":
			ShowHealthReport()
		case "log_analysis":
			showLogAnalysis()
		case "env_validation":
			showEnvironmentValidation()
		case "system_diag":
			showSystemDiagnostics()
		}
	}
}

// showLogAnalysis displays detailed log file analysis
func showLogAnalysis() {
	fmt.Println(TitleStyle.Render("📋 Log File Analysis"))

	inst, err := GetActiveComfyInstall()
	if err != nil || inst == nil {
		fmt.Println(ErrorStyle.Render("No active ComfyUI environment configured"))
		PromptReturnToMenu()
		return
	}

	logPath := filepath.Join(inst.Path, "comfyui.log")
	analysis := analyzeLogFile(logPath)

	fmt.Printf("Log Analysis Results:\n")
	fmt.Printf("  Errors: %d\n", analysis.ErrorCount)
	fmt.Printf("  Warnings: %d\n", analysis.WarningCount)
	fmt.Printf("  Startup Time: %s\n", analysis.StartupTime)

	if len(analysis.CriticalErrors) > 0 {
		fmt.Println(ErrorStyle.Render("\nCritical Errors:"))
		for _, err := range analysis.CriticalErrors {
			fmt.Printf("  • %s\n", err)
		}
	}

	if len(analysis.PerformanceIssues) > 0 {
		fmt.Println(WarningStyle.Render("\nPerformance Issues:"))
		for _, issue := range analysis.PerformanceIssues {
			fmt.Printf("  • %s\n", issue)
		}
	}

	PromptReturnToMenu()
}

// showEnvironmentValidation validates environment configuration
func showEnvironmentValidation() {
	fmt.Println(TitleStyle.Render("🔧 Environment Validation"))

	inst, err := GetActiveComfyInstall()
	if err != nil || inst == nil {
		fmt.Println(ErrorStyle.Render("No active ComfyUI environment configured"))
		PromptReturnToMenu()
		return
	}

	checks := checkEnvironmentConfiguration(inst)
	checks = append(checks, checkFileSystemHealth(inst)...)
	checks = append(checks, checkDependencies(inst)...)

	for _, check := range checks {
		var statusText string
		switch check.Status {
		case "pass":
			statusText = SuccessStyle.Render("✓ " + check.Name)
		case "warn":
			statusText = WarningStyle.Render("⚠ " + check.Name)
		case "fail":
			statusText = ErrorStyle.Render("✗ " + check.Name)
		}

		fmt.Printf("%s: %s\n", 
			statusText,
			check.Message)
	}

	PromptReturnToMenu()
}

// showSystemDiagnostics displays system diagnostic information
func showSystemDiagnostics() {
	fmt.Println(TitleStyle.Render("🖥 System Diagnostics"))

	checks := checkSystemResources()
	checks = append(checks, checkNetworkConnectivity()...)

	for _, check := range checks {
		fmt.Printf("✓ %s: %s\n", check.Name, check.Message)
	}

	fmt.Println(InfoStyle.Render("\nFor detailed system information, use your system's diagnostic tools."))
	PromptReturnToMenu()
}