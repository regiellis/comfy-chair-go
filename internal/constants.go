package internal

// File and directory constants
const (
	// Directory names
	CustomNodesDir = "custom_nodes"
	VenvDir        = "venv"
	DotVenvDir     = ".venv"
	
	// File names
	RequirementsTxt = "requirements.txt"
	ComfyUIPidFile  = "comfyui.pid"
	ComfyUILogFile  = "comfyui.log"
	EnvFileName     = ".env"
	MainPyFile      = "main.py"
	
	// Configuration file names
	ComfyInstallsConfig = "comfy-installs.json"
	
	// File extensions
	PythonExt     = ".py"
	JavaScriptExt = ".js"
	CSSExt        = ".css"
	
	// Template placeholders
	NodeNamePlaceholder      = "{{NodeName}}"
	NodeNameLowerPlaceholder = "{{NodeNameLower}}"
	NodeDescPlaceholder      = "{{NodeDesc}}"
	AuthorPlaceholder        = "{{Author}}"
	PubIDPlaceholder         = "{{PubID}}"
)

// Environment variable constants
const (
	ComfyUIPathKey     = "COMFYUI_PATH"
	GPUTypeKey         = "GPU_TYPE"
	PythonVersionKey   = "PYTHON_VERSION"
	WorkingComfyEnvKey = "WORKING_COMFY_ENV"
	
	// Reload configuration
	ComfyReloadExtsKey     = "COMFY_RELOAD_EXTS"
	ComfyReloadDebounceKey = "COMFY_RELOAD_DEBOUNCE"
)

// Default values
const (
	DefaultPort           = 8188
	DefaultPythonVersion  = "3.12"
	DefaultReloadExts     = ".py,.js,.css"
	DefaultReloadDebounce = "5"
	
	// Timeout values
	MaxWaitTimeSeconds = 60
	
	// ComfyUI repository
	ComfyUIRepoURL = "https://github.com/comfyanonymous/ComfyUI.git"
)

// Installation type constants
const (
	LoungeInstallName = "lounge"
	DenInstallName    = "den"
	NookInstallName   = "nook"
)

// Command constants
const (
	GitCommand    = "git"
	PythonCommand = "python"
	Python3Command = "python3"
	PipCommand    = "pip"
	UVCommand     = "uv"
)

// Git commands
const (
	GitClone = "clone"
	GitPull  = "pull"
	GitStash = "stash"
)

// Common messages
const (
	OperationCancelledMsg = "Operation cancelled by user"
	InstallationCancelledMsg = "Installation cancelled"
	NodeCreationCancelledMsg = "Node creation cancelled"
)