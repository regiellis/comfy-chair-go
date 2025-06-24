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

package internal

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/charmbracelet/huh"
)

// MenuAction represents a callback function for menu actions
type MenuAction func(*ComfyInstall)

// MenuChoice represents a menu choice with its identifier and action
type MenuChoice struct {
	ID     string
	Action MenuAction
}

// MenuChoices defines the available menu action callbacks
type MenuChoices struct {
	StartForeground    MenuAction
	StartBackground    MenuAction
	Stop               MenuAction
	Restart            MenuAction
	Update             MenuAction
	Install            func()
	CreateNode         func()
	ListNodes          func()
	DeleteNode         func()
	PackNode           func()
	UpdateNodes        func()
	MigrateNodes       func()
	MigrateWorkflows   func()
	MigrateImages      func()
	NodeWorkflows      func()
	ManageEnvs         func()
	RemoveEnv          func()
	Reload             func()
	WatchNodes         func()
	Status             MenuAction
	SyncEnv            func() error
	SetWorkingEnv      func()
	Performance        func()
}

// ShowMainMenu displays the main menu loop and handles navigation
func ShowMainMenu(choices MenuChoices, appPaths *Paths) {
	for {
		var choice string
		form := huh.NewForm(
			huh.NewGroup(
				huh.NewSelect[string]().
					Title(TitleStyle.Render("Comfy Chair Manager")).
					Description("Select an action:").
					Options(
						huh.NewOption("Start/Stop/Restart", "main_actions"),
						huh.NewOption("Node Management", "node_mgmt"),
						huh.NewOption("Environment Management", "env_mgmt"),
						huh.NewOption("Other Tools", "other_tools"),
						huh.NewOption("Exit", "exit"),
					).
					Value(&choice),
			),
		).WithTheme(huh.ThemeCharm())
		_ = form.Run()

		// Nested menu logic in a loop
		for {
			if choice == "main_actions" {
				choice = showMainActionsMenu()
			} else if choice == "node_mgmt" {
				choice = showNodeManagementMenu()
			} else if choice == "env_mgmt" {
				choice = showEnvironmentManagementMenu()
			} else if choice == "other_tools" {
				choice = showOtherToolsMenu()
			} else {
				break // Not a menu, break to process action
			}
		}

		// If choice is empty, show main menu again
		if choice == "" {
			continue
		}

		// Process the final actionable choice
		if !appPaths.IsConfigured && choice != "install" && choice != "exit" && choice != "manage_envs" {
			fmt.Println(ErrorStyle.Render(fmt.Sprintf("ComfyUI path is not configured. Please run 'Install/Reconfigure ComfyUI' or set COMFYUI_PATH in %s (located at %s).", EnvFileName, appPaths.EnvFile)))
			os.Exit(1)
		}

		// Execute the chosen action
		executeMenuAction(choice, choices, appPaths)
	}
}

// showMainActionsMenu displays the main actions submenu
func showMainActionsMenu() string {
	var subChoice string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Start/Stop/Restart").
				Options(
					huh.NewOption("Start ComfyUI (Foreground)", "start_fg"),
					huh.NewOption("Start ComfyUI (Background)", "start_bg"),
					huh.NewOption("Stop ComfyUI", "stop"),
					huh.NewOption("Restart ComfyUI (Background)", "restart"),
					huh.NewOption("Update ComfyUI", "update"),
					huh.NewOption("Install/Reconfigure ComfyUI", "install"),
					huh.NewOption("Status (ComfyUI)", "status"),
					huh.NewOption("More Tools", "back"),
				).
				Value(&subChoice),
		),
	).WithTheme(huh.ThemeCharm())
	_ = form.Run()
	if subChoice == "back" || subChoice == "" {
		return ""
	}
	return subChoice
}

// showNodeManagementMenu displays the node management submenu
func showNodeManagementMenu() string {
	var subChoice string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Node Management").
				Options(
					huh.NewOption("Create New Node", "create_node"),
					huh.NewOption("List Custom Nodes", "list_nodes"),
					huh.NewOption("Delete Custom Node", "delete_node"),
					huh.NewOption("Pack Custom Node", "pack_node"),
					huh.NewOption("Update Custom Nodes", "update-nodes"),
					huh.NewOption("Reload ComfyUI on Node Changes", "reload"),
					huh.NewOption("Select Watched Nodes for Reload", "watch_nodes"),
					huh.NewOption("Migrate Custom Nodes", "migrate-nodes"),
					huh.NewOption("Migrate Input Images/Media", "migrate-images"),
					huh.NewOption("Migrate Node Workflows", "migrate-workflows"),
					huh.NewOption("Migrate Workflows", "migrate-workflows"),
					huh.NewOption("Add/Remove Custom Node Workflows", "node_workflows"),
					huh.NewOption("Main Menu", "back"),
				).
				Value(&subChoice),
		),
	).WithTheme(huh.ThemeCharm())
	_ = form.Run()
	if subChoice == "back" || subChoice == "" {
		return ""
	}
	return subChoice
}

// showEnvironmentManagementMenu displays the environment management submenu
func showEnvironmentManagementMenu() string {
	var subChoice string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Environment Management").
				Options(
					huh.NewOption("Manage Environments (Lounge/Den/Nook)", "manage_envs"),
					huh.NewOption("Set Working Environment", "set_working_env"),
					huh.NewOption("Remove Environment (Disconnects, doesn't delete files)", "remove_env"),
					huh.NewOption("Install/Reconfigure ComfyUI", "install"),
					huh.NewOption("Main Menu", "back"),
				).
				Value(&subChoice),
		),
	).WithTheme(huh.ThemeCharm())
	_ = form.Run()
	if subChoice == "back" || subChoice == "" {
		return ""
	}
	return subChoice
}

// showOtherToolsMenu displays the other tools submenu
func showOtherToolsMenu() string {
	for {
		var toolChoice string
		form := huh.NewForm(huh.NewGroup(
			huh.NewSelect[string]().
				Title("Other Tools").
				Options(
					huh.NewOption("Install/Upgrade pip in Environment", "upgrade_pip"),
					huh.NewOption("Performance Monitoring", "performance"),
					huh.NewOption("Back", "back"),
				).
				Value(&toolChoice),
		)).WithTheme(huh.ThemeCharm())
		_ = form.Run()
		if toolChoice == "back" || toolChoice == "" {
			return ""
		}
		if toolChoice == "upgrade_pip" {
			handleUpgradePip()
		} else if toolChoice == "performance" {
			ShowPerformanceMenu()
		}
	}
}

// handleUpgradePip handles the pip upgrade functionality
func handleUpgradePip() {
	inst, err := GetActiveComfyInstall()
	if err != nil {
		fmt.Println(ErrorStyle.Render(err.Error()))
		return
	}
	venvPython, err := FindVenvPython(ExpandUserPath(inst.Path))
	if err != nil {
		fmt.Println(ErrorStyle.Render("Python executable not found in venv for this environment."))
		return
	}
	fmt.Println(InfoStyle.Render("Upgrading pip in environment..."))
	cmd := exec.Command(venvPython, "-m", "pip", "install", "-U", "pip")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Dir = inst.Path
	err = cmd.Run()
	if err != nil {
		fmt.Println(ErrorStyle.Render(fmt.Sprintf("Failed to upgrade pip: %v", err)))
	} else {
		fmt.Println(SuccessStyle.Render("pip upgraded successfully in environment."))
	}
}

// executeMenuAction executes the selected menu action
func executeMenuAction(choice string, choices MenuChoices, appPaths *Paths) {
	switch choice {
	case "start_fg":
		RunWithEnvConfirmation("start", choices.StartForeground)
	case "start_bg":
		RunWithEnvConfirmation("start", choices.StartBackground)
		os.Exit(0)
	case "stop":
		RunWithEnvConfirmation("stop", choices.Stop)
		os.Exit(0)
	case "restart":
		RunWithEnvConfirmation("restart", choices.Restart)
		os.Exit(0)
	case "update":
		RunWithEnvConfirmation("update", choices.Update)
		os.Exit(0)
	case "install":
		choices.Install()
		os.Exit(0)
	case "create_node":
		choices.CreateNode()
	case "list_nodes":
		choices.ListNodes()
	case "delete_node":
		choices.DeleteNode()
	case "pack_node":
		choices.PackNode()
	case "reload":
		choices.Reload()
	case "status":
		RunWithEnvConfirmation("status", choices.Status)
	case "watch_nodes":
		choices.WatchNodes()
	case "sync-env":
		err := choices.SyncEnv()
		if err != nil {
			fmt.Println(ErrorStyle.Render(fmt.Sprintf("Failed to sync .env: %v", err)))
			os.Exit(1)
		}
		os.Exit(0)
	case "update-nodes":
		choices.UpdateNodes()
	case "manage_envs":
		choices.ManageEnvs()
	case "set_working_env":
		handleSetWorkingEnv(appPaths)
	case "migrate-nodes":
		choices.MigrateNodes()
	case "migrate-workflows":
		choices.MigrateWorkflows()
	case "migrate-images":
		choices.MigrateImages()
	case "node_workflows":
		choices.NodeWorkflows()
	case "remove_env":
		choices.RemoveEnv()
		os.Exit(0)
	case "exit":
		fmt.Println(InfoStyle.Render("Exiting."))
		os.Exit(0)
	default:
		fmt.Println(WarningStyle.Render("Invalid choice."))
	}
}

// GetReloadSettings returns the reload settings for an install
func GetReloadSettings(inst *ComfyInstall) (watchDir string, debounce int, exts []string, includedDirs []string) {
	watchDir = ExpandUserPath(filepath.Join(inst.Path, "custom_nodes"))
	exts = []string{".py", ".js", ".css"}
	debounce = 5
	
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
	includedDirs = inst.ReloadIncludeDirs
	
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
			cfg, _ := LoadGlobalConfig()
			for i := range cfg.Installs {
				if cfg.Installs[i].Type == inst.Type {
					cfg.Installs[i].ReloadIncludeDirs = includedDirs
				}
			}
			_ = SaveGlobalConfig(cfg)
		}
	}
	
	return watchDir, debounce, exts, includedDirs
}

// handleSetWorkingEnv handles setting the working environment
func handleSetWorkingEnv(appPaths *Paths) {
	cfg, _ := LoadGlobalConfig()
	if len(cfg.Installs) == 0 {
		fmt.Println(WarningStyle.Render("No environments configured. Use 'Manage Environments' to add one."))
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
		inst := cfg.FindInstallByType(InstallType(selectedEnv))

		if inst != nil {
			UpdateEnvFile(appPaths.EnvFile, map[string]string{
				"WORKING_COMFY_ENV": selectedEnv,
				"COMFYUI_PATH":      inst.Path,
			})
			fmt.Println(SuccessStyle.Render("Working environment set to: " + selectedEnv + " (" + inst.Path + ")"))
		} else {
			fmt.Println(ErrorStyle.Render("Selected environment not found in config."))
		}
	}
}

// SelectNodeDirectories shows a multi-select menu for selecting node directories
func SelectNodeDirectories(customNodesDir string) ([]string, error) {
	entries, err := os.ReadDir(customNodesDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read custom_nodes directory: %w", err)
	}
	
	var nodeDirs []string
	for _, entry := range entries {
		if entry.IsDir() {
			nodeDirs = append(nodeDirs, entry.Name())
		}
	}
	
	if len(nodeDirs) == 0 {
		return nil, fmt.Errorf("no custom node directories found to watch")
	}
	
	var selected []string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("Select custom nodes to actively watch for reload (all others excluded):").
				OptionsFunc(func() []huh.Option[string] {
					opts := make([]huh.Option[string], 0, len(nodeDirs))
					for _, d := range nodeDirs {
						opts = append(opts, huh.NewOption(d, d))
					}
					return opts
				}, nil).
				Value(&selected),
		),
	).WithTheme(huh.ThemeCharm())
	
	if err := form.Run(); err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			return nil, fmt.Errorf("operation cancelled by user")
		}
		return nil, fmt.Errorf("error running form: %w", err)
	}
	
	return selected, nil
}