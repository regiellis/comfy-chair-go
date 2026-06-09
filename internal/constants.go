package internal

// File and directory constants
const (
	// Directory names
	CustomNodesDir = "custom_nodes"
	VenvDir        = "venv"

	// File names
	ComfyUIPidFile = "comfyui.pid"
	ComfyUILogFile = "comfyui.log"
	EnvFileName    = ".env"

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
	WorkingComfyEnvKey = "WORKING_COMFY_ENV"
	ComfyStartFlagsKey = "COMFY_START_FLAGS"
)

// Default values
const (
	AppVersion           = "1.5.0"
	DefaultPythonVersion = "3.13"

	// ComfyUI repository
	ComfyUIRepoURL = "https://github.com/comfyanonymous/ComfyUI.git"
)

// Command constants
const (
	GitCommand = "git"
	GitClone   = "clone"
)
