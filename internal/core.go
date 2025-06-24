package internal

import (
	"fmt"
	"os"

	"github.com/charmbracelet/huh"
)

// GetActiveComfyInstall returns the active ComfyUI installation based on environment variables or default
// If no configuration exists, it offers to create one
func GetActiveComfyInstall() (*ComfyInstall, error) {
	cfg, err := LoadGlobalConfig()
	if err != nil {
		// If config doesn't exist, offer to create it
		fmt.Println(WarningStyle.Render("ComfyUI environment configuration not found."))
		fmt.Println(InfoStyle.Render("You can create a configuration by running the 'Install/Reconfigure ComfyUI' option from the main menu."))
		return nil, fmt.Errorf("no ComfyUI environment configured - use Install option to set up")
	}
	
	workingEnv := os.Getenv(WorkingComfyEnvKey)
	if workingEnv != "" {
		inst := cfg.FindInstallByType(InstallType(workingEnv))
		if inst != nil {
			return inst, nil
		}
		fmt.Println(WarningStyle.Render(fmt.Sprintf("Working environment '%s' not found in configuration.", workingEnv)))
	}

	inst := cfg.FindDefaultInstall()
	if inst == nil {
		fmt.Println(WarningStyle.Render("No default environment found in configuration."))
		fmt.Println(InfoStyle.Render("Use 'Environment Management' > 'Manage Branded Environments' to set a default."))
		return nil, fmt.Errorf("no default environment configured")
	}
	return inst, nil
}

// RunWithEnvConfirmation prompts for environment confirmation and runs the given action with the selected environment
func RunWithEnvConfirmation(action string, fn func(inst *ComfyInstall)) {
	cfg, err := LoadGlobalConfig()
	if err != nil {
		fmt.Println(ErrorStyle.Render("Failed to load global config: " + err.Error()))
		fmt.Println(InfoStyle.Render("Use 'Install/Reconfigure ComfyUI' to create the initial configuration."))
		PromptReturnToMenu()
		return
	}
	
	if len(cfg.Installs) == 0 {
		fmt.Println(WarningStyle.Render("No ComfyUI environments configured."))
		fmt.Println(InfoStyle.Render("Use 'Install/Reconfigure ComfyUI' to add your first environment."))
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
		_ = form.Run()
		if selected != "" {
			inst = cfg.FindInstallByType(InstallType(selected))
		}
	}
	if inst == nil {
		fmt.Println(ErrorStyle.Render("No environment selected or found."))
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
	_ = form.Run()
}