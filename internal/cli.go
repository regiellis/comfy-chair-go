package internal

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/charmbracelet/huh"
)

// CommandHandler represents a function that handles a CLI command
type CommandHandler func()

// EnvironmentCommandHandler represents a function that handles a command with environment context
type EnvironmentCommandHandler func(inst *ComfyInstall)

// Command represents a CLI command with its metadata
type Command struct {
	Name         string
	Aliases      []string
	Description  string
	Handler      CommandHandler
	EnvHandler   EnvironmentCommandHandler
	RequiresEnv  bool
}

// CLIRouter handles command registration and routing
type CLIRouter struct {
	commands   map[string]*Command
	paths      *Paths
	reloadFunc func(string, int, []string, []string)
}

// NewCLIRouter creates a new CLI router
func NewCLIRouter(paths *Paths, reloadFunc func(string, int, []string, []string)) *CLIRouter {
	return &CLIRouter{
		commands:   make(map[string]*Command),
		paths:      paths,
		reloadFunc: reloadFunc,
	}
}

// RegisterCommand registers a command with the router
func (r *CLIRouter) RegisterCommand(cmd *Command) {
	// Register the main command name
	r.commands[cmd.Name] = cmd
	
	// Register all aliases
	for _, alias := range cmd.Aliases {
		r.commands[alias] = cmd
	}
}

// Route processes command line arguments and executes the appropriate command
func (r *CLIRouter) Route(args []string) bool {
	if len(args) <= 1 {
		return false // No command provided, fall back to interactive menu
	}
	
	commandName := args[1]
	cmd, exists := r.commands[commandName]
	if !exists {
		fmt.Printf("Unknown command: %s\n", commandName)
		r.ShowHelp()
		os.Exit(1)
	}
	
	// Execute the command
	if cmd.RequiresEnv && cmd.EnvHandler != nil {
		RunWithEnvConfirmation(commandName, cmd.EnvHandler)
	} else if cmd.Handler != nil {
		cmd.Handler()
	} else {
		fmt.Printf("No handler defined for command: %s\n", commandName)
		os.Exit(1)
	}
	
	return true
}

// ShowHelp displays available commands
func (r *CLIRouter) ShowHelp() {
	fmt.Println(TitleStyle.Render("Comfy Chair - ComfyUI Development Tool"))
	fmt.Println()
	fmt.Println(InfoStyle.Render("Usage: comfy-chair [command]"))
	fmt.Println()
	fmt.Println(InfoStyle.Render("Available Commands:"))
	fmt.Println()
	
	categories := map[string][]*Command{
		"ComfyUI Management": {},
		"Development Tools":  {},
		"Node Management":    {},
		"Migration Tools":    {},
		"Environment":        {},
		"Help":              {},
	}
	
	// Collect unique commands (avoid duplicates from aliases)
	seen := make(map[string]bool)
	for _, cmd := range r.commands {
		if !seen[cmd.Name] {
			seen[cmd.Name] = true
			
			// Categorize commands
			switch {
			case contains([]string{"start", "background", "stop", "restart", "update", "status"}, cmd.Name):
				categories["ComfyUI Management"] = append(categories["ComfyUI Management"], cmd)
			case contains([]string{"reload", "install"}, cmd.Name):
				categories["Development Tools"] = append(categories["Development Tools"], cmd)
			case contains([]string{"create-node", "list-nodes", "delete-node", "pack-node", "update-nodes"}, cmd.Name):
				categories["Node Management"] = append(categories["Node Management"], cmd)
			case contains([]string{"migrate-nodes", "migrate-workflows", "migrate-images", "node-workflows"}, cmd.Name):
				categories["Migration Tools"] = append(categories["Migration Tools"], cmd)
			case contains([]string{"remove-env", "sync-env"}, cmd.Name):
				categories["Environment"] = append(categories["Environment"], cmd)
			case contains([]string{"help"}, cmd.Name):
				categories["Help"] = append(categories["Help"], cmd)
			}
		}
	}
	
	// Display categorized commands
	for category, commands := range categories {
		if len(commands) > 0 {
			fmt.Println(SuccessStyle.Render(category + ":"))
			for _, cmd := range commands {
				aliases := ""
				if len(cmd.Aliases) > 0 {
					aliases = " (" + strings.Join(cmd.Aliases, ", ") + ")"
				}
				fmt.Printf("  %-20s %s%s\n", cmd.Name, cmd.Description, aliases)
			}
			fmt.Println()
		}
	}
	
	fmt.Println(InfoStyle.Render("Run without arguments for interactive menu."))
}

// HandleReloadCommand handles the complex reload command logic
func (r *CLIRouter) HandleReloadCommand() {
	inst, err := GetActiveComfyInstall()
	if err != nil {
		fmt.Println(ErrorStyle.Render(err.Error()))
		PromptReturnToMenu()
		return
	}
	
	watchDir := ExpandUserPath(inst.Path)
	
	// Default values
	debounce := 5
	exts := []string{".py", ".js", ".css"}
	
	// Parse environment variables
	envVars, err := ReadEnvFile(r.paths.EnvFile)
	if err == nil {
		if debounceStr, exists := envVars["COMFY_RELOAD_DEBOUNCE"]; exists {
			if d, err := strconv.Atoi(debounceStr); err == nil {
				debounce = d
			}
		}
		if extsStr, exists := envVars["COMFY_RELOAD_EXTS"]; exists {
			if extsStr != "" {
				extsList := strings.Split(extsStr, ",")
				exts = make([]string, len(extsList))
				for i, ext := range extsList {
					exts[i] = strings.TrimSpace(ext)
				}
			}
		}
	}
	
	// Get included directories from configuration
	includedDirs := inst.ReloadIncludeDirs
	if len(includedDirs) == 0 {
		// Prompt user to select directories
		customNodesDir := ExpandUserPath(filepath.Join(watchDir, "custom_nodes"))
		files, err := os.ReadDir(customNodesDir)
		if err != nil {
			fmt.Println(ErrorStyle.Render(fmt.Sprintf("Failed to read custom nodes directory: %v", err)))
			return
		}
		
		var nodeNames []string
		for _, file := range files {
			if file.IsDir() {
				nodeNames = append(nodeNames, file.Name())
			}
		}
		
		if len(nodeNames) == 0 {
			fmt.Println(InfoStyle.Render("No custom nodes found."))
			return
		}
		
		// Multi-select for directories to watch
		var selected []string
		selectPrompt := huh.NewMultiSelect[string]().
			Title("Select custom node directories to watch (space to select, enter to confirm):").
			OptionsFunc(func() []huh.Option[string] {
				opts := make([]huh.Option[string], len(nodeNames))
				for i, name := range nodeNames {
					opts[i] = huh.NewOption(name, name)
				}
				return opts
			}, nil).
			Value(&selected)
		
		if err := selectPrompt.Run(); err != nil || len(selected) == 0 {
			fmt.Println(InfoStyle.Render("Reload cancelled - no directories selected."))
			return
		}
		
		includedDirs = selected
		
		// Save the selection to configuration
		inst.ReloadIncludeDirs = includedDirs
		cfg, err := LoadGlobalConfig()
		if err == nil {
			SaveGlobalConfig(cfg)
		}
	}
	
	// Call the actual reload function
	r.reloadFunc(watchDir, debounce, exts, includedDirs)
}

// contains checks if a slice contains a string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// SetupCLICommands registers all available CLI commands with their handlers
func (r *CLIRouter) SetupCLICommands(
	startComfyUIFunc func(*ComfyInstall, bool),
	stopComfyUIFunc func(*ComfyInstall),
	restartComfyUIFunc func(*ComfyInstall),
	updateComfyUIFunc func(*ComfyInstall),
	statusComfyUIFunc func(*ComfyInstall),
	installFunc func(),
	createNodeFunc func(),
	listNodesFunc func(),
	deleteNodeFunc func(),
	packNodeFunc func(),
	updateNodesFunc func(),
	migrateNodesFunc func(),
	migrateWorkflowsFunc func(),
	migrateImagesFunc func(),
	nodeWorkflowsFunc func(),
	removeEnvFunc func(),
	syncEnvFunc func(),
) {
	// ComfyUI Management Commands
	r.RegisterCommand(&Command{
		Name:        "start",
		Aliases:     []string{"start_fg", "start-fg"},
		Description: "Start ComfyUI in foreground mode",
		EnvHandler:  func(inst *ComfyInstall) { startComfyUIFunc(inst, false) },
		RequiresEnv: true,
	})
	
	r.RegisterCommand(&Command{
		Name:        "background",
		Aliases:     []string{"start_bg", "start-bg"},
		Description: "Start ComfyUI in background mode",
		EnvHandler:  func(inst *ComfyInstall) { startComfyUIFunc(inst, true) },
		RequiresEnv: true,
	})
	
	r.RegisterCommand(&Command{
		Name:        "stop",
		Aliases:     []string{},
		Description: "Stop ComfyUI",
		EnvHandler:  stopComfyUIFunc,
		RequiresEnv: true,
	})
	
	r.RegisterCommand(&Command{
		Name:        "restart",
		Aliases:     []string{},
		Description: "Restart ComfyUI",
		EnvHandler:  restartComfyUIFunc,
		RequiresEnv: true,
	})
	
	r.RegisterCommand(&Command{
		Name:        "update",
		Aliases:     []string{},
		Description: "Update ComfyUI",
		EnvHandler:  updateComfyUIFunc,
		RequiresEnv: true,
	})
	
	r.RegisterCommand(&Command{
		Name:        "status",
		Aliases:     []string{},
		Description: "Show ComfyUI status and environment info",
		EnvHandler:  statusComfyUIFunc,
		RequiresEnv: true,
	})
	
	// Development Tools
	r.RegisterCommand(&Command{
		Name:        "reload",
		Aliases:     []string{},
		Description: "Watch files and auto-restart ComfyUI on changes",
		Handler:     r.HandleReloadCommand,
		RequiresEnv: false,
	})
	
	r.RegisterCommand(&Command{
		Name:        "install",
		Aliases:     []string{},
		Description: "Install or reconfigure ComfyUI",
		Handler:     installFunc,
		RequiresEnv: false,
	})
	
	// Node Management
	r.RegisterCommand(&Command{
		Name:        "create-node",
		Aliases:     []string{"create_node"},
		Description: "Create a new custom node",
		Handler:     createNodeFunc,
		RequiresEnv: false,
	})
	
	r.RegisterCommand(&Command{
		Name:        "list-nodes",
		Aliases:     []string{"list_nodes"},
		Description: "List all custom nodes",
		Handler:     listNodesFunc,
		RequiresEnv: false,
	})
	
	r.RegisterCommand(&Command{
		Name:        "delete-node",
		Aliases:     []string{"delete_node"},
		Description: "Delete a custom node",
		Handler:     deleteNodeFunc,
		RequiresEnv: false,
	})
	
	r.RegisterCommand(&Command{
		Name:        "pack-node",
		Aliases:     []string{"pack_node"},
		Description: "Pack a custom node for distribution",
		Handler:     packNodeFunc,
		RequiresEnv: false,
	})
	
	r.RegisterCommand(&Command{
		Name:        "update-nodes",
		Aliases:     []string{},
		Description: "Update custom nodes",
		Handler:     updateNodesFunc,
		RequiresEnv: false,
	})
	
	// Migration Tools
	r.RegisterCommand(&Command{
		Name:        "migrate-nodes",
		Aliases:     []string{},
		Description: "Migrate custom nodes between environments",
		Handler:     migrateNodesFunc,
		RequiresEnv: false,
	})
	
	r.RegisterCommand(&Command{
		Name:        "migrate-workflows",
		Aliases:     []string{},
		Description: "Migrate workflows between environments",
		Handler:     migrateWorkflowsFunc,
		RequiresEnv: false,
	})
	
	r.RegisterCommand(&Command{
		Name:        "migrate-images",
		Aliases:     []string{},
		Description: "Migrate images/videos/audio between environments",
		Handler:     migrateImagesFunc,
		RequiresEnv: false,
	})
	
	r.RegisterCommand(&Command{
		Name:        "node-workflows",
		Aliases:     []string{},
		Description: "Add/remove custom node workflows in main workflows folder",
		Handler:     nodeWorkflowsFunc,
		RequiresEnv: false,
	})
	
	// Environment Management
	r.RegisterCommand(&Command{
		Name:        "remove-env",
		Aliases:     []string{},
		Description: "Remove (disconnect) an environment from config",
		Handler:     removeEnvFunc,
		RequiresEnv: false,
	})
	
	r.RegisterCommand(&Command{
		Name:        "sync-env",
		Aliases:     []string{},
		Description: "Sync .env with .env.example",
		Handler:     syncEnvFunc,
		RequiresEnv: false,
	})
	
	// Help
	r.RegisterCommand(&Command{
		Name:        "help",
		Aliases:     []string{"--help", "-h"},
		Description: "Show this help message",
		Handler:     r.ShowHelp,
		RequiresEnv: false,
	})
}