package internal

import (
	"fmt"
	"os"

	"github.com/charmbracelet/huh"
)

// GetActiveComfyInstall returns the active ComfyUI installation based on environment variables or default
func GetActiveComfyInstall() (*ComfyInstall, error) {
	cfg, err := LoadGlobalConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load global config: %w", err)
	}
	workingEnv := os.Getenv(WorkingComfyEnvKey)
	if workingEnv != "" {
		inst := cfg.FindInstallByType(InstallType(workingEnv))
		if inst != nil {
			return inst, nil
		}
	}

	inst := cfg.FindDefaultInstall()
	if inst == nil {
		return nil, fmt.Errorf("no default environment set in comfy-installs.json")
	}
	return inst, nil
}

// RunWithEnvConfirmation prompts for environment confirmation and runs the given action with the selected environment
func RunWithEnvConfirmation(action string, fn func(inst *ComfyInstall)) {
	cfg, err := LoadGlobalConfig()
	if err != nil {
		fmt.Println(ErrorStyle.Render("Failed to load global config: " + err.Error()))
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
		return
	}
	fn(inst)
}