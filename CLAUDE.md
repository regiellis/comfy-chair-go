# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Comfy Chair is a Go-based CLI tool for managing ComfyUI installations and custom node development. It provides rapid node scaffolding, live reload capabilities, and ComfyUI lifecycle management across multiple platforms.

## Build Commands

- `task build` - Build the comfy-chair binary for current OS/arch
- `task build-all` - Build for multiple OS/arch targets (Linux, macOS, Windows)
- `task build-dev` - Build with debug symbols for development
- `task clean` - Remove build artifacts
- `task install` - Install to $GOBIN or $GOPATH/bin
- `task run` - Run the built binary
- `go build -o comfy-chair .` - Direct Go build command

## Testing

The project does not appear to have a formal test suite currently. Any new testing should follow Go conventions with `_test.go` files.

## Architecture

### Core Components

- **main.go** - Entry point with CLI command routing and menu system
- **nodes.go** - Custom node creation, templating, and management functionality
- **reload.go** - File system watching and auto-restart functionality for development
- **internal/utils.go** - Shared utilities, configuration management, and path handling
- **internal/install.go** - ComfyUI installation and setup logic
- **internal/setup.go** - Environment configuration and .env management
- **internal/core.go** - Core functions for environment management and active install detection
- **internal/process.go** - Process management, PID tracking, and command execution utilities

### Key Concepts

- **Multi-Environment Support**: Supports three types of ComfyUI installs (Lounge/main, Den/dev, Nook/experimental)
- **Global Configuration**: Uses `comfy-installs.json` to track multiple ComfyUI installations
- **Cross-Platform**: Handles Windows/Unix differences in `procattr_*.go` files
- **Virtual Environment Detection**: Automatically finds `venv` or `.venv` directories
- **Template System**: Embedded templates in `templates/node/` for scaffolding new custom nodes

### Configuration Files

- `.env` - Local environment configuration (COMFYUI_PATH, GPU_TYPE, etc.)
- `comfy-installs.json` - Global configuration tracking multiple ComfyUI installations
- `taskfile.yml` - Task runner configuration for build/dev tasks

### Dependencies

Key Go modules:
- `github.com/charmbracelet/huh` - Interactive CLI prompts
- `github.com/charmbracelet/lipgloss` - Terminal styling
- `github.com/fsnotify/fsnotify` - File system watching
- `github.com/joho/godotenv` - .env file handling

## Development Patterns

### Path Handling
- Uses `internal.ExpandUserPath()` for cross-platform path expansion with `{HOME}` and `{USERPROFILE}` placeholders
- All paths are normalized through `filepath.Clean()`

### Error Handling
- Uses styled error messages through `internal.ErrorStyle.Render()`
- Graceful fallbacks for missing configurations

### Process Management
- PID file tracking for ComfyUI processes implemented in `internal/process.go`
- Process status caching to reduce system calls
- Cross-platform process detection (Windows/Unix)
- Port conflict detection and resolution

### Template Processing
- Embedded filesystem for node templates
- Placeholder replacement system for generating custom nodes
- Support for both Python and JavaScript components

## Important Environment Variables

- `COMFYUI_PATH` - Path to ComfyUI installation (required)
- `GPU_TYPE` - GPU type for PyTorch installation (nvidia, amd, intel, etc.)
- `PYTHON_VERSION` - Python version for venv (default: 3.12)
- `COMFY_RELOAD_EXTS` - File extensions to watch for reloads (default: .py,.js,.css)
- `COMFY_RELOAD_DEBOUNCE` - Debounce time for reloads (default: 5 seconds)
- `WORKING_COMFY_ENV` - Current working environment type

## Git Workflow and Branch Management

### Feature Development
- Create a new feature branch for each feature or enhancement: `git checkout -b feature/descriptive-name`
- Work on the feature branch, committing changes regularly after significant progress
- Write detailed commit messages explaining the "why" behind changes, not just the "what"
- Push the feature branch when the feature is complete: `git push -u origin feature/descriptive-name`
- Create pull requests from feature branches to main

### Hotfix Development
- Create hotfix branches for critical bug fixes: `git checkout -b hotfix/issue-description`
- Work on the hotfix branch, committing changes with clear descriptions
- **Do not push hotfix branches** - merge directly to main after testing
- Write detailed commit messages explaining the problem solved and approach taken

### Commit Message Guidelines
- Use clear, descriptive commit messages that explain the purpose and impact of changes
- Include context about why the change was needed, not just what was changed
- Reference relevant issues or discussions when applicable
- Example: "Add port conflict detection to prevent startup failures when default port 8188 is in use"

### Documentation Updates
- Always update the README.md when adding new features, changing behavior, or modifying usage patterns
- Update relevant sections to reflect new functionality or changed workflows
- Include examples for new features where appropriate

## File Structure Conventions

- Custom nodes are created in `{COMFYUI_PATH}/custom_nodes/`
- Virtual environments must be named `venv` or `.venv`
- Templates use `{{NodeName}}` and `{{NodeNameLower}}` placeholders
- Configuration files are stored alongside the binary executable

## Code Organization

### Internal Package Structure
The `internal/` package contains reusable modules:
- **core.go** - Environment management functions (`GetActiveComfyInstall`, `RunWithEnvConfirmation`)
- **process.go** - Process lifecycle management and PID tracking
- **utils.go** - Path handling, styling utilities, and shared helpers
- **install.go** - ComfyUI installation and setup logic
- **setup.go** - Environment configuration and .env management
- **constants.go** - Application constants and configuration keys

### Refactoring Guidelines
- Extract commonly used functions to appropriate internal modules
- Use `internal.FunctionName()` calls from main.go and other files
- Keep menu and CLI routing logic in main.go
- Move reusable business logic to internal packages

### User Experience Improvements (v1.3.2+)
- **Robust Environment Handling**: Graceful error handling when environment configurations are missing or invalid
- **Clear User Guidance**: Specific instructions on how to fix configuration issues through the UI
- **Consistent Menu Navigation**: All operations now include "Return to Main Menu" prompts after completion
- **Non-blocking Failures**: Configuration errors don't cause hard exits, users can navigate to Install/Setup options
- **Better Error Messages**: Informative error messages that guide users to solutions rather than just reporting failures

## Project Specifications & Development Roadmap

### Specification Directory Structure
The `./spec/` directory contains comprehensive improvement plans and technical specifications:

- **`spec/improvement-roadmap.md`** - Complete improvement roadmap with 4-phase development plan
- **`spec/phases/`** - Detailed specifications for each development phase
- **`spec/architecture/`** - Technical architecture and design documents
- **`spec/README.md`** - Overview and navigation guide

### Development Phase Strategy

#### Phase 1: Code Quality & Foundation (v1.4.0)
**Branch**: `feature/phase1-code-quality`
- Remove unused functions and clean up technical debt
- Add comprehensive testing infrastructure (target: 60% coverage)
- Continue main.go refactoring (extract menu, migration modules)
- Add version flag, improved logging, shell completion

#### Phase 2: Enhanced User Experience (v1.5.0)
**Branch**: `feature/phase2-user-experience`
- Interactive configuration wizard for first-time setup
- Configuration validation and health checking (`comfy-chair validate`)
- Enhanced help system with examples and troubleshooting
- Improved error handling with actionable solutions

#### Phase 3: Advanced Features (v1.6.0)
**Branch**: `feature/phase3-advanced-features`
- Performance monitoring and resource tracking
- Multiple node scaffolding templates
- Health checks and log analysis
- Node marketplace integration

#### Phase 4: Developer Experience (v1.7.0)
**Branch**: `feature/phase4-developer-experience`
- Debug mode and API integration
- Enhanced hot reload with selective reloading
- Development workflow improvements
- Plugin system and advanced tooling

### Branch Management for Phases

#### Feature Branch Workflow
1. **Create phase branch**: `git checkout -b feature/phase{N}-{description}`
2. **Implement incrementally**: Make small, focused commits with clear messages
3. **Regular integration**: Rebase against main to stay current
4. **Pull request review**: Each phase gets comprehensive review before merge
5. **Release tagging**: Tag releases at completion of each phase

#### Commit Message Guidelines for Phases
```
phase{N}: brief description of change

Detailed explanation of what was implemented and why.
Reference to specification document and acceptance criteria.

- Specific changes made
- Tests added or updated
- Documentation updates

Addresses: spec/phases/phase{N}-{name}.md section X.Y
```

### Quality Gates for Phase Development
- All tests must pass before merging
- Code coverage maintained or improved for each phase
- Documentation updated for user-facing changes
- Specification documents kept current with implementation
- Performance regressions identified and addressed