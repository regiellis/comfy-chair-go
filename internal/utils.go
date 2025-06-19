package internal

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/joho/godotenv"
)

// Styles (exported)
var (
	InfoStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("39"))  // Blue
	ErrorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("196")) // Red
	SuccessStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("40"))  // Green
	WarningStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("214")) // Orange
	TitleStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("141"))
	SpinnerStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("99"))
)

// Spinner frames for CLI animations
var SpinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// Paths struct (exported)
type Paths struct {
	CliDir          string
	EnvFile         string
	ComfyUIDir      string
	VenvPath        string
	VenvPython      string
	PidFile         string
	LogFile         string
	RequirementsTxt string
	IsConfigured    bool
}

// InstallType represents the type of ComfyUI install (branded names).
type InstallType string

const (
	LoungeInstall InstallType = "lounge" // Main/primary environment (was Throne)
	DenInstall    InstallType = "den"    // Secondary/dev environment (was Sofa)
	NookInstall   InstallType = "nook"   // Experimental/testing environment (was Stool)
)

// ComfyInstall represents a single ComfyUI install tracked in the global config.
//
// Lounge: Your main/production ComfyUI install (stable, default for most actions)
// Den:    A secondary/dev install (for development, feature branches, or alternate configs)
// Nook:   An experimental/testing install (for testing new nodes, plugins, or risky changes)
type ComfyInstall struct {
	Name              string      `json:"name"`
	Type              InstallType `json:"type"` // lounge, den, nook
	Path              string      `json:"path"`
	IsDefault         bool        `json:"is_default"`
	CustomNodes       []string    `json:"custom_nodes"`
	ReloadIncludeDirs []string    `json:"reload_include_dirs,omitempty"`
}

// GlobalConfig holds all ComfyUI installs.
type GlobalConfig struct {
	Installs []ComfyInstall `json:"installs"`
}

// ConfigFileName is the name of the global config file.
const ConfigFileName = "comfy-installs.json"

// GetConfigFilePath returns the path to the global config file (next to the binary).
func GetConfigFilePath() (string, error) {
	exePath, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(exePath), ConfigFileName), nil
}

// LoadGlobalConfig loads the global config from disk, or returns an empty config if not found.
func LoadGlobalConfig() (*GlobalConfig, error) {
	path, err := GetConfigFilePath()
	if err != nil {
		return nil, err
	}
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &GlobalConfig{}, nil
		}
		return nil, err
	}
	defer f.Close()
	var cfg GlobalConfig
	dec := json.NewDecoder(f)
	if err := dec.Decode(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// SaveGlobalConfig writes the global config to disk.
func SaveGlobalConfig(cfg *GlobalConfig) error {
	path, err := GetConfigFilePath()
	if err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(cfg)
}

// FindInstallByType returns the install of a given type, or nil if not found.
func (cfg *GlobalConfig) FindInstallByType(t InstallType) *ComfyInstall {
	for i, inst := range cfg.Installs {
		if inst.Type == t {
			return &cfg.Installs[i]
		}
	}
	return nil
}

// FindDefaultInstall returns the default install, or nil if not set.
func (cfg *GlobalConfig) FindDefaultInstall() *ComfyInstall {
	for i, inst := range cfg.Installs {
		if inst.IsDefault {
			return &cfg.Installs[i]
		}
	}
	return nil
}

// AddOrUpdateInstall adds a new install or updates an existing one by type.
func (cfg *GlobalConfig) AddOrUpdateInstall(inst ComfyInstall) {
	for i, existing := range cfg.Installs {
		if existing.Type == inst.Type {
			cfg.Installs[i] = inst
			return
		}
	}
	cfg.Installs = append(cfg.Installs, inst)
}

// RemoveInstallByType removes an install by type.
func (cfg *GlobalConfig) RemoveInstallByType(t InstallType) {
	for i, inst := range cfg.Installs {
		if inst.Type == t {
			cfg.Installs = append(cfg.Installs[:i], cfg.Installs[i+1:]...)
			return
		}
	}
}

// ExpandUserPath replaces {HOME} and {USERPROFILE} with the user's home directory for cross-platform config paths.
func ExpandUserPath(path string) string {
	home, _ := os.UserHomeDir()
	path = strings.ReplaceAll(path, "{HOME}", home)
	if runtime.GOOS == "windows" {
		userProfile := os.Getenv("USERPROFILE")
		if userProfile != "" {
			path = strings.ReplaceAll(path, "{USERPROFILE}", userProfile)
		}
	}
	return filepath.Clean(path)
}

// ReadEnvFile reads a .env file and returns its key-value pairs.
func ReadEnvFile(path string) (map[string]string, error) {
	path = ExpandUserPath(path)
	if _, err := os.Stat(path); err != nil {
		return map[string]string{}, nil // treat missing as empty
	}
	return godotenv.Read(path)
}

// WriteEnvFile writes the given key-value pairs to a .env file.
func WriteEnvFile(path string, env map[string]string) error {
	path = ExpandUserPath(path)
	return godotenv.Write(env, path)
}

// UpdateEnvFile updates (or adds) the given keys in a .env file.
func UpdateEnvFile(path string, updates map[string]string) error {
	path = ExpandUserPath(path)
	env, _ := ReadEnvFile(path)
	for k, v := range updates {
		env[k] = v
	}
	return WriteEnvFile(path, env)
}

// PromptEditEnvFile interactively prompts the user to edit .env variables.
func PromptEditEnvFile(path string) error {
	env, _ := ReadEnvFile(path)
	if len(env) == 0 {
		fmt.Println(InfoStyle.Render("No variables found in .env. Starting with an empty file."))
	}

	// Prepare slices for keys and values
	keys := make([]string, 0, len(env))
	for k := range env {
		keys = append(keys, k)
	}

	// Edit existing variables
	for _, k := range keys {
		newVal := env[k]
		deleteVar := false
		form := huh.NewForm(
			huh.NewGroup(
				huh.NewInput().Title(k).Value(&newVal).Placeholder(env[k]),
				huh.NewConfirm().Title("Delete this variable?").Value(&deleteVar),
			),
		).WithTheme(huh.ThemeCharm())
		if err := form.Run(); err != nil {
			if errors.Is(err, huh.ErrUserAborted) {
				fmt.Println(InfoStyle.Render("Edit cancelled by user."))
				return nil
			}
			return err
		}
		if deleteVar {
			delete(env, k)
		} else {
			env[k] = newVal
		}
	}

	// Option to add a new variable
	addMore := false
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().Title("Add a new variable?").Value(&addMore),
		),
	).WithTheme(huh.ThemeCharm())
	_ = form.Run()
	for addMore {
		var newKey, newVal string
		form := huh.NewForm(
			huh.NewGroup(
				huh.NewInput().Title("Variable Name").Value(&newKey),
				huh.NewInput().Title("Value").Value(&newVal),
			),
		).WithTheme(huh.ThemeCharm())
		if err := form.Run(); err != nil {
			if errors.Is(err, huh.ErrUserAborted) {
				break
			}
			return err
		}
		if newKey != "" {
			env[newKey] = newVal
		}
		form = huh.NewForm(
			huh.NewGroup(
				huh.NewConfirm().Title("Add another variable?").Value(&addMore),
			),
		).WithTheme(huh.ThemeCharm())
		_ = form.Run()
	}

	if err := WriteEnvFile(path, env); err != nil {
		return err
	}
	fmt.Println(SuccessStyle.Render(".env updated successfully."))
	return nil
}

// FindVenvPython returns the path to the Python executable in venv or .venv, or an error if not found.
func FindVenvPython(comfyDir string) (string, error) {
	comfyDir = ExpandUserPath(comfyDir)
	venvDirs := []string{"venv", ".venv"}
	for _, venv := range venvDirs {
		venvPath := filepath.Join(comfyDir, venv)
		var pythonPath string
		if isWindows := (os.PathSeparator == '\\'); isWindows {
			pythonPath = filepath.Join(venvPath, "Scripts", "python.exe")
		} else {
			pythonPath = filepath.Join(venvPath, "bin", "python")
		}
		if stat, err := os.Stat(pythonPath); err == nil && !stat.IsDir() {
			return pythonPath, nil
		}
	}
	return "", fmt.Errorf("python executable not found in venv or .venv under %s", comfyDir)
}

// IsPortAvailable checks if a TCP port is available for listening.
func IsPortAvailable(port int) bool {
	addr := fmt.Sprintf(":%d", port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return false
	}
	_ = ln.Close()
	return true
}

// GetAvailablePort finds the next available port starting from startPort.
func GetAvailablePort(startPort int) int {
	for port := startPort; port < startPort+1000; port++ {
		if IsPortAvailable(port) {
			return port
		}
	}
	return -1 // No available port found in range
}

// PromptForPortConflict checks if the desired port is available, and if not, prompts the user to use another available port.
func PromptForPortConflict(desiredPort int) (int, error) {
	if IsPortAvailable(desiredPort) {
		return desiredPort, nil
	}
	altPort := GetAvailablePort(desiredPort + 1)
	if altPort == -1 {
		return -1, fmt.Errorf("no available port found after %d", desiredPort)
	}
	useAlt := false
	msg := fmt.Sprintf("Port %d is in use. Use available port %d instead?", desiredPort, altPort)
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().Title(msg).Value(&useAlt),
		),
	).WithTheme(huh.ThemeCharm())
	if err := form.Run(); err != nil {
		return -1, err
	}
	if useAlt {
		return altPort, nil
	}
	return -1, fmt.Errorf("user declined to use alternative port")
}

// HandleFormError provides consistent error handling for huh form operations
func HandleFormError(err error, operationName string) bool {
	if err == nil {
		return false // No error, continue
	}
	
	if errors.Is(err, huh.ErrUserAborted) {
		fmt.Println(InfoStyle.Render(fmt.Sprintf("%s cancelled by user.", operationName)))
		return true // User cancelled, should exit
	}
	
	fmt.Println(ErrorStyle.Render(fmt.Sprintf("Form error during %s: %v", operationName, err)))
	return true // Error occurred, should exit
}
