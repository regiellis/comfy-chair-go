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
	ComfyUIPathKey          = "COMFYUI_PATH"
	WorkingComfyEnvKey      = "WORKING_COMFY_ENV"
	ComfyStartFlagsKey      = "COMFY_START_FLAGS"
	ComfyFrontendVersionKey = "COMFY_FRONTEND_VERSION"
)

// Default values
const (
	AppVersion           = "1.7.1"
	DefaultPythonVersion = "3.13"

	// DefaultFrontendVersion is passed to --front-end-version when
	// COMFY_FRONTEND_VERSION is unset. "default" selects the bundled frontend.
	DefaultFrontendVersion = "v1.55.9"

	// ComfyUI repository
	ComfyUIRepoURL = "https://github.com/comfyanonymous/ComfyUI.git"
)

// Command constants
const (
	GitCommand = "git"
	GitClone   = "clone"
)
