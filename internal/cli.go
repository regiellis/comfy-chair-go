package internal

import (
	"fmt"
	"os"
	"strings"
)

// CommandHandler represents a function that handles a CLI command
type CommandHandler func()

// EnvironmentCommandHandler represents a function that handles a command with environment context
type EnvironmentCommandHandler func(inst *ComfyInstall)

// GlobalFlags represents global CLI flags that apply to all commands
type GlobalFlags struct {
	DryRun  bool
	Version bool
}

// Command represents a CLI command with its metadata
type Command struct {
	Name        string
	Aliases     []string
	Description string
	Handler     CommandHandler
	EnvHandler  EnvironmentCommandHandler
	RequiresEnv bool
}

// CLIRouter handles command registration and routing
type CLIRouter struct {
	commands   map[string]*Command
	reloadFunc func(*ComfyInstall, string, int, []string, []string) error
	dryRun     bool
}

// NewCLIRouter creates a new CLI router
func NewCLIRouter(reloadFunc func(*ComfyInstall, string, int, []string, []string) error) *CLIRouter {
	return &CLIRouter{
		commands:   make(map[string]*Command),
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

	// Parse global flags and filter them out
	filteredArgs, globalFlags := r.parseGlobalFlags(args)
	r.dryRun = globalFlags.DryRun

	// Set global dry-run state
	SetDryRunMode(r.dryRun)

	if len(filteredArgs) <= 1 {
		if globalFlags.Version {
			r.ShowVersion()
			return true
		}
		return false // No command after flags, fall back to interactive menu
	}

	commandName := filteredArgs[1]
	cmd, exists := r.commands[commandName]
	if !exists {
		fmt.Printf("Unknown command: %s\n", commandName)
		r.ShowHelp()
		os.Exit(1)
	}

	// Show dry-run message if enabled
	if r.dryRun {
		fmt.Println(WarningStyle.Render("🔍 DRY RUN MODE - No actual changes will be made"))
		fmt.Println()
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

// parseGlobalFlags parses global flags from command line arguments
func (r *CLIRouter) parseGlobalFlags(args []string) ([]string, GlobalFlags) {
	var flags GlobalFlags
	var filteredArgs []string

	// Keep the program name
	if len(args) > 0 {
		filteredArgs = append(filteredArgs, args[0])
	}

	for i := 1; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "--dry-run", "-n":
			flags.DryRun = true
		case "--help", "-h":
			// Help flag is handled as a command, so include it
			filteredArgs = append(filteredArgs, arg)
		case "--version", "-v":
			flags.Version = true
		default:
			// Not a global flag, add to filtered args
			filteredArgs = append(filteredArgs, arg)
		}
	}

	return filteredArgs, flags
}

// IsDryRun returns whether dry-run mode is enabled
func (r *CLIRouter) IsDryRun() bool {
	return r.dryRun
}

// ShowHelp displays available commands
func (r *CLIRouter) ShowHelp() {
	fmt.Println(TitleStyle.Render(fmt.Sprintf("Comfy Chair v%s - ComfyUI Development Tool", AppVersion)))
	fmt.Println()
	fmt.Println(InfoStyle.Render("Usage: comfy-chair [global-flags] [command]"))
	fmt.Println()
	fmt.Println(InfoStyle.Render("Global Flags:"))
	fmt.Println("  --dry-run, -n    Show what would be done without making changes")
	fmt.Println("  --help, -h       Show this help message")
	fmt.Println("  --version, -v    Show the current Comfy Chair version")
	fmt.Println()
	fmt.Println(InfoStyle.Render("Available Commands:"))
	fmt.Println()

	categories := map[string][]*Command{
		"ComfyUI Management": {},
		"Development Tools":  {},
		"Node Management":    {},
		"Migration Tools":    {},
		"Environment":        {},
		"Help":               {},
	}

	// Collect unique commands (avoid duplicates from aliases)
	seen := make(map[string]bool)
	for _, cmd := range r.commands {
		if !seen[cmd.Name] {
			seen[cmd.Name] = true

			// Categorize commands
			switch {
			case contains([]string{"start", "background", "stop", "restart", "update", "nightly", "downgrade", "status"}, cmd.Name):
				categories["ComfyUI Management"] = append(categories["ComfyUI Management"], cmd)
			case contains([]string{"reload", "install", "watch_nodes", "assets", "empty-trash"}, cmd.Name):
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

func (r *CLIRouter) ShowVersion() {
	fmt.Printf("comfy-chair v%s\n", AppVersion)
}

// HandleReloadCommand resolves the active install, gathers (and if needed
// prompts for) the reload settings, and starts the watcher.
func (r *CLIRouter) HandleReloadCommand() {
	inst, err := GetActiveComfyInstall()
	if err != nil {
		fmt.Println(ErrorStyle.Render(err.Error()))
		PromptReturnToMenu()
		return
	}

	watchDir, debounce, exts, includedDirs := GetReloadSettings(inst)
	if len(includedDirs) == 0 {
		fmt.Println(InfoStyle.Render("Reload cancelled - no directories selected."))
		return
	}

	if err := r.reloadFunc(inst, watchDir, debounce, exts, includedDirs); err != nil {
		fmt.Println(ErrorStyle.Render(fmt.Sprintf("Reload failed: %v", err)))
		os.Exit(1)
	}
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
	updateNightlyFunc func(*ComfyInstall),
	downgradeComfyUIFunc func(*ComfyInstall),
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
	watchNodesFunc func(),
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
		Description: "Update ComfyUI to latest stable tag",
		EnvHandler:  updateComfyUIFunc,
		RequiresEnv: true,
	})

	r.RegisterCommand(&Command{
		Name:        "nightly",
		Aliases:     []string{"update-nightly", "nightly-update"},
		Description: "Update ComfyUI to latest nightly (default branch)",
		EnvHandler:  updateNightlyFunc,
		RequiresEnv: true,
	})

	r.RegisterCommand(&Command{
		Name:        "downgrade",
		Aliases:     []string{"update-downgrade"},
		Description: "Downgrade ComfyUI to a selected stable tag",
		EnvHandler:  downgradeComfyUIFunc,
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
		Aliases:     []string{"update_nodes"},
		Description: "Update custom nodes",
		Handler:     updateNodesFunc,
		RequiresEnv: false,
	})

	r.RegisterCommand(&Command{
		Name:        "watch_nodes",
		Aliases:     []string{"watch-nodes"},
		Description: "Select custom nodes to watch for reload (all others excluded)",
		Handler:     watchNodesFunc,
		RequiresEnv: false,
	})

	// Migration Tools
	r.RegisterCommand(&Command{
		Name:        "migrate-nodes",
		Aliases:     []string{"migrate_nodes"},
		Description: "Migrate custom nodes between environments",
		Handler:     migrateNodesFunc,
		RequiresEnv: false,
	})

	r.RegisterCommand(&Command{
		Name:        "migrate-workflows",
		Aliases:     []string{"migrate_workflows"},
		Description: "Migrate workflows between environments",
		Handler:     migrateWorkflowsFunc,
		RequiresEnv: false,
	})

	r.RegisterCommand(&Command{
		Name:        "migrate-images",
		Aliases:     []string{"migrate_images"},
		Description: "Migrate images/videos/audio between environments",
		Handler:     migrateImagesFunc,
		RequiresEnv: false,
	})

	r.RegisterCommand(&Command{
		Name:        "node-workflows",
		Aliases:     []string{"node_workflows"},
		Description: "Add/remove custom node workflows in main workflows folder",
		Handler:     nodeWorkflowsFunc,
		RequiresEnv: false,
	})

	// Environment Management
	r.RegisterCommand(&Command{
		Name:        "remove-env",
		Aliases:     []string{"remove_env"},
		Description: "Remove (disconnect) an environment from config",
		Handler:     removeEnvFunc,
		RequiresEnv: false,
	})

	r.RegisterCommand(&Command{
		Name:        "sync-env",
		Aliases:     []string{"sync_env"},
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

	r.RegisterCommand(&Command{
		Name:        "version",
		Aliases:     []string{"--version", "-v"},
		Description: "Show the current Comfy Chair version",
		Handler:     r.ShowVersion,
		RequiresEnv: false,
	})
}
