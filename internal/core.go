package internal

import (
	"errors"
	"fmt"
	"os"

	"github.com/charmbracelet/huh"
)

// GetActiveComfyInstall returns the active ComfyUI installation based on environment variables or default
// If no configuration exists, it offers to create one interactively
func GetActiveComfyInstall() (*ComfyInstall, error) {
	cfg, err := LoadGlobalConfig()
	if err != nil {
		// If config doesn't exist, offer interactive options
		Log.Warning("ComfyUI environment configuration not found.")

		var choice string
		form := huh.NewForm(huh.NewGroup(
			huh.NewSelect[string]().
				Title("No ComfyUI environment configured. What would you like to do?").
				Options(
					huh.NewOption("Go to main menu to install/configure", "menu"),
					huh.NewOption("Exit", "exit"),
				).
				Value(&choice),
		)).WithTheme(huh.ThemeCharm())

		if err := form.Run(); err != nil {
			return nil, fmt.Errorf("operation cancelled")
		}

		if choice == "exit" || choice == "" {
			return nil, fmt.Errorf("operation cancelled - no environment configured")
		}

		// Return special error to signal caller should return to menu
		return nil, fmt.Errorf("no ComfyUI environment configured - returning to menu")
	}

	workingEnv := os.Getenv(WorkingComfyEnvKey)
	if workingEnv != "" {
		inst := cfg.FindInstallByType(InstallType(workingEnv))
		if inst != nil {
			return inst, nil
		}
		Log.Warning("Working environment '%s' not found in configuration.", workingEnv)
	}

	inst := cfg.FindDefaultInstall()
	if inst == nil {
		// Offer to select from available environments or set a default
		if len(cfg.Installs) > 0 {
			Log.Warning("No default environment set, but %d environment(s) found.", len(cfg.Installs))
			var envOptions []huh.Option[string]
			for _, i := range cfg.Installs {
				label := string(i.Type)
				if i.Name != "" && i.Name != string(i.Type) {
					label += " - " + i.Name
				}
				envOptions = append(envOptions, huh.NewOption(label, string(i.Type)))
			}
			envOptions = append(envOptions, huh.NewOption("Cancel", "cancel"))

			var selected string
			form := huh.NewForm(huh.NewGroup(
				huh.NewSelect[string]().
					Title("Select an environment to use:").
					Options(envOptions...).
					Value(&selected),
			)).WithTheme(huh.ThemeCharm())

			if err := form.Run(); err != nil {
				return nil, fmt.Errorf("operation cancelled")
			}

			if selected == "cancel" || selected == "" {
				return nil, fmt.Errorf("no environment selected")
			}

			inst = cfg.FindInstallByType(InstallType(selected))
			if inst != nil {
				return inst, nil
			}
		}

		Log.Warning("No default environment found in configuration.")
		Log.Info("Use 'Environment Management' > 'Manage Branded Environments' to set a default.")
		return nil, fmt.Errorf("no default environment configured")
	}
	return inst, nil
}

// RunWithEnvConfirmation prompts for environment confirmation and runs the given action with the selected environment
func RunWithEnvConfirmation(action string, fn func(inst *ComfyInstall)) {
	cfg, err := LoadGlobalConfig()
	if err != nil {
		Log.Error("Failed to load global config: %v", err)
		Log.Info("Use 'Install/Reconfigure ComfyUI' to create the initial configuration.")
		PromptReturnToMenu()
		return
	}
	
	if len(cfg.Installs) == 0 {
		Log.Warning("No ComfyUI environments configured.")
		Log.Info("Use 'Install/Reconfigure ComfyUI' to add your first environment.")
		PromptReturnToMenu()
		return
	}
	workingEnv := os.Getenv(WorkingComfyEnvKey)
	if workingEnv != "" {
		inst := cfg.FindInstallByType(InstallType(workingEnv))
		if inst != nil {
			fn(inst)
			return
		}
	}
	inst := cfg.FindDefaultInstall()
	if len(cfg.Installs) > 1 && workingEnv == "" {
		var envOptions []huh.Option[string]
		for _, i := range cfg.Installs {
			label := string(i.Type)
			if i.Name != "" && i.Name != string(i.Type) {
				label += " - " + i.Name
			}
			if i.IsDefault {
				label += " (default)"
			}
			envOptions = append(envOptions, huh.NewOption(label, string(i.Type)))
		}
		var selected string
		form := huh.NewForm(huh.NewGroup(huh.NewSelect[string]().Title("Select environment for '" + action + "':").Options(envOptions...).Value(&selected))).WithTheme(huh.ThemeCharm())
		if err := form.Run(); err != nil {
			if errors.Is(err, huh.ErrUserAborted) {
				Log.Info("Operation cancelled.")
				PromptReturnToMenu()
				return
			}
			Log.Error("Form error: %v", err)
			PromptReturnToMenu()
			return
		}
		if selected != "" {
			inst = cfg.FindInstallByType(InstallType(selected))
		}
	}
	if inst == nil {
		Log.Error("No environment selected or found.")
		PromptReturnToMenu()
		return
	}
	fn(inst)
	PromptReturnToMenu()
}

// PromptReturnToMenu shows a prompt to return to the main menu
func PromptReturnToMenu() {
	var dummy string
	form := huh.NewForm(huh.NewGroup(
		huh.NewSelect[string]().
			Title("Operation completed").
			Options(
				huh.NewOption("Return to Main Menu", "return"),
			).
			Value(&dummy),
	)).WithTheme(huh.ThemeCharm())
	if err := form.Run(); err != nil {
		// Ignore errors here since this is just a prompt to continue
		if !errors.Is(err, huh.ErrUserAborted) {
			Log.Error("Form error: %v", err)
		}
	}
}