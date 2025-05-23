// Copyright (C) 2025 Regi Ellis
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

//go:build !windows
// +build !windows

package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/joho/godotenv"
)

// Configuration
const (
	// comfyUIDirNameDefault is used as a default if no .env is found and no ComfyUI dir is chosen by user
	comfyUIDirNameDefault = "ComfyUI"
	venvDirName           = "venv"
	pidFileName           = "comfyui.pid" // Relative to CLI executable dir
	logFileName           = "comfyui.log" // Relative to CLI executable dir
	envFileName           = ".env"        // Relative to CLI executable dir
	envComfyUIPathKey     = "COMFYUI_PATH"
	maxWaitTime           = 60 * time.Second
	comfyReadyString      = "Starting server" // String to look for in logs
	comfyUIRepoURL        = "https://github.com/comfyanonymous/ComfyUI.git"
)

// Styles and related variables
var (
	docStyle                = lipgloss.NewStyle().Margin(1, 2)
	titleStyle              = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("62")) // Light Purple
	successStyle            = lipgloss.NewStyle().Foreground(lipgloss.Color("40"))            // Green
	errorStyle              = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))           // Red
	infoStyle               = lipgloss.NewStyle().Foreground(lipgloss.Color("39"))            // Blue
	warningStyle            = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))           // Orange
	spinnerStyle            = lipgloss.NewStyle().Foreground(lipgloss.Color("69"))
	spinnerFrames           = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	pythonExecutable        = "python"
	pythonExecutableWindows = "python.exe"
)

// paths struct to hold all relevant paths
type paths struct {
	cliDir          string // Directory where the CLI executable is running
	envFile         string // Path to the .env file
	comfyUIDir      string // Root directory of ComfyUI installation
	venvPath        string
	venvPython      string
	pidFile         string
	logFile         string
	requirementsTxt string
	isConfigured    bool // True if comfyUIDir is successfully determined
}

// Global paths variable
var appPaths paths

// getPythonExecutables returns a list of common Python executable names.
func getPythonExecutables() []string {
	if runtime.GOOS == "windows" {
		return []string{"python.exe", "python3.exe"}
	}
	return []string{"python3", "python"}
}

// findSystemPython attempts to find a Python executable in the system PATH.
func findSystemPython() (string, error) {
	pythons := getPythonExecutables()
	for _, p := range pythons {
		path, err := exec.LookPath(p)
		if err == nil {
			return path, nil
		}
	}
	return "", fmt.Errorf("could not find a Python executable (tried: %s) in your system PATH", strings.Join(pythons, ", "))
}

// initPaths initializes application paths, prioritizing .env file.
func initPaths() error {
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}
	appPaths.cliDir = filepath.Dir(exePath)
	appPaths.envFile = filepath.Join(appPaths.cliDir, envFileName)
	appPaths.pidFile = filepath.Join(appPaths.cliDir, pidFileName)
	appPaths.logFile = filepath.Join(appPaths.cliDir, logFileName)
	appPaths.isConfigured = false

	// Load .env file from CLI directory
	err = godotenv.Load(appPaths.envFile)
	if err != nil {
		if (!os.IsNotExist(err)) {
			fmt.Println(warningStyle.Render(fmt.Sprintf("Warning: Could not load .env file at %s: %v", appPaths.envFile, err)))
		}
		// If .env does not exist, it's not an error yet.
	}

	// .env validation: check required variables
	requiredEnv := []string{"COMFYUI_PATH"}
	missing := []string{}
	for _, key := range requiredEnv {
		if os.Getenv(key) == "" {
			missing = append(missing, key)
		}
	}
	if len(missing) > 0 {
		fmt.Println(warningStyle.Render("Warning: The following required .env variables are missing:"))
		for _, key := range missing {
			fmt.Println("  - " + key)
		}
		fmt.Println(infoStyle.Render("Please set them in your .env file before using all features."))
	}

	comfyPathFromEnv := os.Getenv(envComfyUIPathKey)

	if comfyPathFromEnv != "" {
		absComfyPath, pathErr := filepath.Abs(comfyPathFromEnv)
		if pathErr != nil {
			fmt.Println(warningStyle.Render(fmt.Sprintf("Warning: COMFYUI_PATH '%s' from .env is not a valid path: %v. Ignoring.", comfyPathFromEnv, pathErr)))
		} else if stat, err := os.Stat(absComfyPath); err == nil && stat.IsDir() {
			appPaths.comfyUIDir = absComfyPath
			appPaths.isConfigured = true
		} else {
			fmt.Println(warningStyle.Render(fmt.Sprintf("Warning: COMFYUI_PATH '%s' from .env is not a valid directory. Current value: '%s'. Ignoring.", comfyPathFromEnv, absComfyPath)))
		}
	}

	// If not configured via .env, try default location (ComfyUI subdirectory next to CLI)
	if !appPaths.isConfigured {
		defaultComfyDir := filepath.Join(appPaths.cliDir, comfyUIDirNameDefault)
		if stat, err := os.Stat(defaultComfyDir); err == nil && stat.IsDir() {
			appPaths.comfyUIDir = defaultComfyDir // It's already absolute if cliDir is.
			appPaths.isConfigured = true
			fmt.Println(infoStyle.Render(fmt.Sprintf("No %s found or COMFYUI_PATH not set. Using default ComfyUI location: %s", envFileName, appPaths.comfyUIDir)))
		}
	}

	if appPaths.isConfigured {
		// Check for both venv and .venv directories
		venvCandidates := []string{"venv", ".venv"}
		venvFound := false
		for _, venvDir := range venvCandidates {
			candidatePath := filepath.Join(appPaths.comfyUIDir, venvDir)
			if stat, err := os.Stat(candidatePath); err == nil && stat.IsDir() {
				appPaths.venvPath = candidatePath
				if runtime.GOOS == "windows" {
					appPaths.venvPython = filepath.Join(candidatePath, "Scripts", "python.exe")
				} else {
					appPaths.venvPython = filepath.Join(candidatePath, "bin", "python")
				}
				venvFound = true
				break
			}
		}
		appPaths.requirementsTxt = filepath.Join(appPaths.comfyUIDir, "requirements.txt")

		if !venvFound {
			// Warn about both venv and .venv missing
			fmt.Println(warningStyle.Render(fmt.Sprintf("Warning: No Python executable found in either 'venv' or '.venv' in %s. The virtual environment might need setup via the 'Install' option.", appPaths.comfyUIDir)))
			// Default to venv for error messages
			appPaths.venvPath = filepath.Join(appPaths.comfyUIDir, "venv")
			if runtime.GOOS == "windows" {
				appPaths.venvPython = filepath.Join(appPaths.venvPath, "Scripts", "python.exe")
			} else {
				appPaths.venvPython = filepath.Join(appPaths.venvPath, "bin", "python")
			}
		}

		if _, err := os.Stat(appPaths.comfyUIDir); os.IsNotExist(err) {
			fmt.Println(warningStyle.Render(fmt.Sprintf("Warning: Configured ComfyUI directory not found at %s.", appPaths.comfyUIDir)))
			appPaths.isConfigured = false // Mark as not configured if dir doesn't exist
		} else if _, err := os.Stat(appPaths.venvPython); os.IsNotExist(err) && appPaths.isConfigured {
			// Only warn if configured, otherwise user needs to install anyway
			fmt.Println(warningStyle.Render(fmt.Sprintf("Warning: Python executable not found in virtual environment: %s. The env might need setup via the 'Install' option.", appPaths.venvPython)))
		}
	} else {
		fmt.Println(infoStyle.Render(fmt.Sprintf("ComfyUI path is not configured. Use the 'Install/Reconfigure ComfyUI' option to set it up, or create a %s file in %s with COMFYUI_PATH.", envFileName, appPaths.cliDir)))
	}

	envPaths := []string{filepath.Join(appPaths.cliDir, envFileName)}
	// Add platform-specific config paths
	switch runtime.GOOS {
	case "linux":
		xdgConfig := os.Getenv("XDG_CONFIG_HOME")
		if xdgConfig != "" {
			envPaths = append(envPaths, filepath.Join(xdgConfig, "comfy-chair", envFileName))
		}
		home, _ := os.UserHomeDir()
		if home != "" {
			envPaths = append(envPaths, filepath.Join(home, ".config", "comfy-chair", envFileName))
		}
	case "darwin":
		home, _ := os.UserHomeDir()
		if home != "" {
			envPaths = append(envPaths, filepath.Join(home, "Library", "Application Support", "comfy-chair", envFileName))
		}
	case "windows":
		appData := os.Getenv("APPDATA")
		if appData != "" {
			envPaths = append(envPaths, filepath.Join(appData, "comfy-chair", envFileName))
		}
	}

	foundEnv := false
	for _, envPath := range envPaths {
		if _, err := os.Stat(envPath); err == nil {
			appPaths.envFile = envPath
			err = godotenv.Load(appPaths.envFile)
			if err != nil {
				fmt.Println(warningStyle.Render(fmt.Sprintf("Warning: Could not load .env file at %s: %v", appPaths.envFile, err)))
			} else {
				foundEnv = true
				break
			}
		}
	}

	if !foundEnv {
		fmt.Println(errorStyle.Render("No .env file found. Please create a .env file with COMFYUI_PATH set to your ComfyUI installation directory."))
		fmt.Println(infoStyle.Render("Example .env content:"))
		fmt.Println("COMFYUI_PATH=/path/to/your/ComfyUI")
		appPaths.isConfigured = false
		return nil
	}

	if comfyPathFromEnv != "" {
		absComfyPath, pathErr := filepath.Abs(comfyPathFromEnv)
		if pathErr != nil {
			fmt.Println(warningStyle.Render(fmt.Sprintf("Warning: COMFYUI_PATH '%s' from .env is not a valid path: %v. Ignoring.", comfyPathFromEnv, pathErr)))
		} else if stat, err := os.Stat(absComfyPath); err == nil && stat.IsDir() {
			appPaths.comfyUIDir = absComfyPath
			appPaths.isConfigured = true
		} else {
			fmt.Println(warningStyle.Render(fmt.Sprintf("Warning: COMFYUI_PATH '%s' from .env is not a valid directory. Current value: '%s'. Ignoring.", comfyPathFromEnv, absComfyPath)))
		}
	}

	// If not configured via .env, try default location (ComfyUI subdirectory next to CLI)
	if !appPaths.isConfigured {
		defaultComfyDir := filepath.Join(appPaths.cliDir, comfyUIDirNameDefault)
		if stat, err := os.Stat(defaultComfyDir); err == nil && stat.IsDir() {
			appPaths.comfyUIDir = defaultComfyDir // It's already absolute if cliDir is.
			appPaths.isConfigured = true
			fmt.Println(infoStyle.Render(fmt.Sprintf("No %s found or COMFYUI_PATH not set. Using default ComfyUI location: %s", envFileName, appPaths.comfyUIDir)))
		}
	}

	if appPaths.isConfigured {
		// Check for both venv and .venv directories
		venvCandidates := []string{"venv", ".venv"}
		venvFound := false
		for _, venvDir := range venvCandidates {
			candidatePath := filepath.Join(appPaths.comfyUIDir, venvDir)
			if stat, err := os.Stat(candidatePath); err == nil && stat.IsDir() {
				appPaths.venvPath = candidatePath
				if runtime.GOOS == "windows" {
					appPaths.venvPython = filepath.Join(candidatePath, "Scripts", "python.exe")
				} else {
					appPaths.venvPython = filepath.Join(candidatePath, "bin", "python")
				}
				venvFound = true
				break
			}
		}
		appPaths.requirementsTxt = filepath.Join(appPaths.comfyUIDir, "requirements.txt")

		if !venvFound {
			// Warn about both venv and .venv missing
			fmt.Println(warningStyle.Render(fmt.Sprintf("Warning: No Python executable found in either 'venv' or '.venv' in %s. The virtual environment might need setup via the 'Install' option.", appPaths.comfyUIDir)))
			// Default to venv for error messages
			appPaths.venvPath = filepath.Join(appPaths.comfyUIDir, "venv")
			if runtime.GOOS == "windows" {
				appPaths.venvPython = filepath.Join(appPaths.venvPath, "Scripts", "python.exe")
			} else {
				appPaths.venvPython = filepath.Join(appPaths.venvPath, "bin", "python")
			}
		}

		if _, err := os.Stat(appPaths.comfyUIDir); os.IsNotExist(err) {
			fmt.Println(warningStyle.Render(fmt.Sprintf("Warning: Configured ComfyUI directory not found at %s.", appPaths.comfyUIDir)))
			appPaths.isConfigured = false // Mark as not configured if dir doesn't exist
		} else if _, err := os.Stat(appPaths.venvPython); os.IsNotExist(err) && appPaths.isConfigured {
			// Only warn if configured, otherwise user needs to install anyway
			fmt.Println(warningStyle.Render(fmt.Sprintf("Warning: Python executable not found in virtual environment: %s. The venv might need setup via the 'Install' option.", appPaths.venvPython)))
		}
	} else {
		fmt.Println(infoStyle.Render(fmt.Sprintf("ComfyUI path is not configured. Use the 'Install/Reconfigure ComfyUI' option to set it up, or create a %s file in %s with COMFYUI_PATH.", envFileName, appPaths.cliDir)))
	}
	return nil
}

// saveComfyUIPathToEnv saves the COMFYUI_PATH to the .env file.
func saveComfyUIPathToEnv(comfyPath string) error {
	absComfyPath, err := filepath.Abs(comfyPath)
	if err != nil {
		return fmt.Errorf("could not get absolute path for ComfyUI: %w", err)
	}

	envMap := make(map[string]string)
	// Read existing .env if it exists, to preserve other variables
	if _, err := os.Stat(appPaths.envFile); err == nil {
		existingEnv, readErr := godotenv.Read(appPaths.envFile)
		if readErr != nil {
			// Log warning but proceed to overwrite if unreadable
			fmt.Println(warningStyle.Render(fmt.Sprintf("Warning: failed to read existing .env file at %s, it might be corrupted: %v", appPaths.envFile, readErr)))
		} else {
			envMap = existingEnv
		}
	}

	envMap[envComfyUIPathKey] = absComfyPath
	return godotenv.Write(envMap, appPaths.envFile)
}

// checkVenvPython ensures the virtual environment's Python executable exists.
func checkVenvPython() error {
	if !appPaths.isConfigured { // Should be caught earlier, but as a safeguard
		return fmt.Errorf("ComfyUI path is not configured")
	}
	if _, err := os.Stat(appPaths.venvPython); os.IsNotExist(err) {
		return fmt.Errorf("python executable not found in virtual environment: %s. Please ensure ComfyUI is installed correctly and the venv is set up (e.g., via the 'Install/Reconfigure' option)", appPaths.venvPython)
	}
	return nil
}

func startComfyUI(background bool) {
	if err := checkVenvPython(); err != nil {
		fmt.Println(errorStyle.Render(err.Error()))
		return
	}
	action := "foreground"
	if background {
		action = "background"
	}
	fmt.Println(infoStyle.Render(fmt.Sprintf("Starting ComfyUI from %s in the %s...", appPaths.comfyUIDir, action)))

	if pid, isRunning := getRunningPID(); isRunning {
		fmt.Println(warningStyle.Render(fmt.Sprintf("ComfyUI is already running (PID: %d).", pid)))
		return
	}
	// Clean up previous PID file if it exists but process is not running
	if _, err := os.Stat(appPaths.pidFile); err == nil {
		// Check if the PID in file is actually running, otherwise it's stale
		pidFromFile, _ := readPID()
		if pidFromFile != 0 && !isProcessRunning(pidFromFile) {
			fmt.Println(infoStyle.Render(fmt.Sprintf("Removing stale PID file for PID %d.", pidFromFile)))
			os.Remove(appPaths.pidFile)
		}
	}
	// Clean up previous log file for background starts
	if background {
		if _, err := os.Stat(appPaths.logFile); err == nil {
			os.Remove(appPaths.logFile)
		}
	}

	args := []string{"-s", filepath.Join(appPaths.comfyUIDir, "main.py"), "--listen", "--preview-method", "auto", "--front-end-version", "Comfy-Org/ComfyUI_frontend@latest"} // Removed --front-end-version for now, let ComfyUI handle its default
	process, err := executeCommand(appPaths.venvPython, args, appPaths.comfyUIDir, appPaths.logFile, background)
	if err != nil {
		fmt.Println(errorStyle.Render(fmt.Sprintf("Failed to start ComfyUI: %v", err)))
		return
	}

	if background && process != nil {
		err := writePID(process.Pid)
		if err != nil {
			fmt.Println(errorStyle.Render(fmt.Sprintf("Failed to write PID file: %v", err)))
			process.Kill() // Kill the process if we can't manage it
			return
		}
		fmt.Println(successStyle.Render(fmt.Sprintf("ComfyUI started in background. PID: %d. Log: %s", process.Pid, appPaths.logFile)))
		if err := waitForComfyUIReady(); err != nil {
			fmt.Println(warningStyle.Render(fmt.Sprintf("ComfyUI might not be fully operational: %v", err)))
		}
	} else if !background {
		fmt.Println(successStyle.Render("ComfyUI started in foreground. Press Ctrl+C to stop."))
	}
}

func updateComfyUI() {
	if err := checkVenvPython(); err != nil {
		fmt.Println(errorStyle.Render(err.Error()))
		return
	}
	fmt.Println(infoStyle.Render(fmt.Sprintf("Updating ComfyUI at %s...", appPaths.comfyUIDir)))

	// 1. Git Pull
	fmt.Println(infoStyle.Render("Pulling latest changes from Git..."))
	_, err := executeCommand("git", []string{"pull", "origin", "master"}, appPaths.comfyUIDir, "", false)
	if err != nil {
		fmt.Println(errorStyle.Render(fmt.Sprintf("Failed to update ComfyUI (git pull): %v", err)))
		return
	}
	fmt.Println(successStyle.Render("Git pull successful."))

	// 2. Update requirements
	fmt.Println(infoStyle.Render("Updating Python dependencies..."))
	if _, err := os.Stat(appPaths.requirementsTxt); os.IsNotExist(err) {
		fmt.Println(warningStyle.Render(fmt.Sprintf("requirements.txt not found at %s. Skipping dependency update.", appPaths.requirementsTxt)))
		fmt.Println(successStyle.Render("ComfyUI core updated. Dependency update skipped."))
		return
	}
	// Check for uv in PATH
	uvPath, err := exec.LookPath("uv")
	if err != nil {
		fmt.Println(warningStyle.Render("'uv' is not installed or not found in your PATH. Please install it from https://github.com/astral-sh/uv or with 'pipx install uv' or your package manager."))
		fmt.Println(warningStyle.Render("Dependency update skipped. Run the above command, then try again."))
		return
	}
	args := []string{"pip", "install", "-r", appPaths.requirementsTxt}
	_, err = executeCommand(uvPath, args, appPaths.comfyUIDir, "", false)
	if err != nil {
		fmt.Println(errorStyle.Render(fmt.Sprintf("Failed to update dependencies (uv pip install): %v", err)))
		return
	}
	fmt.Println(successStyle.Render("ComfyUI and dependencies updated successfully."))
}

func installComfyUI() {
	fmt.Println(titleStyle.Render("ComfyUI Installation / Reconfiguration"))

	// Check for Git
	if _, err := exec.LookPath("git"); err != nil {
		fmt.Println(errorStyle.Render("Error: Git is not installed or not found in your system PATH. Please install Git and ensure it's in PATH, then try again."))
		return
	}

	var installPath string
	defaultInstallPath := filepath.Join(appPaths.cliDir, comfyUIDirNameDefault) // Default to subdir of CLI
	if appPaths.isConfigured && appPaths.comfyUIDir != "" {
		defaultInstallPath = appPaths.comfyUIDir // If already configured, suggest current path
	}
	absDefaultInstallPath, _ := filepath.Abs(defaultInstallPath)

	var systemPythonExec string
	foundSystemPython, _ := findSystemPython() // Ignore error, will be placeholder if not found

	// Set default values explicitly if they are valid
	if absDefaultInstallPath != "" {
		installPath = absDefaultInstallPath
	}
	if foundSystemPython != "" {
		systemPythonExec = foundSystemPython
	}

	group := huh.NewGroup(
		huh.NewInput().
			Title("Enter the full desired path for your ComfyUI installation").
			Description(fmt.Sprintf("This path will be saved to %s in %s.", envFileName, appPaths.cliDir)).
			Value(&installPath).
			Placeholder(absDefaultInstallPath),
		huh.NewInput().
			Title("Enter path to your system's Python 3 executable").
			Description("This will be used to create the virtual environment (e.g., /usr/bin/python3 or C:\\Python39\\python.exe).").
			Value(&systemPythonExec).
			Placeholder(foundSystemPython),
	)

	form := huh.NewForm(group).WithTheme(huh.ThemeCharm())
	if err := form.Run(); err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			fmt.Println(infoStyle.Render("Installation cancelled."))
		} else {
			fmt.Println(errorStyle.Render(fmt.Sprintf("Form error: %v", err)))
		}
		return
	}

	if strings.TrimSpace(installPath) == "" {
		fmt.Println(errorStyle.Render("Installation path cannot be empty."))
		return
	}
	if strings.TrimSpace(systemPythonExec) == "" {
		fmt.Println(errorStyle.Render("System Python executable path cannot be empty."))
		return
	}

	var err error
	installPath, err = filepath.Abs(strings.TrimSpace(installPath))
	if err != nil {
		fmt.Println(errorStyle.Render(fmt.Sprintf("Invalid installation path %s: %v", installPath, err)))
		return
	}
	systemPythonExec = strings.TrimSpace(systemPythonExec)
	if _, err := os.Stat(systemPythonExec); os.IsNotExist(err) {
		fmt.Println(errorStyle.Render(fmt.Sprintf("Provided Python executable not found at: %s", systemPythonExec)))
		return
	}
	// Basic Python version check
	cmdPyVersion := exec.Command(systemPythonExec, "--version")
	outputPyVersion, errPyVersion := cmdPyVersion.CombinedOutput()
	if errPyVersion != nil {
		fmt.Println(warningStyle.Render(fmt.Sprintf("Could not verify Python version for %s: %v", systemPythonExec, errPyVersion)))
	} else if !strings.Contains(string(outputPyVersion), "Python 3.") {
		fmt.Println(warningStyle.Render(fmt.Sprintf("Warning: %s does not appear to be Python 3. Output: %s", systemPythonExec, string(outputPyVersion))))
		var confirmProceed bool
		proceedForm := huh.NewForm(huh.NewGroup(huh.NewConfirm().Title("The selected Python does not report as Python 3. Continue with installation anyway?").Affirmative("Yes").Negative("No").Value(&confirmProceed))).WithTheme(huh.ThemeCharm())
		if err := proceedForm.Run(); err != nil || !confirmProceed {
			fmt.Println(infoStyle.Render("Installation aborted by user due to Python version concern."))
			return
		}
	}

	// Handle existing directory for installationPath
	targetInfo, err := os.Stat(installPath)
	if err == nil { // Path exists
		if (!targetInfo.IsDir()) {
			fmt.Println(errorStyle.Render(fmt.Sprintf("Error: The path %s exists but is a file, not a directory. Please choose a different path or remove the file.", installPath)))
			return
		}
		gitDir := filepath.Join(installPath, ".git")
		if _, err := os.Stat(gitDir); err == nil { // .git exists, assume it's ComfyUI
			fmt.Println(infoStyle.Render(fmt.Sprintf("Directory %s already exists and appears to be a ComfyUI installation. Will attempt to set up venv and dependencies if needed.", installPath)))
			// Skip clone
		} else { // Directory exists, but not a .git repo
			entries, _ := os.ReadDir(installPath)
			if len(entries) > 0 {
				fmt.Println(errorStyle.Render(fmt.Sprintf("Error: Directory %s exists, is not empty, and does not appear to be a ComfyUI installation (no .git folder). Please choose an empty directory or an existing ComfyUI installation.", installPath)))
				return
			}
			// Directory is empty, can proceed with clone
			if errClone := cloneComfyUI(installPath); errClone != nil {
				return
			}
		}
	} else { // Path does not exist
		if !os.IsNotExist(err) { // Some other error
			fmt.Println(errorStyle.Render(fmt.Sprintf("Error checking path %s: %v", installPath, err)))
			return
		}
		// Path does not exist, create it and clone
		fmt.Println(infoStyle.Render(fmt.Sprintf("Creating directory %s and cloning ComfyUI...", installPath)))
		if errMkdir := os.MkdirAll(installPath, os.ModePerm); errMkdir != nil {
			fmt.Println(errorStyle.Render(fmt.Sprintf("Failed to create directory %s: %v", installPath, errMkdir)))
			return
		}
		if errClone := cloneComfyUI(installPath); errClone != nil {
			return
		}
	}

	// --- At this point, ComfyUI code should be in installPath ---
	// Define paths for the (potentially new) installation
	currentInstallVenvPath := filepath.Join(installPath, venvDirName)
	var currentInstallVenvPython string
	if runtime.GOOS == "windows" {
		currentInstallVenvPython = filepath.Join(currentInstallVenvPath, "Scripts", "python.exe")
	} else {
		currentInstallVenvPython = filepath.Join(currentInstallVenvPath, "bin", "python")
	}
	currentInstallReqTxt := filepath.Join(installPath, "requirements.txt")

	// 2. Create/Verify Virtual Environment
	fmt.Println(infoStyle.Render(fmt.Sprintf("Setting up Python virtual environment at %s using %s...", currentInstallVenvPath, systemPythonExec)))
	if _, err := os.Stat(currentInstallVenvPath); os.IsNotExist(err) {
		_, errCmd := executeCommand(systemPythonExec, []string{"-m", "venv", currentInstallVenvPath}, installPath, "", false)
		if errCmd != nil {
			fmt.Println(errorStyle.Render(fmt.Sprintf("Failed to create virtual environment: %v", errCmd)))
			fmt.Println(infoStyle.Render("Please ensure your system Python can create virtual environments (e.g., `python -m venv test-venv` works)."))
			return
		}
		fmt.Println(successStyle.Render("Virtual environment created."))
	} else if err == nil {
		fmt.Println(infoStyle.Render("Virtual environment already exists. Skipping creation."))
	} else { // Some other error checking venv path
		fmt.Println(errorStyle.Render(fmt.Sprintf("Error checking virtual environment path %s: %v", currentInstallVenvPath, err)))
		return
	}
	// Verify venv python after creation attempt
	if _, err := os.Stat(currentInstallVenvPython); os.IsNotExist(err) {
		fmt.Println(errorStyle.Render(fmt.Sprintf("Failed to find Python executable in created venv: %s. Venv creation might have failed silently or Python path is incorrect.", currentInstallVenvPython)))
		return
	}

	// 3. Install Dependencies
	fmt.Println(infoStyle.Render(fmt.Sprintf("Installing dependencies from %s using uv...", currentInstallReqTxt)))
	if _, err := os.Stat(currentInstallReqTxt); os.IsNotExist(err) {
		fmt.Println(warningStyle.Render(fmt.Sprintf("requirements.txt not found at %s. Skipping dependency installation.", currentInstallReqTxt)))
	} else {
		// Check for uv in PATH
		uvPath, err := exec.LookPath("uv")
		if err != nil {
			fmt.Println(warningStyle.Render("'uv' is not installed or not found in your PATH. Please install it from https://github.com/astral-sh/uv or with 'pipx install uv' or your package manager."))
			fmt.Println(warningStyle.Render("Dependency installation skipped. Run the above command, then try again."))
		} else {
			fmt.Println(infoStyle.Render("Upgrading pip in venv using uv..."))
			pipUpgradeArgs := []string{"pip", "install", "--upgrade", "pip"}
			_, errPipUpgrade := executeCommand(uvPath, pipUpgradeArgs, installPath, "", false)
			if errPipUpgrade != nil {
				fmt.Println(warningStyle.Render(fmt.Sprintf("Failed to upgrade pip with uv, proceeding with dependency install anyway: %v", errPipUpgrade)))
			} else {
				fmt.Println(successStyle.Render("pip upgraded successfully (via uv)."))
			}

			fmt.Println(infoStyle.Render("Installing requirements with uv..."))
			reqArgs := []string{"pip", "install", "-r", currentInstallReqTxt}
			_, errCmd := executeCommand(uvPath, reqArgs, installPath, "", false)
			if errCmd != nil {
				fmt.Println(errorStyle.Render(fmt.Sprintf("Failed to install dependencies with uv: %v", errCmd)))
				return
			}
			fmt.Println(successStyle.Render("Dependencies installed successfully (via uv)."))
		}
	}

	// 4. Save path to .env
	if err := saveComfyUIPathToEnv(installPath); err != nil {
		fmt.Println(errorStyle.Render(fmt.Sprintf("Failed to save ComfyUI path to %s (in %s): %v", envFileName, appPaths.cliDir, err)))
		return
	}
	fmt.Println(successStyle.Render(fmt.Sprintf("ComfyUI path '%s' saved to %s (in %s)", installPath, envFileName, appPaths.cliDir)))

	// Re-initialize paths with the new configuration
	fmt.Println(infoStyle.Render("Re-initializing application paths..."))
	if err := initPaths(); err != nil { // initPaths will print its own messages
		fmt.Println(errorStyle.Render(fmt.Sprintf("Error re-initializing paths after install: %v", err)))
	} else if !appPaths.isConfigured || appPaths.comfyUIDir != installPath {
		fmt.Println(errorStyle.Render(fmt.Sprintf("Failed to properly configure ComfyUI path to '%s' after installation steps. Please check %s and directory permissions.", installPath, appPaths.envFile)))
	} else {
		fmt.Println(successStyle.Render("ComfyUI installation/reconfiguration complete! You can now use other commands."))
	}
}

// cloneComfyUI clones the ComfyUI repository into the specified installPath.
func cloneComfyUI(installPath string) error {
	fmt.Println(infoStyle.Render(fmt.Sprintf("Cloning ComfyUI from %s into %s...", comfyUIRepoURL, installPath)))
	// Git clone creates the final directory, so workDir is its parent.
	parentDir := filepath.Dir(installPath)
	repoDirName := filepath.Base(installPath) // The name of the directory to be created by clone

	_, err := executeCommand("git", []string{"clone", comfyUIRepoURL, repoDirName}, parentDir, "", false)
	if err != nil {
		fmt.Println(errorStyle.Render(fmt.Sprintf("Failed to clone ComfyUI repository: %v", err)))
		// Don't removeall installPath as git clone might have failed for other reasons, and path might contain user data if it existed partially.
		return err
	}
	fmt.Println(successStyle.Render("ComfyUI cloned successfully."))
	return nil
}

// executeCommand runs a command, optionally in the background.
func executeCommand(commandName string, args []string, workDir string, logFilePath string, inBackground bool) (*os.Process, error) {
	cmd := exec.Command(commandName, args...)
	if workDir != "" {
		cmd.Dir = workDir
	}

	if inBackground {
		if logFilePath == "" {
			return nil, fmt.Errorf("logFilePath cannot be empty for background commands")
		}
		logFile, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
		if err != nil {
			return nil, fmt.Errorf("failed to open log file %s: %w", logFilePath, err)
		}
		cmd.Stdout = logFile
		cmd.Stderr = logFile

		// Only set SysProcAttr.Setsid on Unix (not Windows)
		// This avoids build errors on Windows where Setsid is not available.
		if runtime.GOOS != "windows" {
			cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
		}

		err = cmd.Start()
		if err != nil {
			logFile.Close() // Close file if Start fails.
			return nil, fmt.Errorf("failed to start command '%s %s' in background: %w", commandName, strings.Join(args, " "), err)
		}
		// logFile is intentionally kept open as the background process writes to it.
		// It will be closed by the OS when the process exits.
		return cmd.Process, nil
	}

	// Foreground execution
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin // Allow interaction

	err := cmd.Run() // Waits for completion
	if err != nil {
		return nil, fmt.Errorf("command '%s %s' execution failed: %w", commandName, strings.Join(args, " "), err)
	}
	return cmd.Process, nil
}

// writePID writes the process ID to the pidFile.
func writePID(pid int) error {
	return os.WriteFile(appPaths.pidFile, []byte(strconv.Itoa(pid)), 0644)
}

// readPID reads the process ID from the pidFile.
func readPID() (int, error) {
	if _, err := os.Stat(appPaths.pidFile); os.IsNotExist(err) {
		return 0, os.ErrNotExist // Return specific error
	}
	data, err := os.ReadFile(appPaths.pidFile)
	if err != nil {
		return 0, err
	}
	if len(data) == 0 {
		return 0, fmt.Errorf("pid file is empty: %s", appPaths.pidFile)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0, fmt.Errorf("invalid PID in file %s: %w", appPaths.pidFile, err)
	}
	return pid, nil
}

// cleanupPIDFile removes the pidFile.
func cleanupPIDFile() {
	if err := os.Remove(appPaths.pidFile); err != nil && !os.IsNotExist(err) {
		fmt.Println(warningStyle.Render(fmt.Sprintf("Warning: Failed to remove PID file %s: %v", appPaths.pidFile, err)))
	}
}

// isProcessRunning checks if a process with the given PID is currently running.
func isProcessRunning(pid int) bool {
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
	// POSIX: Sending signal 0.
	err = process.Signal(syscall.Signal(0))
	return err == nil
}

// getRunningPID reads PID from file and checks if the process is running.
func getRunningPID() (pid int, isRunning bool) {
	pidRead, err := readPID()
	if err != nil {
		// os.ErrNotExist is normal if ComfyUI not started via this tool's background mode.
		// Other errors (permission, corrupted file) are warnings.
		if !errors.Is(err, os.ErrNotExist) {
			fmt.Println(warningStyle.Render(fmt.Sprintf("Warning: Could not read PID file %s: %v", appPaths.pidFile, err)))
		}
		return 0, false
	}
	if isProcessRunning(pidRead) {
		return pidRead, true
	}
	return pidRead, false // PID read, but process not running (stale PID)
}

func stopComfyUI() {
	pid, isRunning := getRunningPID()

	if !isRunning {
		if pid != 0 { // Stale PID file found
			fmt.Println(infoStyle.Render(fmt.Sprintf("Found stale PID file for PID %d (process not running). Removing PID file: %s", pid, appPaths.pidFile)))
			cleanupPIDFile()
		} else { // No PID file, or unreadable and process not found by other means
			fmt.Println(infoStyle.Render("ComfyUI is not running in the background (or PID file not found/readable)."))
		}
		return
	}

	fmt.Println(infoStyle.Render(fmt.Sprintf("Stopping ComfyUI (PID: %d)...", pid)))
	process, err := os.FindProcess(pid)
	if err != nil { // Should be rare if isProcessRunning was true
		fmt.Println(errorStyle.Render(fmt.Sprintf("Failed to find process with PID %d, though it was reported as running: %v", pid, err)))
		cleanupPIDFile() // Clean up PID file as state is inconsistent
		return
	}

	var killErr error
	if runtime.GOOS == "windows" {
		killErr = process.Kill() // Sends TerminateProcess.
	} else {
		killErr = process.Signal(syscall.SIGTERM) // Graceful termination.
	}

	if killErr != nil {
		fmt.Println(warningStyle.Render(fmt.Sprintf("Failed to send termination signal to PID %d: %v. It might have already exited.", pid, killErr)))
		// Check if it's gone before cleaning up PID file, as it might still be there if signal failed for permission reasons
		if !isProcessRunning(pid) {
			cleanupPIDFile()
		}
		return
	}

	if err := waitForComfyUIStop(pid); err != nil {
		fmt.Println(warningStyle.Render(fmt.Sprintf("ComfyUI (PID: %d) did not stop gracefully: %v. Forcing stop.", pid, err)))
		if forceKillErr := process.Kill(); forceKillErr != nil { // SIGKILL or TerminateProcess again
			fmt.Println(errorStyle.Render(fmt.Sprintf("Failed to force kill PID %d: %v", pid, forceKillErr)))
		} else {
			fmt.Println(infoStyle.Render(fmt.Sprintf("ComfyUI (PID: %d) force killed.", pid)))
		}
	} else {
		fmt.Println(successStyle.Render(fmt.Sprintf("ComfyUI (PID: %d) stopped.", pid)))
	}
	cleanupPIDFile() // Always clean up PID file after stop attempt.
}

func restartComfyUI() {
	if !appPaths.isConfigured {
		fmt.Println(errorStyle.Render(fmt.Sprintf("ComfyUI path is not configured. Please run 'Install/Reconfigure ComfyUI' or set COMFYUI_PATH in %s.", appPaths.envFile)))
		return
	}
	if err := checkVenvPython(); err != nil {
		fmt.Println(errorStyle.Render(err.Error()))
		return
	}

	fmt.Println(infoStyle.Render("Restarting ComfyUI..."))
	pid, isRunning := getRunningPID()
	if isRunning {
		stopComfyUI() // stopComfyUI will print its messages
		fmt.Println(infoStyle.Render("Waiting a few seconds before restarting..."))
		time.Sleep(3 * time.Second) // Give port/resources time to free up
	} else {
		if pid != 0 { // Stale PID file was found and stopComfyUI (called implicitly or explicitly) would have handled it
			fmt.Println(infoStyle.Render("Previous ComfyUI process was not running (stale PID found and cleaned)."))
		} else {
			fmt.Println(infoStyle.Render("ComfyUI was not running. Starting it now."))
		}
	}
	startComfyUI(true) // Restart in background
}

// waitForComfyUIReady waits for ComfyUI to signal it's ready by checking the log file.
func waitForComfyUIReady() error {
	if appPaths.logFile == "" { // Should be set by initPaths
		return fmt.Errorf("log file path is not set, cannot wait for ComfyUI readiness")
	}

	fmt.Println(infoStyle.Render(fmt.Sprintf("Waiting for ComfyUI to be fully operational (checking for '%s' in %s)...", comfyReadyString, appPaths.logFile)))
	startTime := time.Now()

	spinIdx := 0
	ticker := time.NewTicker(250 * time.Millisecond) // Check log file periodically
	defer ticker.Stop()

	// Give the process a moment to create the log file.
	time.Sleep(1 * time.Second)

	for {
		if time.Since(startTime) > maxWaitTime {
			fmt.Print("\r \r") // Clear spinner line
			return fmt.Errorf("timeout: ComfyUI did not signal readiness (string '%s' not found in log) within %v seconds", comfyReadyString, maxWaitTime.Seconds())
		}

		fmt.Printf("\r%s ", spinnerStyle.Render(spinnerFrames[spinIdx]))
		spinIdx = (spinIdx + 1) % len(spinnerFrames)

		logFile, errOpen := os.Open(appPaths.logFile)
		if errOpen == nil {
			scanner := bufio.NewScanner(logFile)
			found := false
			for scanner.Scan() {
				if strings.Contains(scanner.Text(), comfyReadyString) {
					found = true
					break
				}
			}
			logFile.Close() // Close file after scanning

			if errScan := scanner.Err(); errScan != nil && !errors.Is(errScan, io.EOF) {
				fmt.Print("\r \r")
				return fmt.Errorf("error reading log file %s: %w", appPaths.logFile, errScan)
			}
			if found {
				fmt.Print("\r \r")
				fmt.Println(successStyle.Render("ComfyUI is now fully operational."))
				return nil
			}
		} else if !os.IsNotExist(errOpen) {
			// Log file exists but can't be opened for other reasons
			fmt.Print("\r \r")
			return fmt.Errorf("could not open log file %s to check status: %w", appPaths.logFile, errOpen)
		}
		// If log file doesn't exist yet, or string not found, wait for next tick.

		<-ticker.C
	}
}

// waitForComfyUIStop waits for a process to stop running.
func waitForComfyUIStop(pid int) error {
	fmt.Println(infoStyle.Render(fmt.Sprintf("Waiting for ComfyUI (PID: %d) to stop...", pid)))
	startTime := time.Now()

	spinIdx := 0
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()

	for {
		if !isProcessRunning(pid) {
			fmt.Print("\r \r") // Clear spinner line
			return nil         // Process is no longer running
		}

		if time.Since(startTime) > maxWaitTime {
			fmt.Print("\r \r")
			return fmt.Errorf("timeout: ComfyUI (PID: %d) did not stop within %v seconds", pid, maxWaitTime.Seconds())
		}

		fmt.Printf("\r%s ", spinnerStyle.Render(spinnerFrames[spinIdx]))
		spinIdx = (spinIdx + 1) % len(spinnerFrames)

		<-ticker.C
	}
}

func statusComfyUI() {
	pid, isRunning := getRunningPID()
	fmt.Println(titleStyle.Render("ComfyUI Status"))
	if isRunning {
		fmt.Printf("ComfyUI is running (PID: %d)\n", pid)
	} else if pid != 0 {
		fmt.Printf("ComfyUI is not running, but a stale PID file was found (PID: %d)\n", pid)
		fmt.Print("Would you like to remove the stale PID file? [y/N]: ")
		var resp string
		scan := bufio.NewScanner(os.Stdin)
		if scan.Scan() {
			resp = strings.TrimSpace(strings.ToLower(scan.Text()))
		}
		if resp == "y" || resp == "yes" {
			cleanupPIDFile()
			fmt.Println(successStyle.Render("Stale PID file removed."))
		} else {
			fmt.Println(infoStyle.Render("Stale PID file not removed."))
		}
	} else {
		fmt.Println("ComfyUI is not running.")
	}
	fmt.Printf("ComfyUI Path: %s\n", appPaths.comfyUIDir)
	fmt.Printf("Virtualenv: %s\n", appPaths.venvPath)
	fmt.Printf("Python: %s\n", appPaths.venvPython)
	fmt.Printf("Log file: %s\n", appPaths.logFile)
	fmt.Printf(".env file: %s\n", appPaths.envFile)
}

func printUsage() {
	fmt.Println(titleStyle.Render("Comfy Chair CLI Usage"))
	fmt.Println("Usage: comfy-chair [command]")
	fmt.Println("Commands:")
	fmt.Println("  start, start_fg, start-fg         Start ComfyUI in foreground")
	fmt.Println("  background, start_bg, start-bg     Start ComfyUI in background")
	fmt.Println("  stop                               Stop ComfyUI")
	fmt.Println("  restart                            Restart ComfyUI")
	fmt.Println("  update                             Update ComfyUI")
	fmt.Println("  reload                             Watch for changes and reload ComfyUI")
	fmt.Println("  create_node, create-node           Scaffold a new custom node")
	fmt.Println("  list_nodes, list-nodes             List all custom nodes")
	fmt.Println("  delete_node, delete-node           Delete a custom node")
	fmt.Println("  pack_node, pack-node               Pack a custom node into a zip file")
	fmt.Println("  install                            Install or reconfigure ComfyUI")
	fmt.Println("  status                             Show ComfyUI status and environment")
	fmt.Println("  help, --help, -h                   Show this help message")
}

func main() {
	if err := initPaths(); err != nil {
		fmt.Println(errorStyle.Render(fmt.Sprintf("Critical error initializing CLI paths: %v", err)))
		os.Exit(1)
	}

	if len(os.Args) > 1 {
		arg := os.Args[1]
		switch arg {
		case "start", "start_fg", "start-fg":
			startComfyUI(false)
		case "background", "start_bg", "start-bg":
			startComfyUI(true)
		case "stop":
			stopComfyUI()
		case "restart":
			restartComfyUI()
		case "update":
			updateComfyUI()
		case "reload":
			watchDir := filepath.Join(appPaths.comfyUIDir, "custom_nodes")
			exts := []string{".py", ".js", ".css"}
			debounce := 5
			if val := os.Getenv("COMFY_RELOAD_EXTS"); val != "" {
				exts = strings.Split(val, ",")
				for i := range exts {
					exts[i] = strings.TrimSpace(exts[i])
				}
			}
			if val := os.Getenv("COMFY_RELOAD_DEBOUNCE"); val != "" {
				if d, err := strconv.Atoi(val); err == nil && d > 0 {
					debounce = d
				}
			}
			reloadComfyUI(watchDir, debounce, exts)
		case "create_node", "create-node":
			createNewNode()
		case "list_nodes", "list-nodes":
			listCustomNodes()
		case "delete_node", "delete-node":
			deleteCustomNode()
		case "pack_node", "pack-node":
			packNode()
		case "install":
			installComfyUI()
		case "status":
			statusComfyUI()
		case "help", "--help", "-h":
			printUsage()
			os.Exit(0)
		default:
			fmt.Println(errorStyle.Render(fmt.Sprintf("Unknown argument: %s", arg)))
			printUsage()
			os.Exit(1)
		}
		return
	}

	var choice string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title(titleStyle.Render("ComfyUI Manager")).
				Description("Select an action:").
				Options(
					huh.NewOption("Start ComfyUI (Foreground)", "start_fg"),
					huh.NewOption("Start ComfyUI (Background)", "start_bg"),
					huh.NewOption("Stop ComfyUI", "stop"),
					huh.NewOption("Restart ComfyUI (Background)", "restart"),
					huh.NewOption("Update ComfyUI", "update"),
					huh.NewOption("Install/Reconfigure ComfyUI", "install"),
					huh.NewOption("Create New Node", "create_node"),
					huh.NewOption("List Custom Nodes", "list_nodes"),
					huh.NewOption("Delete Custom Node", "delete_node"),
					huh.NewOption("Pack Custom Node", "pack_node"),
					huh.NewOption("Reload ComfyUI on Node Changes", "reload"),
					huh.NewOption("Status (ComfyUI)", "status"),
					huh.NewOption("Exit", "exit"),
				).
				Value(&choice),
		),
	).WithTheme(huh.ThemeCharm())

	err := form.Run()
	if err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			fmt.Println(infoStyle.Render("Operation cancelled by user."))
			os.Exit(0)
		}
		log.Fatal(errorStyle.Render(fmt.Sprintf("Error running form: %v", err)))
	}

	if !appPaths.isConfigured && choice != "install" && choice != "exit" {
		fmt.Println(errorStyle.Render(fmt.Sprintf("ComfyUI path is not configured. Please run 'Install/Reconfigure ComfyUI' or set COMFYUI_PATH in %s (located at %s).", envFileName, appPaths.envFile)))
		os.Exit(1)
	}

	switch choice {
	case "start_fg":
		startComfyUI(false)
	case "start_bg":
		startComfyUI(true)
	case "stop":
		stopComfyUI()
	case "restart":
		restartComfyUI()
	case "update":
		updateComfyUI()
	case "install":
		installComfyUI()
	case "create_node":
		createNewNode()
	case "list_nodes":
		listCustomNodes()
	case "delete_node":
		deleteCustomNode()
	case "pack_node":
		packNode()
	case "reload":
		watchDir := filepath.Join(appPaths.comfyUIDir, "custom_nodes")
		exts := []string{".py", ".js", ".css"}
		debounce := 5
		if val := os.Getenv("COMFY_RELOAD_EXTS"); val != "" {
			exts = strings.Split(val, ",")
			for i := range exts {
				exts[i] = strings.TrimSpace(exts[i])
			}
		}
		if val := os.Getenv("COMFY_RELOAD_DEBOUNCE"); val != "" {
			if d, err := strconv.Atoi(val); err == nil && d > 0 {
				debounce = d
			}
		}
		reloadComfyUI(watchDir, debounce, exts)
	case "status":
		statusComfyUI()
	case "exit":
		fmt.Println(infoStyle.Render("Exiting."))
		os.Exit(0)
	default:
		fmt.Println(warningStyle.Render("Invalid choice."))
	}
}
