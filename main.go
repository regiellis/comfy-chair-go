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

package main

import (
	"bufio"
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

	"github.com/charmbracelet/huh"
	"github.com/joho/godotenv"
	"github.com/regiellis/comfyui-chair-go/internal"
)

// Configuration
const (
	// comfyUIDirNameDefault is used as a default if no .env is found and no ComfyUI dir is chosen by user
	comfyUIDirNameDefault = "ComfyUI"
	maxWaitTime           = 60 * time.Second
	comfyReadyString      = "Starting server" // String to look for in logs
)

// Define the global appPaths for the project, using the struct from internal/utils.go
var appPaths internal.Paths

// Process status cache for performance optimization
type processStatus struct {
	isRunning bool
	lastCheck time.Time
}

type processCache struct {
	mu          sync.RWMutex
	cache       map[int]processStatus
	lastCleanup time.Time
}

// Global process cache instance
var procCache = &processCache{
	cache: make(map[int]processStatus),
}

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

// Helper to strip surrounding quotes from a string
func stripQuotes(s string) string {
	if len(s) >= 2 && ((s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'')) {
		return s[1 : len(s)-1]
	}
	return s
}

// initPaths initializes application paths, prioritizing .env file.
func initPaths() error {
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}
	appPaths.CliDir = internal.ExpandUserPath(filepath.Dir(exePath))
	appPaths.EnvFile = internal.ExpandUserPath(filepath.Join(appPaths.CliDir, internal.EnvFileName))
	appPaths.PidFile = internal.ExpandUserPath(filepath.Join(appPaths.CliDir, internal.ComfyUIPidFile))
	appPaths.LogFile = internal.ExpandUserPath(filepath.Join(appPaths.CliDir, internal.ComfyUILogFile))
	appPaths.IsConfigured = false

	// Load .env file from CLI directory
	err = godotenv.Load(appPaths.EnvFile)
	if (err != nil) && (!os.IsNotExist(err)) {
		fmt.Println(internal.WarningStyle.Render(fmt.Sprintf("Warning: Could not load .env file at %s: %v", appPaths.EnvFile, err)))
		// If .env does not exist, it's not an error yet.
	}

	// .env validation: check required variables
	requiredEnv := []string{internal.ComfyUIPathKey}
	missing := []string{}
	for _, key := range requiredEnv {
		if os.Getenv(key) == "" {
			missing = append(missing, key)
		}
	}
	if len(missing) > 0 {
		fmt.Println(internal.WarningStyle.Render("Warning: The following required .env variables are missing:"))
		for _, key := range missing {
			fmt.Println("  - " + key)
		}
		fmt.Println(internal.InfoStyle.Render("Please set them in your .env file before using all features."))
	}

	comfyPathFromEnv := os.Getenv(internal.ComfyUIPathKey)
	comfyPathFromEnv = stripQuotes(comfyPathFromEnv)

	if comfyPathFromEnv != "" {
		absComfyPath, pathErr := filepath.Abs(comfyPathFromEnv)
		if pathErr != nil {
			fmt.Println(internal.WarningStyle.Render(fmt.Sprintf("Warning: COMFYUI_PATH '%s' from .env is not a valid path: %v. Ignoring.", comfyPathFromEnv, pathErr)))
		} else if stat, err := os.Stat(internal.ExpandUserPath(absComfyPath)); err == nil && stat.IsDir() {
			appPaths.ComfyUIDir = internal.ExpandUserPath(absComfyPath)
			appPaths.IsConfigured = true
		} else {
			fmt.Println(internal.WarningStyle.Render(fmt.Sprintf("Warning: COMFYUI_PATH '%s' from .env is not a valid directory. Current value: '%s'. Ignoring.", comfyPathFromEnv, absComfyPath)))
		}
	}

	// If not configured via .env, try default location (ComfyUI subdirectory next to CLI)
	if !appPaths.IsConfigured {
		defaultComfyDir := internal.ExpandUserPath(filepath.Join(appPaths.CliDir, comfyUIDirNameDefault))
		if stat, err := os.Stat(defaultComfyDir); err == nil && stat.IsDir() {
			appPaths.ComfyUIDir = defaultComfyDir // It's already absolute if cliDir is.
			appPaths.IsConfigured = true
			fmt.Println(internal.InfoStyle.Render(fmt.Sprintf("No %s found or COMFYUI_PATH not set. Using default ComfyUI location: %s", internal.EnvFileName, appPaths.ComfyUIDir)))
		}
	}

	if appPaths.IsConfigured {
		// Use FindVenvPython to set appPaths.VenvPython and appPaths.VenvPath
		venvPython, err := internal.FindVenvPython(appPaths.ComfyUIDir)
		if err == nil {
			appPaths.VenvPython = internal.ExpandUserPath(venvPython)
			appPaths.VenvPath = internal.ExpandUserPath(filepath.Dir(filepath.Dir(venvPython)))
		} else {
			// Default to venv for error messages
			appPaths.VenvPath = internal.ExpandUserPath(filepath.Join(appPaths.ComfyUIDir, ".venv"))
			if runtime.GOOS == "windows" {
				appPaths.VenvPython = internal.ExpandUserPath(filepath.Join(appPaths.VenvPath, "Scripts", "python.exe"))
			} else {
				appPaths.VenvPython = internal.ExpandUserPath(filepath.Join(appPaths.VenvPath, "bin", "python"))
			}
		}

		if _, err := os.Stat(internal.ExpandUserPath(appPaths.ComfyUIDir)); os.IsNotExist(err) {
			fmt.Println(internal.WarningStyle.Render(fmt.Sprintf("Warning: Configured ComfyUI directory not found at %s.", appPaths.ComfyUIDir)))
			appPaths.IsConfigured = false // Mark as not configured if dir doesn't exist
		} else {
			if venvErr := func() error {
				_, err := internal.FindVenvPython(appPaths.ComfyUIDir)
				return err
			}(); venvErr != nil && appPaths.IsConfigured {
				fmt.Println(internal.WarningStyle.Render(fmt.Sprintf("Warning: Python executable not found in 'venv' or '.venv' under %s. The virtual environment might need setup via the 'Install' option.", appPaths.ComfyUIDir)))
			}
		}
	} else {
		fmt.Println(internal.InfoStyle.Render(fmt.Sprintf("ComfyUI path is not configured. Use the 'Install/Reconfigure ComfyUI' option to set it up, or create a %s file in %s with COMFYUI_PATH.", internal.EnvFileName, appPaths.CliDir)))
	}

	envPaths := []string{internal.ExpandUserPath(filepath.Join(appPaths.CliDir, internal.EnvFileName))}
	// Add platform-specific config paths
	switch runtime.GOOS {
	case "linux":
		xdgConfig := os.Getenv("XDG_CONFIG_HOME")
		if xdgConfig != "" {
			envPaths = append(envPaths, internal.ExpandUserPath(filepath.Join(xdgConfig, "comfy-chair", internal.EnvFileName)))
		}
		home, _ := os.UserHomeDir()
		if home != "" {
			envPaths = append(envPaths, internal.ExpandUserPath(filepath.Join(home, ".config", "comfy-chair", internal.EnvFileName)))
		}
	case "darwin":
		home, _ := os.UserHomeDir()
		if home != "" {
			envPaths = append(envPaths, internal.ExpandUserPath(filepath.Join(home, "Library", "Application Support", "comfy-chair", internal.EnvFileName)))
		}
	case "windows":
		appData := os.Getenv("APPDATA")
		if appData != "" {
			envPaths = append(envPaths, internal.ExpandUserPath(filepath.Join(appData, "comfy-chair", internal.EnvFileName)))
		}
	}

	foundEnv := false
	for _, envPath := range envPaths {
		if _, err := os.Stat(internal.ExpandUserPath(envPath)); err == nil {
			appPaths.EnvFile = internal.ExpandUserPath(envPath)
			err = godotenv.Load(appPaths.EnvFile)
			if err != nil {
				fmt.Println(internal.WarningStyle.Render(fmt.Sprintf("Warning: Could not load .env file at %s: %v", appPaths.EnvFile, err)))
			} else {
				foundEnv = true
				break
			}
		}
	}

	if !foundEnv {
		fmt.Println(internal.ErrorStyle.Render("No .env file found. Please create a .env file with COMFYUI_PATH set to your ComfyUI installation directory."))
		fmt.Println(internal.InfoStyle.Render("Example .env content:"))
		fmt.Println("COMFYUI_PATH=/path/to/your/ComfyUI")
		appPaths.IsConfigured = false
		return nil
	}

	if comfyPathFromEnv != "" {
		absComfyPath, pathErr := filepath.Abs(comfyPathFromEnv)
		if pathErr != nil {
			fmt.Println(internal.WarningStyle.Render(fmt.Sprintf("Warning: COMFYUI_PATH '%s' from .env is not a valid path: %v. Ignoring.", comfyPathFromEnv, pathErr)))
		} else if stat, err := os.Stat(internal.ExpandUserPath(absComfyPath)); err == nil && stat.IsDir() {
			appPaths.ComfyUIDir = internal.ExpandUserPath(absComfyPath)
			appPaths.IsConfigured = true
		} else {
			fmt.Println(internal.WarningStyle.Render(fmt.Sprintf("Warning: COMFYUI_PATH '%s' from .env is not a valid directory. Current value: '%s'. Ignoring.", comfyPathFromEnv, absComfyPath)))
		}
	}

	// If not configured via .env, try default location (ComfyUI subdirectory next to CLI)
	if !appPaths.IsConfigured {
		defaultComfyDir := internal.ExpandUserPath(filepath.Join(appPaths.CliDir, comfyUIDirNameDefault))
		if stat, err := os.Stat(defaultComfyDir); err == nil && stat.IsDir() {
			appPaths.ComfyUIDir = defaultComfyDir // It's already absolute if cliDir is.
			appPaths.IsConfigured = true
			fmt.Println(internal.InfoStyle.Render(fmt.Sprintf("No %s found or COMFYUI_PATH not set. Using default ComfyUI location: %s", internal.EnvFileName, appPaths.ComfyUIDir)))
		}
	}

	if appPaths.IsConfigured {
		// Use FindVenvPython to set appPaths.VenvPython and appPaths.VenvPath
		venvPython, err := internal.FindVenvPython(appPaths.ComfyUIDir)
		if err == nil {
			appPaths.VenvPython = internal.ExpandUserPath(venvPython)
			appPaths.VenvPath = internal.ExpandUserPath(filepath.Dir(filepath.Dir(venvPython)))
		} else {
			// Default to venv for error messages
			appPaths.VenvPath = internal.ExpandUserPath(filepath.Join(appPaths.ComfyUIDir, "venv"))
			if runtime.GOOS == "windows" {
				appPaths.VenvPython = internal.ExpandUserPath(filepath.Join(appPaths.VenvPath, "Scripts", "python.exe"))
			} else {
				appPaths.VenvPython = internal.ExpandUserPath(filepath.Join(appPaths.VenvPath, "bin", "python"))
			}
		}

		if _, err := os.Stat(internal.ExpandUserPath(appPaths.ComfyUIDir)); os.IsNotExist(err) {
			fmt.Println(internal.WarningStyle.Render(fmt.Sprintf("Warning: Configured ComfyUI directory not found at %s.", appPaths.ComfyUIDir)))
			appPaths.IsConfigured = false // Mark as not configured if dir doesn't exist
		} else {
			if venvErr := func() error {
				_, err := internal.FindVenvPython(appPaths.ComfyUIDir)
				return err
			}(); venvErr != nil && appPaths.IsConfigured {
				fmt.Println(internal.WarningStyle.Render(fmt.Sprintf("Warning: Python executable not found in virtual environment: %s. The venv might need setup via the 'Install' option.", appPaths.VenvPython)))
			}
		}
	} else {
		fmt.Println(internal.InfoStyle.Render(fmt.Sprintf("ComfyUI path is not configured. Use the 'Install/Reconfigure ComfyUI' option to set it up, or create a %s file in %s with COMFYUI_PATH.", internal.EnvFileName, appPaths.CliDir)))
	}
	return nil
}



func startComfyUI(background bool) {
	inst, err := internal.GetActiveComfyInstall()
	if err != nil {
		fmt.Println(internal.ErrorStyle.Render(err.Error()))
		return
	}
	comfyDir := inst.Path
	venvPython, err := internal.FindVenvPython(internal.ExpandUserPath(comfyDir))
	if err != nil {
		fmt.Println(internal.ErrorStyle.Render(fmt.Sprintf("Python executable not found in 'venv' or '.venv' under %s. Please ensure ComfyUI is installed correctly and the venv is set up (via the 'Install' option).", comfyDir)))
		return
	}
	// Use comfyDir as provided by the user, do not append 'ComfyUI' or any subdir
	pidFile := internal.ExpandUserPath(filepath.Join(comfyDir, "comfyui.pid"))
	logFile := internal.ExpandUserPath(filepath.Join(comfyDir, "comfyui.log"))
	mainPy := internal.ExpandUserPath(filepath.Join(comfyDir, "main.py"))

	action := "foreground"
	if background {
		action = "background"
	}
	fmt.Println(internal.InfoStyle.Render(fmt.Sprintf("Starting ComfyUI from %s in the %s...", comfyDir, action)))

	if pid, isRunning := internal.GetRunningPIDForEnv(pidFile); isRunning {
		fmt.Println(internal.WarningStyle.Render(fmt.Sprintf("ComfyUI is already running (PID: %d).", pid)))
		return
	}
	if _, err := os.Stat(internal.ExpandUserPath(pidFile)); err == nil {
		pidFromFile, _ := internal.ReadPIDForEnv(pidFile)
		if pidFromFile != 0 && !internal.IsProcessRunning(pidFromFile) {
			fmt.Println(internal.InfoStyle.Render(fmt.Sprintf("Removing stale PID file for PID %d.", pidFromFile)))
			os.Remove(internal.ExpandUserPath(pidFile))
		}
	}
	if background {
		if _, err := os.Stat(internal.ExpandUserPath(logFile)); err == nil {
			os.Remove(internal.ExpandUserPath(logFile))
		}
	}

	// Port conflict detection and prompt
	defaultPort := 8188
	chosenPort, err := internal.PromptForPortConflict(defaultPort)
	if err != nil {
		fmt.Println(internal.ErrorStyle.Render(fmt.Sprintf("Could not start ComfyUI: %v", err)))
		return
	}

	args := []string{"-s", mainPy, "--listen", "--port", fmt.Sprintf("%d", chosenPort), "--preview-method", "auto", "--front-end-version", "Comfy-Org/ComfyUI_frontend@latest"}
	process, err := internal.ExecuteCommand(venvPython, args, comfyDir, logFile, background)
	if err != nil {
		fmt.Println(internal.ErrorStyle.Render(fmt.Sprintf("Failed to start ComfyUI: %v", err)))
		return
	}
	if background && process != nil {
		err := internal.WritePIDForEnv(process.Pid, pidFile)
		if err != nil {
			fmt.Println(internal.ErrorStyle.Render(fmt.Sprintf("Failed to write PID file: %v (path: %s)", err, pidFile)))
			process.Kill()
			return
		}
		// Extra debug: check if file exists and print its contents
		if _, statErr := os.Stat(pidFile); statErr != nil {
			fmt.Println(internal.ErrorStyle.Render("PID file was not created"))
		} else {
			pidData, readErr := os.ReadFile(pidFile)
			if readErr != nil {
				fmt.Println(internal.ErrorStyle.Render("Could not read PID file"))
			} else {
				fmt.Println(internal.InfoStyle.Render(fmt.Sprintf("PID file created with PID: %s", string(pidData))))
			}
		}
		fmt.Println(internal.SuccessStyle.Render(fmt.Sprintf("ComfyUI started in background. PID: %d. Log: %s", process.Pid, logFile)))
		if err := waitForComfyUIReady(); err != nil {
			fmt.Println(internal.WarningStyle.Render(fmt.Sprintf("ComfyUI might not be fully operational: %v", err)))
		}
	} else if !background {
		fmt.Println(internal.SuccessStyle.Render("ComfyUI started in foreground. Press Ctrl+C to stop."))
	}
}

func updateComfyUI() {
	inst, err := internal.GetActiveComfyInstall()
	if err != nil {
		fmt.Println(internal.ErrorStyle.Render(err.Error()))
		return
	}
	comfyDir := inst.Path
	venvPython, err := internal.FindVenvPython(internal.ExpandUserPath(comfyDir))
	if err != nil {
		fmt.Println(internal.ErrorStyle.Render(fmt.Sprintf("python executable not found in 'venv' or '.venv' under %s. Please ensure ComfyUI is installed correctly and the venv is set up.", comfyDir)))
		return
	}
	fmt.Println(internal.InfoStyle.Render(fmt.Sprintf("Updating ComfyUI at %s...", comfyDir)))
	fmt.Println(internal.InfoStyle.Render("Pulling latest changes from Git..."))
	pullOut, err := internal.ExecuteCommand("git", []string{"pull", "origin", "master"}, comfyDir, "", false)
	if err != nil {
		pullOutput := ""
		if pullOut != nil {
			pullOutput = fmt.Sprintf("%v", pullOut)
		}
		unstaged := false
		if strings.Contains(pullOutput, "would be overwritten by merge") ||
			strings.Contains(pullOutput, "Please commit your changes or stash them") ||
			strings.Contains(pullOutput, "error: Your local changes to the following files would be overwritten") {
			unstaged = true
		}
		if unstaged {
			fmt.Println(internal.ErrorStyle.Render("Git pull failed due to unstaged or conflicting changes in your ComfyUI directory."))
			fmt.Println(internal.WarningStyle.Render("You must resolve these changes before updating. Choose an action:"))
			var action string
			form := huh.NewForm(
				huh.NewGroup(
					huh.NewSelect[string]().
						Title("How would you like to proceed?").
						Description("Unstaged changes detected. Stash, abort, or resolve manually?").
						Options(
							huh.NewOption("Stash changes and retry update", "stash"),
							huh.NewOption("Abort update", "abort"),
							huh.NewOption("I'll resolve manually, then retry", "manual"),
						).
						Value(&action),
				),
			).WithTheme(huh.ThemeCharm())
			_ = form.Run()
			switch action {
			case "stash":
				fmt.Println(internal.InfoStyle.Render("Stashing local changes..."))
				_, stashErr := internal.ExecuteCommand("git", []string{"stash"}, comfyDir, "", false)
				if stashErr != nil {
					fmt.Println(internal.ErrorStyle.Render(fmt.Sprintf("Failed to stash changes: %v", stashErr)))
					return
				}
				fmt.Println(internal.SuccessStyle.Render("Changes stashed. Retrying update..."))
				_, err2 := executeCommand("git", []string{"pull", "origin", "master"}, comfyDir, "", false)
				if err2 != nil {
					fmt.Println(internal.ErrorStyle.Render(fmt.Sprintf("Git pull still failed: %v", err2)))
					return
				}
				fmt.Println(internal.SuccessStyle.Render("Git pull successful after stashing."))
				var popStash bool
				form2 := huh.NewForm(
					huh.NewGroup(
						huh.NewConfirm().
							Title("Would you like to apply (pop) your stashed changes now?").
							Value(&popStash),
					),
				).WithTheme(huh.ThemeCharm())
				_ = form2.Run()
				if popStash {
					fmt.Println(internal.InfoStyle.Render("Applying stashed changes..."))
					_, popErr := executeCommand("git", []string{"stash", "pop"}, comfyDir, "", false)
					if popErr != nil {
						fmt.Println(internal.ErrorStyle.Render(fmt.Sprintf("Failed to pop stash: %v", popErr)))
					} else {
						fmt.Println(internal.SuccessStyle.Render("Stashed changes applied."))
					}
				} else {
					fmt.Println(internal.InfoStyle.Render("You can apply your stashed changes later with 'git stash pop' in your ComfyUI directory."))
				}
			case "abort":
				fmt.Println(internal.InfoStyle.Render("Update aborted. No changes made."))
				return
			default:
				fmt.Println(internal.InfoStyle.Render("Please resolve the git issue in your ComfyUI directory, then retry the update."))
				return
			}
		} else {
			fmt.Println(internal.ErrorStyle.Render(fmt.Sprintf("Failed to update ComfyUI (git pull): %v", err)))
			return
		}
	} else {
		fmt.Println(internal.SuccessStyle.Render("Git pull successful."))
	}
	fmt.Println(internal.InfoStyle.Render("Updating Python dependencies..."))
	reqTxt := internal.ExpandUserPath(filepath.Join(comfyDir, "requirements.txt"))
	if _, err := os.Stat(reqTxt); os.IsNotExist(err) {
		fmt.Println(internal.WarningStyle.Render(fmt.Sprintf("requirements.txt not found at %s. Skipping dependency update.", reqTxt)))
		fmt.Println(internal.SuccessStyle.Render("ComfyUI core updated. Dependency update skipped."))
		return
	}
	uvPath, err := exec.LookPath("uv")
	if err == nil {
		args := []string{"pip", "install", "-r", reqTxt}
		_, err = executeCommand(uvPath, args, comfyDir, "", false)
		if err != nil {
			fmt.Println(internal.ErrorStyle.Render(fmt.Sprintf("Failed to update dependencies (uv pip install): %v", err)))
			return
		}
		fmt.Println(internal.SuccessStyle.Render("ComfyUI and dependencies updated successfully."))
		internal.PromptReturnToMenu()
		return
	}
	// Fallback to venvPython if uv is not found
	args := []string{"pip", "install", "-r", reqTxt}
	_, err = executeCommand(venvPython, args, comfyDir, "", false)
	if err != nil {
		fmt.Println(internal.ErrorStyle.Render(fmt.Sprintf("Failed to update dependencies (pip install): %v", err)))
		return
	}
	fmt.Println(internal.SuccessStyle.Render("ComfyUI and dependencies updated successfully."))
	internal.PromptReturnToMenu()
}

func installComfyUI() {
	fmt.Println(internal.TitleStyle.Render("ComfyUI Installation / Reconfiguration"))

	// 1. Prompt for environment type
	var envType string
	formEnv := huh.NewForm(huh.NewGroup(
		huh.NewSelect[string]().
			Title("Which environment is this install for?").
			Options(
				huh.NewOption("Lounge (Main/Stable)", "lounge"),
				huh.NewOption("Den (Dev/Alternate)", "den"),
				huh.NewOption("Nook (Experimental/Test)", "nook"),
				huh.NewOption("Custom", "custom"),
			).
			Value(&envType),
	)).WithTheme(huh.ThemeCharm())
	_ = formEnv.Run()
	if envType == "" {
		fmt.Println(internal.InfoStyle.Render("Installation cancelled."))
		return
	}

	// 2. Prompt for install path
	var installPath string
	formPath := huh.NewForm(huh.NewGroup(
		huh.NewInput().Title("Enter the full desired path for your ComfyUI installation").Value(&installPath),
	)).WithTheme(huh.ThemeCharm())
	_ = formPath.Run()
	installPath = strings.TrimSpace(installPath)
	if installPath == "" {
		fmt.Println(internal.ErrorStyle.Render("Installation path cannot be empty."))
		return
	}
	installPath, _ = filepath.Abs(installPath)

	// Prompt for GPU_TYPE and PYTHON_VERSION if not set in .env
	envVars, _ := internal.ReadEnvFile(appPaths.EnvFile)
	gpuType := envVars["GPU_TYPE"]
	pythonVersion := envVars["PYTHON_VERSION"]
	if gpuType == "" {
		var gpuOpt string
		form := huh.NewForm(huh.NewGroup(
			huh.NewSelect[string]().
				Title("Select your GPU type for PyTorch install").
				Options(
					huh.NewOption("Nvidia (CUDA)", "nvidia"),
					huh.NewOption("AMD (ROCm, Linux only)", "amd"),
					huh.NewOption("Intel (Arc/XPU)", "intel"),
					huh.NewOption("Apple Silicon (Metal)", "apple"),
					huh.NewOption("DirectML (AMD/Windows)", "directml"),
					huh.NewOption("Ascend NPU", "ascend"),
					huh.NewOption("Cambricon MLU", "cambricon"),
					huh.NewOption("CPU Only", "cpu"),
				).
				Value(&gpuOpt),
		)).WithTheme(huh.ThemeCharm())
		if internal.HandleFormError(form.Run(), "GPU type selection") {
			return
		}
		gpuType = gpuOpt
	}
	if pythonVersion == "" {
		var pyVer string = "3.12"
		form := huh.NewForm(huh.NewGroup(
			huh.NewInput().Title("Python version for venv (3.12 recommended, 3.13 supported)").Value(&pyVer).Placeholder("3.12"),
		)).WithTheme(huh.ThemeCharm())
		if internal.HandleFormError(form.Run(), "Python version selection") {
			return
		}
		pythonVersion = pyVer
	}
	internal.UpdateEnvFile(appPaths.EnvFile, map[string]string{"GPU_TYPE": gpuType, "PYTHON_VERSION": pythonVersion})

	// AMD-specific: offer Zluda option
	comfyRepo := internal.ComfyUIRepoURL
	if gpuType == "amd" {
		var zluda bool
		formZluda := huh.NewForm(huh.NewGroup(
			huh.NewConfirm().Title("Would you like to use the ComfyUI-Zluda repo for AMD (experimental, for running CUDA on AMD)?").Value(&zluda),
		)).WithTheme(huh.ThemeCharm())
		_ = formZluda.Run()
		if zluda {
			comfyRepo = "git@github.com:patientx/ComfyUI-Zluda.git"
			fmt.Println(internal.WarningStyle.Render("Using ComfyUI-Zluda repo for AMD (experimental)."))
		} else {
			fmt.Println(internal.InfoStyle.Render("Using standard ComfyUI repo for AMD."))
		}
	}

	// 3. Check for existing install/config
	exists := false
	if stat, err := os.Stat(internal.ExpandUserPath(installPath)); err == nil && stat.IsDir() {
		exists = true
	}
	isLounge := envType == "lounge"
	if exists {
		// Check for ComfyUI files
		gitDir := internal.ExpandUserPath(filepath.Join(installPath, ".git"))
		comfyFound := false
		if _, err := os.Stat(gitDir); err == nil {
			comfyFound = true
		}
		if comfyFound {
			var action string
			form := huh.NewForm(huh.NewGroup(
				huh.NewSelect[string]().
					Title("ComfyUI install detected at this location. What would you like to do?").
					Options(
						huh.NewOption("Update existing install (recommended)", "update"),
						huh.NewOption("Replace (delete and reinstall)", "replace"),
						huh.NewOption("Cancel", "cancel"),
					).
					Value(&action),
			)).WithTheme(huh.ThemeCharm())
			if internal.HandleFormError(form.Run(), "Installation action selection") {
				return
			}
			if action == "cancel" || action == "" {
				fmt.Println(internal.InfoStyle.Render("Installation cancelled."))
				return
			}
			if action == "update" {
				fmt.Println(internal.InfoStyle.Render("Updating existing ComfyUI install..."))
				updateComfyUI()
				return
			}
			if action == "replace" {
				if isLounge {
					var confirm bool
					form2 := huh.NewForm(huh.NewGroup(huh.NewConfirm().Title("You are about to REPLACE the Lounge (source of truth) install. Are you absolutely sure?").Value(&confirm))).WithTheme(huh.ThemeCharm())
					_ = form2.Run()
					if !confirm {
						fmt.Println(internal.InfoStyle.Render("Lounge replacement cancelled."))
						return
					}
				}
				fmt.Println(internal.WarningStyle.Render("Deleting existing install..."))
				_ = os.RemoveAll(internal.ExpandUserPath(installPath))
			}
		}
		// If not ComfyUI, but dir exists, prompt to replace or cancel
		if !comfyFound {
			var confirm bool
			form := huh.NewForm(huh.NewGroup(huh.NewConfirm().Title("Directory exists but does not appear to be ComfyUI. Replace it?").Value(&confirm))).WithTheme(huh.ThemeCharm())
			_ = form.Run()
			if !confirm {
				fmt.Println(internal.InfoStyle.Render("Installation cancelled."))
				return
			}
			_ = os.RemoveAll(internal.ExpandUserPath(installPath))
		}
	}

	// 4. Proceed with install (clone, venv, deps)
	// Check for Git
	if _, err := exec.LookPath("git"); err != nil {
		fmt.Println(internal.ErrorStyle.Render("Error: Git is not installed or not found in your system PATH. Please install Git and ensure it's in PATH, then try again."))
		return
	}

	// Try to detect python in venv/.venv
	venvCandidates := []string{"venv", ".venv"}
	systemPythonExec := ""
	venvFound := false
	for _, venvDir := range venvCandidates {
		candidatePath := internal.ExpandUserPath(filepath.Join(installPath, venvDir))
		if stat, err := os.Stat(candidatePath); err == nil && stat.IsDir() {
			venvFound = true
			if runtime.GOOS == "windows" {
				systemPythonExec = internal.ExpandUserPath(filepath.Join(candidatePath, "Scripts", "python.exe"))
			} else {
				systemPythonExec = internal.ExpandUserPath(filepath.Join(candidatePath, "bin", "python3"))
				if _, err := os.Stat(systemPythonExec); os.IsNotExist(err) {
					systemPythonExec = internal.ExpandUserPath(filepath.Join(candidatePath, "bin", "python"))
				}
			}
			if _, err := os.Stat(systemPythonExec); err == nil {
				break
			}
		}
	}
	if !venvFound || systemPythonExec == "" {
		// Try to find system python
		foundSystemPython, _ := findSystemPython()
		if foundSystemPython == "" {
			fmt.Println(internal.ErrorStyle.Render("Could not find a Python 3 executable in your system PATH. Please install Python 3 and try again."))
			return
		}
		systemPythonExec = foundSystemPython
		fmt.Println(internal.InfoStyle.Render(fmt.Sprintf("Using detected system Python: %s", systemPythonExec)))
	}

	installPath, err := filepath.Abs(strings.TrimSpace(installPath))
	if err != nil {
		fmt.Println(internal.ErrorStyle.Render(fmt.Sprintf("Invalid installation path %s: %v", installPath, err)))
		return
	}
	systemPythonExec = strings.TrimSpace(systemPythonExec)
	if _, err := os.Stat(systemPythonExec); os.IsNotExist(err) {
		fmt.Println(internal.ErrorStyle.Render(fmt.Sprintf("Provided Python executable not found at: %s", systemPythonExec)))
		return
	}
	// Basic Python version check
	cmdPyVersion := exec.Command(systemPythonExec, "--version")
	outputPyVersion, errPyVersion := cmdPyVersion.CombinedOutput()
	if errPyVersion != nil {
		fmt.Println(internal.WarningStyle.Render(fmt.Sprintf("Could not verify Python version for %s: %v", systemPythonExec, errPyVersion)))
	} else if !strings.Contains(string(outputPyVersion), "Python 3.") {
		fmt.Println(internal.WarningStyle.Render(fmt.Sprintf("Warning: %s does not appear to be Python 3. Output: %s", systemPythonExec, string(outputPyVersion))))
		var confirmProceed bool
		proceedForm := huh.NewForm(huh.NewGroup(huh.NewConfirm().Title("The selected Python does not report as Python 3. Continue with installation anyway?").Affirmative("Yes").Negative("No").Value(&confirmProceed))).WithTheme(huh.ThemeCharm())
		if err := proceedForm.Run(); err != nil || !confirmProceed {
			fmt.Println(internal.InfoStyle.Render("Installation aborted by user due to Python version concern."))
			return
		}
	}

	// Handle existing directory for installationPath
	targetInfo, err := os.Stat(internal.ExpandUserPath(installPath))
	if err == nil { // Path exists
		if !targetInfo.IsDir() {
			fmt.Println(internal.ErrorStyle.Render(fmt.Sprintf("Error: The path %s exists but is a file, not a directory. Please choose a different path or remove the file.", installPath)))
			return
		}
		gitDir := internal.ExpandUserPath(filepath.Join(installPath, ".git"))
		if _, err := os.Stat(gitDir); err == nil { // .git exists, assume it's ComfyUI
			fmt.Println(internal.InfoStyle.Render(fmt.Sprintf("Directory %s already exists and appears to be a ComfyUI installation. Will attempt to set up venv and dependencies if needed.", installPath)))
			// Skip clone
		} else { // Directory exists, but not a .git repo
			entries, _ := os.ReadDir(internal.ExpandUserPath(installPath))
			if len(entries) > 0 {
				fmt.Println(internal.ErrorStyle.Render(fmt.Sprintf("Error: Directory %s exists, is not empty, and does not appear to be a ComfyUI installation (no .git folder). Please choose an empty directory or an existing ComfyUI installation.", installPath)))
				return
			}
			// Directory is empty, can proceed with clone
			if errClone := internal.CloneComfyUI(comfyRepo, installPath, executeCommand); errClone != nil {
				return
			}
		}
	} else { // Path does not exist
		if !os.IsNotExist(err) { // Some other error
			fmt.Println(internal.ErrorStyle.Render(fmt.Sprintf("Error checking path %s: %v", installPath, err)))
			return
		}
		// Path does not exist, create it and clone
		fmt.Println(internal.InfoStyle.Render(fmt.Sprintf("Creating directory %s and cloning ComfyUI...", installPath)))
		if errMkdir := os.MkdirAll(internal.ExpandUserPath(installPath), os.ModePerm); errMkdir != nil {
			fmt.Println(internal.ErrorStyle.Render(fmt.Sprintf("Failed to create directory %s: %v", installPath, errMkdir)))
			return
		}
		if errClone := internal.CloneComfyUI(comfyRepo, installPath, executeCommand); errClone != nil {
			return
		}
	}

	// --- At this point, ComfyUI code should be in installPath ---
	// Define paths for the (potentially new) installation
	currentInstallVenvPath := internal.ExpandUserPath(filepath.Join(installPath, internal.VenvDir))
	var currentInstallVenvPython string
	if runtime.GOOS == "windows" {
		currentInstallVenvPython = internal.ExpandUserPath(filepath.Join(currentInstallVenvPath, "Scripts", "python.exe"))
	} else {
		currentInstallVenvPython = internal.ExpandUserPath(filepath.Join(currentInstallVenvPath, "bin", "python"))
	}
	currentInstallReqTxt := internal.ExpandUserPath(filepath.Join(installPath, "requirements.txt"))

	// 2. Create/Verify Virtual Environment
	fmt.Println(internal.InfoStyle.Render(fmt.Sprintf("Setting up Python virtual environment at %s...", currentInstallVenvPath)))
	uvPath, uvErr := exec.LookPath("uv")
	venvCreated := false
	if uvErr == nil {
		fmt.Println(internal.InfoStyle.Render("Using 'uv venv' to create virtual environment..."))
		venvArgs := []string{"venv", "--relocatable", "--python", "3.12", "--python-preference", "only-managed", currentInstallVenvPath}
		_, errCmd := executeCommand(uvPath, venvArgs, installPath, "", false)
		if errCmd != nil {
			fmt.Println(internal.WarningStyle.Render(fmt.Sprintf("'uv venv' failed: %v. Falling back to python -m venv...", errCmd)))
		} else {
			venvCreated = true
			fmt.Println(internal.SuccessStyle.Render("Virtual environment created with uv."))
		}
	}
	if !venvCreated {
		fmt.Println(internal.InfoStyle.Render("Using 'python -m venv' to create virtual environment..."))
		_, errCmd := executeCommand(systemPythonExec, []string{"-m", "venv", currentInstallVenvPath}, installPath, "", false)
		if errCmd != nil {
			fmt.Println(internal.ErrorStyle.Render(fmt.Sprintf("Failed to create virtual environment: %v", errCmd)))
			fmt.Println(internal.InfoStyle.Render("Please ensure your system Python can create virtual environments (e.g., `python -m venv test-venv` works)."))
			return
		}
		fmt.Println(internal.SuccessStyle.Render("Virtual environment created with python."))
	}

	// Activate the venv for the next commands (only needed for shell, not subprocess)
	// Instead, just use the venv's python/uv directly for pip install

	// Verify venv python after creation attempt
	if _, err := os.Stat(currentInstallVenvPython); os.IsNotExist(err) {
		fmt.Println(internal.ErrorStyle.Render(fmt.Sprintf("Failed to find Python executable in created venv: %s. Venv creation might have failed silently or Python path is incorrect.", currentInstallVenvPython)))
		return
	}

	// 3. Install Dependencies
	fmt.Println(internal.InfoStyle.Render(fmt.Sprintf("Installing dependencies from %s using uv...", currentInstallReqTxt)))
	if _, err := os.Stat(currentInstallReqTxt); os.IsNotExist(err) {
		fmt.Println(internal.WarningStyle.Render(fmt.Sprintf("requirements.txt not found at %s. Skipping dependency installation.", currentInstallReqTxt)))
	} else {
		// Prefer venv's uv if available
		venvBin := internal.ExpandUserPath(filepath.Join(currentInstallVenvPath, "bin"))
		uvPath = internal.ExpandUserPath(filepath.Join(venvBin, "uv"))
		uvExec := ""
		if stat, err := os.Stat(uvPath); err == nil && !stat.IsDir() {
			uvExec = uvPath
		} else if uvPath, err := exec.LookPath("uv"); err == nil {
			uvExec = uvPath
		}
		if uvExec == "" {
			fmt.Println(internal.WarningStyle.Render("'uv' is not installed or not found in your PATH. Please install it from https://github.com/astral-sh/uv or with 'pipx install uv' or your package manager."))
			fmt.Println(internal.WarningStyle.Render("Dependency installation skipped. Run the above command, then try again."))
		} else {
			fmt.Println(internal.InfoStyle.Render("Installing requirements with uv (in venv context)..."))
			reqArgs := []string{"pip", "install", "-r", currentInstallReqTxt}
			cmd := exec.Command(uvExec, reqArgs...)
			cmd.Dir = installPath
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			cmd.Env = append(os.Environ(),
				"VIRTUAL_ENV="+currentInstallVenvPath,
				"PATH="+venvBin+":"+os.Getenv("PATH"),
			)
			if err := cmd.Run(); err != nil {
				fmt.Println(internal.ErrorStyle.Render(fmt.Sprintf("Failed to install dependencies with uv: %v", err)))
				return
			}
			fmt.Println(internal.SuccessStyle.Render("Dependencies installed successfully (via uv)."))
		}
	}

	// --- Install default custom nodes ---
	defaultNodes := []struct {
		Name string
		Repo string
	}{
		{"ComfyUI-Manager", "https://github.com/Comfy-Org/ComfyUI-Manager.git"},
		{"ComfyUI-Crystools", "https://github.com/crystian/ComfyUI-Crystools.git"},
		{"rgthree-comfy", "https://github.com/rgthree/rgthree-comfy"},
	}
	customNodesDir := internal.ExpandUserPath(filepath.Join(installPath, internal.CustomNodesDir))
	if err := os.MkdirAll(customNodesDir, 0755); err != nil {
		fmt.Println(internal.ErrorStyle.Render(fmt.Sprintf("Failed to create custom_nodes directory: %v", err)))
		return
	}
	for _, node := range defaultNodes {
		nodePath := internal.ExpandUserPath(filepath.Join(customNodesDir, node.Name))
		if _, err := os.Stat(nodePath); os.IsNotExist(err) {
			fmt.Println(internal.InfoStyle.Render("Cloning default node: " + node.Name))
			cmd := exec.Command(internal.GitCommand, internal.GitClone, node.Repo, nodePath)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			if err := cmd.Run(); err != nil {
				fmt.Println(internal.ErrorStyle.Render(fmt.Sprintf("Failed to clone node %s: %v", node.Name, err)))
				continue
			}
			fmt.Println(internal.SuccessStyle.Render(fmt.Sprintf("Successfully cloned node: %s", node.Name)))
		}
	}

	// --- Install comfy-cli in venv ---
	venvBin := internal.ExpandUserPath(filepath.Join(currentInstallVenvPath, "bin"))
	uvPath = internal.ExpandUserPath(filepath.Join(venvBin, "uv"))
	if _, err := os.Stat(uvPath); err == nil {
		fmt.Println(internal.InfoStyle.Render("Installing comfy-cli in virtual environment..."))
		cmd := exec.Command(uvPath, "pip", "install", "comfy-cli")
		cmd.Dir = installPath
		cmd.Env = append(os.Environ(), "PATH="+venvBin+":"+os.Getenv("PATH"))
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			fmt.Println(internal.ErrorStyle.Render(fmt.Sprintf("Failed to install comfy-cli: %v", err)))
		} else {
			fmt.Println(internal.SuccessStyle.Render("Successfully installed comfy-cli"))
		}
	}

	// --- Update comfy-installs.json (global config) ---
	cfg, _ := internal.LoadGlobalConfig()
	realComfyPath := filepath.Join(installPath)
	newInstall := internal.ComfyInstall{
		Name:      envType,
		Type:      internal.InstallType(envType),
		Path:      realComfyPath,
		IsDefault: false, // Do not set as default/active automatically
	}
	cfg.AddOrUpdateInstall(newInstall)
	if err := internal.SaveGlobalConfig(cfg); err != nil {
		fmt.Println(internal.ErrorStyle.Render(fmt.Sprintf("Failed to update comfy-installs.json: %v", err)))
		return
	}
	fmt.Println(internal.SuccessStyle.Render("ComfyUI installation/reconfiguration complete!"))
	fmt.Println(internal.InfoStyle.Render("This environment is now available in comfy-installs.json."))
	fmt.Println(internal.InfoStyle.Render("To make it active, use the 'set active' or 'set_working_env' command. The .env file was NOT changed."))
}

// executeCommand runs a command, optionally in the background.
func executeCommand(commandName string, args []string, workDir string, logFilePath string, inBackground bool) (*os.Process, error) {
	// Basic validation: command name should not be empty and should not contain path separators unless it's an absolute path
	if commandName == "" {
		return nil, fmt.Errorf("command name cannot be empty")
	}
	
	// Validate working directory if provided
	if workDir != "" {
		cleanWorkDir := filepath.Clean(internal.ExpandUserPath(workDir))
		// Ensure the work directory exists or is creatable
		if _, err := os.Stat(cleanWorkDir); err != nil && !os.IsNotExist(err) {
			return nil, fmt.Errorf("invalid working directory: %w", err)
		}
	}
	
	cmd := exec.Command(commandName, args...)
	if workDir != "" {
		cmd.Dir = internal.ExpandUserPath(workDir)
	}

	if inBackground {
		if logFilePath == "" {
			return nil, fmt.Errorf("logFilePath cannot be empty for background commands")
		}
		logFile, err := os.OpenFile(internal.ExpandUserPath(logFilePath), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
		if err != nil {
			return nil, fmt.Errorf("failed to open log file %s: %w", logFilePath, err)
		}
		cmd.Stdout = logFile
		cmd.Stderr = logFile

		// Call the platform-specific function to configure SysProcAttr
		configureCmdSysProcAttr(cmd)

		err = cmd.Start()
		if err != nil {
			logFile.Close() // Close file if Start fails.
			return nil, fmt.Errorf("failed to start command '%s %s' in background: %w", commandName, strings.Join(args, " "), err)
		}
		
		// Start a goroutine to monitor the process and close the file when it exits
		go func() {
			defer logFile.Close()
			_ = cmd.Wait() // Wait for the process to exit, then close the log file
		}()
		
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

// readPID reads the process ID from the pidFile.
func readPID() (int, error) {
	if _, err := os.Stat(internal.ExpandUserPath(appPaths.PidFile)); os.IsNotExist(err) {
		return 0, os.ErrNotExist // Return specific error
	}
	data, err := os.ReadFile(internal.ExpandUserPath(appPaths.PidFile))
	if err != nil {
		return 0, err
	}
	if len(data) == 0 {
		return 0, fmt.Errorf("pid file is empty: %s", appPaths.PidFile)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0, fmt.Errorf("invalid PID in file %s: %w", appPaths.PidFile, err)
	}
	return pid, nil
}

// cleanupPIDFile removes the pidFile.
func cleanupPIDFile() {
	if err := os.Remove(internal.ExpandUserPath(appPaths.PidFile)); err != nil && !os.IsNotExist(err) {
		fmt.Println(internal.WarningStyle.Render(fmt.Sprintf("Warning: Failed to remove PID file %s: %v", appPaths.PidFile, err)))
	}
}




// getRunningPID reads PID from file and checks if the process is running.
func getRunningPID() (pid int, isRunning bool) {
	pidRead, err := readPID()
	if err != nil {
		// os.ErrNotExist is normal if ComfyUI not started via this tool's background mode.
		// Other errors (permission, corrupted file) are warnings.
		if !errors.Is(err, os.ErrNotExist) {
			fmt.Println(internal.WarningStyle.Render(fmt.Sprintf("Warning: Could not read PID file %s: %v", appPaths.PidFile, err)))
		}
		return 0, false
	}
	if internal.IsProcessRunning(pidRead) {
		return pidRead, true
	}
	return pidRead, false // PID read, but process not running (stale PID)
}




func removeEnv() {
	// Prompt for environment to remove
	cfg, err := internal.LoadGlobalConfig()
	if err != nil || len(cfg.Installs) == 0 {
		fmt.Println(internal.ErrorStyle.Render("No environments configured to remove."))
		return
	}
	// Prepare options
	var envOptions []huh.Option[string]
	for _, inst := range cfg.Installs {
		label := string(inst.Type)
		if inst.IsDefault {
			label += " (default)"
		}
		if inst.Name != "" && inst.Name != string(inst.Type) {
			label += " - " + inst.Name
		}
		envOptions = append(envOptions, huh.NewOption(label, string(inst.Type)))
	}
	var selectedEnv string
	form := huh.NewForm(huh.NewGroup(huh.NewSelect[string]().Title("Select environment to remove (disconnect):").Options(envOptions...).Value(&selectedEnv))).WithTheme(huh.ThemeCharm())
	_ = form.Run()
	if selectedEnv == "" {
		fmt.Println(internal.InfoStyle.Render("No environment selected. Operation cancelled."))
		return
	}
	inst := cfg.FindInstallByType(internal.InstallType(selectedEnv))
	if inst == nil {
		fmt.Println(internal.ErrorStyle.Render("Selected environment not found in config."))
		return
	}
	if inst.Type == internal.LoungeInstall {
		fmt.Println(internal.WarningStyle.Render("Warning: You are about to disconnect the Lounge (main) environment. This is your primary environment!"))
	}
	var confirm bool
	form2 := huh.NewForm(huh.NewGroup(huh.NewConfirm().Title("Are you sure you want to disconnect this environment? This will NOT delete any files on disk.").Value(&confirm))).WithTheme(huh.ThemeCharm())
	_ = form2.Run()
	if !confirm {
		fmt.Println(internal.InfoStyle.Render("Operation cancelled by user."))
		return
	}
	cfg.RemoveInstallByType(inst.Type)
	if err := internal.SaveGlobalConfig(cfg); err != nil {
		fmt.Println(internal.ErrorStyle.Render("Failed to update config: " + err.Error()))
		return
	}
	fmt.Println(internal.SuccessStyle.Render("Environment disconnected from Comfy Chair config."))
	// Platform-specific instructions
	fmt.Println(internal.InfoStyle.Render("\nTo delete the environment files from disk, run the following command in your terminal:"))
	if runtime.GOOS == "windows" {
		fmt.Println("  rmdir /S /Q \"" + inst.Path + "\"")
	} else {
		fmt.Println("  rm -rf \"" + inst.Path + "\"")
	}
	fmt.Println(internal.WarningStyle.Render("This will permanently delete all files in the environment directory. Use with caution!"))
}

func printUsage() {
	fmt.Println(internal.TitleStyle.Render("Comfy Chair CLI Usage"))
	fmt.Println("Usage: comfy-chair [command]")
	fmt.Println("Commands:")
	fmt.Println("  start, start-fg                   Start ComfyUI in foreground")
	fmt.Println("  background, start-bg              Start ComfyUI in background")
	fmt.Println("  stop                              Stop ComfyUI")
	fmt.Println("  restart                           Restart ComfyUI")
	fmt.Println("  update                            Update ComfyUI")
	fmt.Println("  reload                            Watch for changes and reload ComfyUI")
	fmt.Println("  create-node                       Scaffold a new custom node")
	fmt.Println("  list-nodes                        List all custom nodes")
	fmt.Println("  delete-node                       Delete a custom node")
	fmt.Println("  pack-node                         Pack a custom node into a zip file")
	fmt.Println("  install                           Install or reconfigure ComfyUI")
	fmt.Println("  status                            Show ComfyUI status and environment")
	fmt.Println("  update-nodes                      Update selected or all custom nodes using uv")
	fmt.Println("  watch_nodes                       Custom nodes to watch (all others excluded)")
	fmt.Println("  sync-env                          Sync .env with .env.example")
	fmt.Println("  migrate-nodes                     Migrate custom nodes between environments")
	fmt.Println("  migrate-workflows                 Migrate workflows from one environment to another")
	fmt.Println("  migrate-images					 Migrate images/videos/audio from one environment to another")
	fmt.Println("  node-workflows                    Lets you migrate workflows from selected custom nodes")
	fmt.Println("  remove_env                        Remove (disconnect) an environment from config")
	fmt.Println("  help, --help, -h                  Show this help message")
}

func main() {
	if err := initPaths(); err != nil {
		fmt.Println(internal.ErrorStyle.Render(fmt.Sprintf("Critical error initializing CLI paths: %v", err)))
		os.Exit(1)
	}

	// Support short CLI commands (e.g., comfy-chair start, stop, update, etc.)
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "start", "start_fg", "start-fg":
			internal.RunWithEnvConfirmation("start", func(inst *internal.ComfyInstall) { startComfyUIWithEnv(inst, false) })
			return
		case "background", "start_bg", "start-bg":
			internal.RunWithEnvConfirmation("start", func(inst *internal.ComfyInstall) { startComfyUIWithEnv(inst, true) })
			return
		case "stop":
			internal.RunWithEnvConfirmation("stop", func(inst *internal.ComfyInstall) { stopComfyUIWithEnv(inst) })
			return
		case "restart":
			internal.RunWithEnvConfirmation("restart", func(inst *internal.ComfyInstall) { restartComfyUIWithEnv(inst) })
			return
		case "update":
			internal.RunWithEnvConfirmation("update", func(inst *internal.ComfyInstall) { updateComfyUIWithEnv(inst) })
			return
		case "reload":
			inst, err := internal.GetActiveComfyInstall()
			if err != nil {
				fmt.Println(internal.ErrorStyle.Render(err.Error()))
				return
			}
			watchDir := internal.ExpandUserPath(filepath.Join(inst.Path, "custom_nodes"))
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
			includedDirs := inst.ReloadIncludeDirs
			if len(includedDirs) == 0 {
				entries, err := os.ReadDir(watchDir)
				if err == nil {
					var nodeOptions []huh.Option[string]
					for _, entry := range entries {
						if entry.IsDir() {
							nodeOptions = append(nodeOptions, huh.NewOption(entry.Name(), entry.Name()))
						}
					}
					var selected []string
					form := huh.NewForm(huh.NewGroup(
						huh.NewMultiSelect[string]().Title("Select custom node directories to watch for reloads:").Options(nodeOptions...).Value(&selected),
					)).WithTheme(huh.ThemeCharm())
					_ = form.Run()
					includedDirs = selected
					// Save to comfy-installs.json
					cfg, _ := internal.LoadGlobalConfig()
					for i := range cfg.Installs {
						if cfg.Installs[i].Type == inst.Type {
							cfg.Installs[i].ReloadIncludeDirs = includedDirs
						}
					}
					_ = internal.SaveGlobalConfig(cfg)
				}
			}
			reloadComfyUI(watchDir, debounce, exts, includedDirs)
			return
		case "install":
			installComfyUI()
			return
		case "status":
			internal.RunWithEnvConfirmation("status", func(inst *internal.ComfyInstall) { statusComfyUIWithEnv(inst) })
			return
		case "update-nodes":
			updateCustomNodes()
			return
		case "create_node", "create-node":
			createNewNode()
			return
		case "list_nodes", "list-nodes":
			listCustomNodes()
			return
		case "delete_node", "delete-node":
			deleteCustomNode()
			return
		case "pack_node", "pack-node":
			packNode()
			return
		case "migrate-nodes":
			migrateCustomNodes()
			return
		case "migrate-workflows":
			migrateWorkflows()
			return
		case "migrate-images":
			migrateInputImages()
			return
		case "node-workflows":
			addOrRemoveNodeWorkflows()
			return
		case "remove-env":
			removeEnv()
			return
		case "sync-env":
			err := syncEnvWithExample()
			if err != nil {
				fmt.Println(internal.ErrorStyle.Render(fmt.Sprintf("Failed to sync .env: %v", err)))
				os.Exit(1)
			}
			os.Exit(0)
		case "help", "--help", "-h":
			printUsage()
			os.Exit(0)
		}
	}

	// Initialize menu choices
	menuChoices := internal.MenuChoices{
		StartForeground:  func(inst *internal.ComfyInstall) { startComfyUIWithEnv(inst, false) },
		StartBackground:  func(inst *internal.ComfyInstall) { startComfyUIWithEnv(inst, true) },
		Stop:             func(inst *internal.ComfyInstall) { stopComfyUIWithEnv(inst) },
		Restart:          func(inst *internal.ComfyInstall) { restartComfyUIWithEnv(inst) },
		Update:           func(inst *internal.ComfyInstall) { updateComfyUIWithEnv(inst) },
		Install:          installComfyUI,
		CreateNode:       createNewNode,
		ListNodes:        listCustomNodes,
		DeleteNode:       deleteCustomNode,
		PackNode:         packNode,
		UpdateNodes:      updateCustomNodes,
		MigrateNodes:     migrateCustomNodes,
		MigrateWorkflows: migrateWorkflows,
		MigrateImages:    migrateInputImages,
		NodeWorkflows:    addOrRemoveNodeWorkflows,
		ManageEnvs:       manageBrandedEnvironments,
		RemoveEnv:        removeEnv,
		Reload:           func() { handleReloadFunction() },
		WatchNodes:       func() { handleWatchNodesFunction() },
		Status:           func(inst *internal.ComfyInstall) { statusComfyUIWithEnv(inst) },
		SyncEnv:          syncEnvWithExample,
		SetWorkingEnv:    func() { handleSetWorkingEnvFunction() },
	}

	// Start the main menu loop
	internal.ShowMainMenu(menuChoices, &appPaths)
}

// handleReloadFunction handles the reload functionality
func handleReloadFunction() {
	inst, err := internal.GetActiveComfyInstall()
	if err != nil {
		fmt.Println(internal.ErrorStyle.Render(err.Error()))
		return
	}
	watchDir, debounce, exts, includedDirs := internal.GetReloadSettings(inst)
	reloadComfyUI(watchDir, debounce, exts, includedDirs)
}

// handleWatchNodesFunction handles the watch nodes functionality
func handleWatchNodesFunction() {
	inst, err := internal.GetActiveComfyInstall()
	if err != nil {
		fmt.Println(internal.ErrorStyle.Render(err.Error()))
		os.Exit(1)
	}
	customNodesDir := internal.ExpandUserPath(filepath.Join(inst.Path, "custom_nodes"))
	
	selected, err := internal.SelectNodeDirectories(customNodesDir)
	if err != nil {
		if strings.Contains(err.Error(), "cancelled") {
			fmt.Println(internal.InfoStyle.Render("Operation cancelled by user."))
			os.Exit(0)
		}
		if strings.Contains(err.Error(), "no custom node directories") {
			fmt.Println(internal.WarningStyle.Render("No custom node directories found to watch."))
			os.Exit(0)
		}
		fmt.Println(internal.ErrorStyle.Render(err.Error()))
		os.Exit(1)
	}
	
	// Save to comfy-installs.json
	cfg, _ := internal.LoadGlobalConfig()
	for i := range cfg.Installs {
		if cfg.Installs[i].Type == inst.Type {
			cfg.Installs[i].ReloadIncludeDirs = selected
		}
	}
	if err := internal.SaveGlobalConfig(cfg); err != nil {
		fmt.Println(internal.ErrorStyle.Render(fmt.Sprintf("Failed to update comfy-installs.json: %v", err)))
		os.Exit(1)
	}
	fmt.Println(internal.SuccessStyle.Render(fmt.Sprintf("Updated comfy-installs.json: reload_include_dirs=%s", strings.Join(selected, ","))))
	os.Exit(0)
}

// handleSetWorkingEnvFunction handles setting the working environment
func handleSetWorkingEnvFunction() {
	cfg, _ := internal.LoadGlobalConfig()
	if len(cfg.Installs) == 0 {
		fmt.Println(internal.WarningStyle.Render("No environments configured. Use 'Manage Environments' to add one."))
		return
	}
	var envOptions []huh.Option[string]
	for _, inst := range cfg.Installs {
		label := string(inst.Type)
		if inst.IsDefault {
			label += " (default)"
		}
		if inst.Name != "" && inst.Name != string(inst.Type) {
			label += " - " + inst.Name
		}
		envOptions = append(envOptions, huh.NewOption(label, string(inst.Type)))
	}
	var selectedEnv string
	form := huh.NewForm(huh.NewGroup(huh.NewSelect[string]().Title("Select working environment:").Options(envOptions...).Value(&selectedEnv))).WithTheme(huh.ThemeCharm())
	_ = form.Run()
	if selectedEnv != "" {
		inst := cfg.FindInstallByType(internal.InstallType(selectedEnv))

		if inst != nil {
			internal.UpdateEnvFile(appPaths.EnvFile, map[string]string{
				"WORKING_COMFY_ENV": selectedEnv,
				"COMFYUI_PATH":      inst.Path,
			})
			fmt.Println(internal.SuccessStyle.Render("Working environment set to: " + selectedEnv + " (" + inst.Path + ")"))
		} else {
			fmt.Println(internal.ErrorStyle.Render("Selected environment not found in config."))
		}
	}
}


// waitForComfyUIReady waits for ComfyUI to be ready by checking the log file for a specific string.
func waitForComfyUIReady() error {
	startTime := time.Now()
	for {
		if time.Since(startTime) > maxWaitTime {
			return fmt.Errorf("timeout waiting for ComfyUI to be ready")
		}
		logFile, err := os.Open(internal.ExpandUserPath(appPaths.LogFile))
		if err != nil {
			return fmt.Errorf("failed to open log file: %w", err)
		}
		defer logFile.Close()
		scanner := bufio.NewScanner(logFile)
		for scanner.Scan() {
			if strings.Contains(scanner.Text(), comfyReadyString) {
				return nil
			}
		}
		time.Sleep(1 * time.Second)
	}
}

// waitForComfyUIStop waits for ComfyUI to stop by checking if the process is no longer running.
func waitForComfyUIStop(pid int) error {
	startTime := time.Now()
	for {
		if time.Since(startTime) > maxWaitTime {
			return fmt.Errorf("timeout waiting for ComfyUI to stop")
		}
		if !internal.IsProcessRunning(pid) {
			return nil
		}
		time.Sleep(1 * time.Second)
	}
}


// startComfyUIWithEnv starts ComfyUI for the given environment, foreground/background.
func startComfyUIWithEnv(inst *internal.ComfyInstall, background bool) {
	oldDir := appPaths.ComfyUIDir
	appPaths.ComfyUIDir = inst.Path
	defer func() { appPaths.ComfyUIDir = oldDir }()
	startComfyUI(background)
}

// updateComfyUIWithEnv updates ComfyUI for the given environment.
func updateComfyUIWithEnv(inst *internal.ComfyInstall) {
	oldDir := appPaths.ComfyUIDir
	appPaths.ComfyUIDir = inst.Path
	defer func() { appPaths.ComfyUIDir = oldDir }()
	updateComfyUI()
}

// stopComfyUIWithEnv stops ComfyUI for the given environment.
func stopComfyUIWithEnv(inst *internal.ComfyInstall) {
	oldDir := appPaths.ComfyUIDir
	appPaths.ComfyUIDir = inst.Path
	defer func() { appPaths.ComfyUIDir = oldDir }()
	pidFile := internal.ExpandUserPath(filepath.Join(appPaths.ComfyUIDir, "comfyui.pid"))
	if pid, isRunning := internal.GetRunningPIDForEnv(pidFile); isRunning {
		process, err := os.FindProcess(pid)
		if err == nil {
			if runtime.GOOS == "windows" {
				process.Kill()
			} else {
				process.Signal(syscall.SIGTERM)
			}
			fmt.Println(internal.SuccessStyle.Render(fmt.Sprintf("Stopped ComfyUI (PID: %d)", pid)))
			os.Remove(pidFile)
		} else {
			fmt.Println(internal.ErrorStyle.Render("Could not find process to stop."))
		}
	} else {
		fmt.Println(internal.InfoStyle.Render("ComfyUI is not running."))
	}
}

// restartComfyUIWithEnv restarts ComfyUI for the given environment.
func restartComfyUIWithEnv(inst *internal.ComfyInstall) {
	stopComfyUIWithEnv(inst)
	startComfyUIWithEnv(inst, true)
}

// statusComfyUIWithEnv shows status for the given environment, with color and formatting (no table).
func statusComfyUIWithEnv(inst *internal.ComfyInstall) {
	oldDir := appPaths.ComfyUIDir
	appPaths.ComfyUIDir = inst.Path
	defer func() { appPaths.ComfyUIDir = oldDir }()
	pidFile := internal.ExpandUserPath(filepath.Join(appPaths.ComfyUIDir, "comfyui.pid"))
	logFile := internal.ExpandUserPath(filepath.Join(appPaths.ComfyUIDir, "comfyui.log"))
	pid, isRunning := internal.GetRunningPIDForEnv(pidFile)

	// Gather log info
	logSize := "(not found)"
	var lastLines []string
	if info, err := os.Stat(logFile); err == nil {
		logSize = fmt.Sprintf("%d bytes", info.Size())
		file, err := os.Open(logFile)
		if err == nil {
			defer file.Close()
			var lines []string
			scanner := bufio.NewScanner(file)
			for scanner.Scan() {
				lines = append(lines, scanner.Text())
			}
			if len(lines) > 0 {
				start := max(0, len(lines)-5)
				lastLines = lines[start:]
			}
		}
	}

	// Color helpers
	bold := func(s string) string { return internal.TitleStyle.Render(s) }
	green := func(s string) string { return internal.SuccessStyle.Render(s) }
	red := func(s string) string { return internal.ErrorStyle.Render(s) }
	yellow := func(s string) string { return internal.WarningStyle.Render(s) }
	blue := func(s string) string { return internal.InfoStyle.Render(s) }

	fmt.Println(bold("ComfyUI Environment Status"))
	fmt.Println(blue("──────────────────────────────"))
	fmt.Printf("%s: %s\n", bold("Name"), inst.Name)
	fmt.Printf("%s: %s\n", bold("Path"), inst.Path)
	fmt.Printf("%s: %s\n", bold("PID File"), pidFile)
	fmt.Printf("%s: %s\n", bold("Log File"), logFile)
	fmt.Printf("%s: %s\n", bold("PID"), func() string {
		if pid != 0 {
			return yellow(fmt.Sprintf("%d", pid))
		}
		return red("(not found)")
	}())
	fmt.Printf("%s: %s\n", bold("Status"), func() string {
		if isRunning {
			return green("RUNNING")
		}
		return red("NOT RUNNING")
	}())
	fmt.Printf("%s: %s\n", bold("Log Size"), logSize)
	fmt.Println(blue("──────────────────────────────"))

	if len(lastLines) > 0 {
		fmt.Println(bold("Last Log Lines:"))
		for _, l := range lastLines {
			fmt.Printf("  %s\n", l)
		}
	}
}

// manageBrandedEnvironments allows the user to manage branded environments (Lounge, Den, Nook).
func manageBrandedEnvironments() {
	cfg, err := internal.LoadGlobalConfig()
	if err != nil {
		fmt.Println(internal.ErrorStyle.Render("Failed to load global config: " + err.Error()))
		return
	}
	labels := map[internal.InstallType]string{
		internal.LoungeInstall: "Lounge (Main/Stable)",
		internal.DenInstall:    "Den (Dev/Alternate)",
		internal.NookInstall:   "Nook (Experimental/Test)",
	}
	for {
		var selectedEnv string
		form := huh.NewForm(
			huh.NewGroup(
				huh.NewSelect[string]().
					Title("Select an environment to manage:").
					Options(
						huh.NewOption("Lounge (Main/Stable)", string(internal.LoungeInstall)),
						huh.NewOption("Den (Dev/Alternate)", string(internal.DenInstall)),
						huh.NewOption("Nook (Experimental/Test)", string(internal.NookInstall)),
					).
					Value(&selectedEnv),
			),
		).WithTheme(huh.ThemeCharm())
		if err := form.Run(); err != nil {
			if errors.Is(err, huh.ErrUserAborted) {
				fmt.Println(internal.InfoStyle.Render("Operation cancelled by user."))
				return
			}
			fmt.Println(internal.ErrorStyle.Render(fmt.Sprintf("Error running form: %v", err)))
			return
		}
		envType := internal.InstallType(selectedEnv)
		inst := cfg.FindInstallByType(envType)
		if inst == nil {
			fmt.Println(internal.WarningStyle.Render("No install found for this environment. You can add it by running 'Install/Reconfigure ComfyUI'."))
			return
		}
		var action string
		form2 := huh.NewForm(
			huh.NewGroup(
				huh.NewSelect[string]().
					Title(fmt.Sprintf("Manage %s:", labels[envType])).
					Options(
						huh.NewOption("Set as Default", "set_default"),
						huh.NewOption("Update", "update"),
						huh.NewOption("Remove (Disconnect)", "remove"),
						huh.NewOption("Back", "back"),
					).
					Value(&action),
			),
		).WithTheme(huh.ThemeCharm())
		if err := form2.Run(); err != nil {
			if errors.Is(err, huh.ErrUserAborted) {
				fmt.Println(internal.InfoStyle.Render("Operation cancelled by user."))
				return
			}
			fmt.Println(internal.ErrorStyle.Render(fmt.Sprintf("Error running form: %v", err)))
			return
		}
		switch action {
		case "set_default":
			for i := range cfg.Installs {
				cfg.Installs[i].IsDefault = cfg.Installs[i].Type == envType
			}
			if err := internal.SaveGlobalConfig(cfg); err != nil {
				fmt.Println(internal.ErrorStyle.Render(fmt.Sprintf("Failed to save global config: %v", err)))
				return
			}
			fmt.Println(internal.SuccessStyle.Render(fmt.Sprintf("%s set as default environment.", labels[envType])))
		case "update":
			updateComfyUIWithEnv(inst)
		case "remove":
			removeEnv()
			return
		case "back":
			return
		}
	}
}

// migrateInputImages allows the user to copy input images between environments. from  "input"
// it will have to copy common images, video, and audio extensions, plus recursive directories.
func migrateInputImages() {
	fmt.Println(internal.TitleStyle.Render("Migrate Input Images Between Environments"))
	cfg, err := internal.LoadGlobalConfig()
	if err != nil || len(cfg.Installs) < 2 {
		fmt.Println(internal.ErrorStyle.Render("At least two environments must be configured to migrate input images."))
		return
	}

	// --- DESIGN WARNING ---
	fmt.Println(internal.WarningStyle.Render("Only the first 50 files/folders and one level deep are shown for migration. This is a design choice to avoid overwhelming the interface. If you need to migrate a deeper or specific path, use the custom path option (relative to the input directory)."))

	// Supported extensions
	exts := []string{".jpg", ".jpeg", ".png", ".gif", ".mp4", ".webm", ".wav", ".mp3"}
	isMedia := func(name string) bool {
		lower := strings.ToLower(name)
		for _, ext := range exts {
			if strings.HasSuffix(lower, ext) {
				return true
			}
		}
		return false
	}

	type Entry struct {
		Display string // e.g. "folder/file.jpg" or "folder/"
		Path    string // relative to srcInputDir
		IsDir   bool
	}
	// 1. Prompt for source and target environments
	envOptions := []huh.Option[string]{}
	for _, inst := range cfg.Installs {
		label := string(inst.Type)
		if inst.Name != "" && inst.Name != string(inst.Type) {
			label += " - " + inst.Name
		}
		envOptions = append(envOptions, huh.NewOption(label, string(inst.Type)))
	}
	var srcEnvType, dstEnvType string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().Title("Select source environment:").Options(envOptions...).Value(&srcEnvType),
			huh.NewSelect[string]().Title("Select target environment:").Options(envOptions...).Value(&dstEnvType),
		),
	).WithTheme(huh.ThemeCharm())
	_ = form.Run()
	if srcEnvType == dstEnvType || srcEnvType == "" || dstEnvType == "" {
		fmt.Println(internal.InfoStyle.Render("Migration cancelled (invalid selection)."))
		return
	}
	src := cfg.FindInstallByType(internal.InstallType(srcEnvType))
	dst := cfg.FindInstallByType(internal.InstallType(dstEnvType))
	if src == nil || dst == nil {
		fmt.Println(internal.ErrorStyle.Render("Invalid environment selection."))
		return
	}

	srcInputDir := internal.ExpandUserPath(filepath.Join(src.Path, "input"))
	dstInputDir := internal.ExpandUserPath(filepath.Join(dst.Path, "input"))

	entries := make([]Entry, 0, 50)
	files, err := os.ReadDir(srcInputDir)
	if err != nil {
		fmt.Println(internal.ErrorStyle.Render("Failed to read source input directory: " + err.Error()))
		return
	}
	count := 0
	for _, f := range files {
		if count >= 50 {
			break
		}
		if f.IsDir() {
			entries = append(entries, Entry{Display: f.Name() + string(os.PathSeparator), Path: f.Name(), IsDir: true})
		} else if isMedia(f.Name()) {
			entries = append(entries, Entry{Display: f.Name(), Path: f.Name(), IsDir: false})
		}
		count++
	}
	customPathOption := "[Custom path...]"
	entries = append(entries, Entry{Display: customPathOption, Path: "__CUSTOM__", IsDir: false})

	allOption := "ALL (select all files and folders)"
	var selected []string
	form2 := huh.NewForm(huh.NewGroup(
		huh.NewMultiSelect[string]().
			Title("Select input files or folders to migrate:").
			OptionsFunc(func() []huh.Option[string] {
				opts := make([]huh.Option[string], 0, len(entries)+1)
				opts = append(opts, huh.NewOption(allOption, allOption))
				for _, e := range entries {
					opts = append(opts, huh.NewOption(e.Display, e.Path))
				}
				return opts
			}, nil).
			Value(&selected),
	)).WithTheme(huh.ThemeCharm())
	_ = form2.Run()
	if len(selected) == 0 {
		fmt.Println(internal.InfoStyle.Render("No files or folders selected. Migration cancelled."))
		return
	}

	pathsToCopy := map[string]struct{}{}
	foldersToCopy := map[string]struct{}{}
	results := []string{}
	// Expand selection
	if len(selected) == 1 && selected[0] == allOption {
		for _, e := range entries {
			if e.IsDir {
				foldersToCopy[e.Path] = struct{}{}
			} else {
				pathsToCopy[e.Path] = struct{}{}
			}
		}
	} else {
		for _, sel := range selected {
			for _, e := range entries {
				if sel == e.Path {
					if e.IsDir {
						foldersToCopy[e.Path] = struct{}{}
					} else {
						pathsToCopy[e.Path] = struct{}{}
					}
				}
			}
		}
	}

	for relPath := range pathsToCopy {
		srcPath := filepath.Join(srcInputDir, relPath)
		dstPath := filepath.Join(dstInputDir, relPath)
		if err := os.MkdirAll(filepath.Dir(dstPath), 0755); err != nil {
			results = append(results, fmt.Sprintf("%s: %s", relPath, internal.ErrorStyle.Render("Failed to create directory")))
			continue
		}
		input, err := os.ReadFile(srcPath)
		if err != nil {
			results = append(results, fmt.Sprintf("%s: %s", relPath, internal.ErrorStyle.Render("Failed to read")))
			continue
		}
		if err := os.WriteFile(dstPath, input, 0644); err != nil {
			results = append(results, fmt.Sprintf("%s: %s", relPath, internal.ErrorStyle.Render("Failed to write")))
			continue
		}
		results = append(results, fmt.Sprintf("%s: %s", relPath, internal.SuccessStyle.Render("Copied")))
	}
	for relPath := range foldersToCopy {
		srcPath := filepath.Join(srcInputDir, relPath)
		dstPath := filepath.Join(dstInputDir, relPath)
		if err := copyDir(srcPath, dstPath); err != nil {
			results = append(results, fmt.Sprintf("%s: %s", relPath, internal.ErrorStyle.Render("Failed to copy folder")))
			continue
		}
		results = append(results, fmt.Sprintf("%s: %s", relPath, internal.SuccessStyle.Render("Folder copied")))
	}

	// 6. Summary
	fmt.Println(internal.TitleStyle.Render("\nInput File Migration Summary:"))
	for _, r := range results {
		fmt.Println("  " + r)
	}
}

// copyDir recursively copies a directory tree from src to dst.
func copyDir(src string, dst string) error {
	src = internal.ExpandUserPath(src)
	dst = internal.ExpandUserPath(dst)
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dst, srcInfo.Mode()); err != nil {
		return err
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())
		if entry.IsDir() {
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			data, err := os.ReadFile(srcPath)
			if err != nil {
				return err
			}
			if err := os.WriteFile(dstPath, data, 0644); err != nil {
				return err
			}
		}
	}
	return nil
}

// migrateWorkflows allows the user to copy workflow JSON files between environments.
func migrateWorkflows() {
	fmt.Println(internal.TitleStyle.Render("Migrate Workflows Between Environments"))
	cfg, err := internal.LoadGlobalConfig()
	if err != nil || len(cfg.Installs) < 2 {
		fmt.Println(internal.ErrorStyle.Render("At least two environments must be configured to migrate workflows."))
		return
	}

	// 1. Prompt for source and target environments
	envOptions := []huh.Option[string]{}
	for _, inst := range cfg.Installs {
		label := string(inst.Type)
		if inst.Name != "" && inst.Name != string(inst.Type) {
			label += " - " + inst.Name
		}
		envOptions = append(envOptions, huh.NewOption(label, string(inst.Type)))
	}
	var srcEnvType, dstEnvType string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().Title("Select source environment:").Options(envOptions...).Value(&srcEnvType),
			huh.NewSelect[string]().Title("Select target environment:").Options(envOptions...).Value(&dstEnvType),
		),
	).WithTheme(huh.ThemeCharm())
	_ = form.Run()
	if srcEnvType == dstEnvType || srcEnvType == "" || dstEnvType == "" {
		fmt.Println(internal.InfoStyle.Render("Migration cancelled (invalid selection)."))
		return
	}
	src := cfg.FindInstallByType(internal.InstallType(srcEnvType))
	dst := cfg.FindInstallByType(internal.InstallType(dstEnvType))
	if src == nil || dst == nil {
		fmt.Println(internal.ErrorStyle.Render("Invalid environment selection."))
		return
	}

	// 2. Recursively list workflow files and folders in source env
	srcWorkflowsDir := internal.ExpandUserPath(filepath.Join(src.Path, "user", "default", "workflows"))
	dstWorkflowsDir := internal.ExpandUserPath(filepath.Join(dst.Path, "user", "default", "workflows"))

	type Entry struct {
		Display string // e.g. "folder/file.json" or "folder/"
		Path    string // relative to srcWorkflowsDir
		IsDir   bool
	}
	var entries []Entry

	err = filepath.Walk(srcWorkflowsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip errors
		}
		rel, _ := filepath.Rel(srcWorkflowsDir, path)
		if rel == "." {
			return nil
		}
		if info.IsDir() {
			entries = append(entries, Entry{Display: rel + string(os.PathSeparator), Path: rel, IsDir: true})
		} else if strings.HasSuffix(info.Name(), ".json") {
			entries = append(entries, Entry{Display: rel, Path: rel, IsDir: false})
		}
		return nil
	})
	if err != nil {
		fmt.Println(internal.ErrorStyle.Render("Failed to read source workflows directory: " + err.Error()))
		return
	}
	if len(entries) == 0 {
		fmt.Println(internal.InfoStyle.Render("No workflow files or folders found in source environment."))
		return
	}

	// 3. Prompt for selection (allow selecting files and folders, plus "ALL")
	allOption := "ALL (select all files and folders)"
	var selected []string
	form2 := huh.NewForm(huh.NewGroup(
		huh.NewMultiSelect[string]().
			Title("Select workflows or folders to migrate:").
			OptionsFunc(func() []huh.Option[string] {
				opts := make([]huh.Option[string], 0, len(entries)+1)
				opts = append(opts, huh.NewOption(allOption, allOption))
				for _, e := range entries {
					opts = append(opts, huh.NewOption(e.Display, e.Path))
				}
				return opts
			}, nil).
			Value(&selected),
	)).WithTheme(huh.ThemeCharm())
	_ = form2.Run()
	if len(selected) == 0 {
		fmt.Println(internal.InfoStyle.Render("No workflows or folders selected. Migration cancelled."))
		return
	}

	// 4. Resolve selection (expand folders, handle ALL)
	pathsToCopy := map[string]struct{}{}
	if len(selected) == 1 && selected[0] == allOption {
		// Select all files (not folders)
		for _, e := range entries {
			if !e.IsDir {
				pathsToCopy[e.Path] = struct{}{}
			}
		}
	} else {
		for _, sel := range selected {
			// If folder, add all files under it
			for _, e := range entries {
				if !e.IsDir {
					if sel == e.Path || (strings.HasSuffix(sel, string(os.PathSeparator)) && strings.HasPrefix(e.Path, sel)) {
						pathsToCopy[e.Path] = struct{}{}
					}
				}
			}
			// If file, add directly
			for _, e := range entries {
				if !e.IsDir && sel == e.Path {
					pathsToCopy[e.Path] = struct{}{}
				}
			}
		}
	}
	if len(pathsToCopy) == 0 {
		fmt.Println(internal.InfoStyle.Render("No workflow files selected for migration."))
		return
	}

	// 5. Copy selected workflows, preserving folder structure
	var results []string
	for relPath := range pathsToCopy {
		srcPath := filepath.Join(srcWorkflowsDir, relPath)
		dstPath := filepath.Join(dstWorkflowsDir, relPath)
		// Ensure destination directory exists
		if err := os.MkdirAll(filepath.Dir(dstPath), 0755); err != nil {
			results = append(results, fmt.Sprintf("%s: %s", relPath, internal.ErrorStyle.Render("Failed to create directory")))
			continue
		}
		input, err := os.ReadFile(srcPath)
		if err != nil {
			results = append(results, fmt.Sprintf("%s: %s", relPath, internal.ErrorStyle.Render("Failed to read")))
			continue
		}
		if err := os.WriteFile(dstPath, input, 0644); err != nil {
			results = append(results, fmt.Sprintf("%s: %s", relPath, internal.ErrorStyle.Render("Failed to write")))
			continue
		}
		results = append(results, fmt.Sprintf("%s: %s", relPath, internal.SuccessStyle.Render("Copied")))
	}

	// 6. Summary
	fmt.Println(internal.TitleStyle.Render("\nWorkflow Migration Summary:"))
	for _, r := range results {
		fmt.Println("  " + r)
	}
}

// migrateCustomNodes allows the user to copy custom node directories between environments and install requirements.
func migrateCustomNodes() {
	fmt.Println(internal.TitleStyle.Render("Migrate Custom Nodes Between Environments"))
	cfg, err := internal.LoadGlobalConfig()
	if err != nil || len(cfg.Installs) < 2 {
		fmt.Println(internal.ErrorStyle.Render("At least two environments must be configured to migrate custom nodes."))
		return
	}

	// 1. Prompt for source and target environments
	envOptions := []huh.Option[string]{}
	for _, inst := range cfg.Installs {
		label := string(inst.Type)
		if inst.Name != "" && inst.Name != string(inst.Type) {
			label += " - " + inst.Name
		}
		envOptions = append(envOptions, huh.NewOption(label, string(inst.Type)))
	}
	var srcEnvType, dstEnvType string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().Title("Select source environment:").Options(envOptions...).Value(&srcEnvType),
			huh.NewSelect[string]().Title("Select target environment:").Options(envOptions...).Value(&dstEnvType),
		),
	).WithTheme(huh.ThemeCharm())
	_ = form.Run()
	if srcEnvType == dstEnvType || srcEnvType == "" || dstEnvType == "" {
		fmt.Println(internal.InfoStyle.Render("Migration cancelled (invalid selection)."))
		return
	}
	src := cfg.FindInstallByType(internal.InstallType(srcEnvType))
	dst := cfg.FindInstallByType(internal.InstallType(dstEnvType))
	if src == nil || dst == nil {
		fmt.Println(internal.ErrorStyle.Render("Invalid environment selection."))
		return
	}

	// 2. List custom node directories in source env
	srcNodesDir := internal.ExpandUserPath(filepath.Join(src.Path, "custom_nodes"))
	dstNodesDir := internal.ExpandUserPath(filepath.Join(dst.Path, "custom_nodes"))
	files, err := os.ReadDir(srcNodesDir)
	if err != nil {
		fmt.Println(internal.ErrorStyle.Render("Failed to read source custom_nodes directory: " + err.Error()))
		return
	}
	var nodeDirs []string
	for _, f := range files {
		if f.IsDir() && f.Name() != "venv" && f.Name() != ".venv" {
			nodeDirs = append(nodeDirs, f.Name())
		}
	}
	if len(nodeDirs) == 0 {
		fmt.Println(internal.InfoStyle.Render("No custom node directories found in source environment."))
		return
	}
	var selected []string
	form2 := huh.NewForm(huh.NewGroup(
		huh.NewMultiSelect[string]().Title("Select custom nodes to migrate:").OptionsFunc(func() []huh.Option[string] {
			opts := make([]huh.Option[string], len(nodeDirs))
			for i, n := range nodeDirs {
				opts[i] = huh.NewOption(n, n)
			}
			return opts
		}, nil).Value(&selected),
	)).WithTheme(huh.ThemeCharm())
	_ = form2.Run()
	if len(selected) == 0 {
		fmt.Println(internal.InfoStyle.Render("No custom nodes selected. Migration cancelled."))
		return
	}

	// 3. Copy selected custom nodes and install requirements
	if err := os.MkdirAll(dstNodesDir, 0755); err != nil {
		fmt.Println(internal.ErrorStyle.Render("Failed to create target custom_nodes directory: " + err.Error()))
		return
	}
	venvPath := filepath.Join(dst.Path, "venv")
	if _, err := os.Stat(venvPath); os.IsNotExist(err) {
		venvPath = filepath.Join(dst.Path, ".venv")
	}
	results := []string{}
	err = internal.CopyAndInstallCustomNodes(srcNodesDir, dstNodesDir, venvPath, selected)
	if err != nil {
		results = append(results, internal.ErrorStyle.Render(err.Error()))
	} else {
		for _, n := range selected {
			results = append(results, fmt.Sprintf("%s: %s", n, internal.SuccessStyle.Render("Migrated")))
		}
	}

	// 4. Summary
	fmt.Println(internal.TitleStyle.Render("\nCustom Node Migration Summary:"))
	for _, r := range results {
		fmt.Println("  " + r)
	}
}

// syncEnvWithExample copies missing keys from .env.example to .env, preserving existing values.
func syncEnvWithExample() error {
	envPath := appPaths.EnvFile
	examplePath := envPath + ".example"
	existing, err := internal.ReadEnvFile(envPath)
	if err != nil {
		return fmt.Errorf("failed to read .env: %w", err)
	}
	example, err := internal.ReadEnvFile(examplePath)
	if err != nil {
		return fmt.Errorf("failed to read .env.example: %w", err)
	}
	changed := false
	for k, v := range example {
		if _, ok := existing[k]; !ok {
			existing[k] = v
			changed = true
		}
	}
	if changed {
		if err := internal.WriteEnvFile(envPath, existing); err != nil {
			return fmt.Errorf("failed to write .env: %w", err)
		}
	}
	return nil
}
