# Repository Guidelines

## Project Structure & Module Organization
- `main.go`, `nodes.go`, `reload.go`, `procattr_*.go`: CLI entrypoints and platform-specific logic.
- `internal/`: core application modules (CLI, setup, install, process management, reload, health, logging).
- `templates/`: scaffolding for ComfyUI node templates (Python/JS/CSS assets).
- `demos/`: demo scripts and media.
- Root configs: `go.mod`, `go.sum`, `.env.example`, `taskfile.yml`.

## Build, Test, and Development Commands
- `task build`: build the local binary (`./comfy-chair`).
- `task run`: run the CLI after building.
- `task build-all`: cross-compile binaries into `dist/`.
- `task clean`: remove build artifacts.
- `task install`: `go install` into your Go bin.
- `go build -o comfy-chair .`: direct build without Taskfile.

## Coding Style & Naming Conventions
- Go code follows `gofmt` formatting; use standard Go idioms and error handling.
- Files and packages use lower_snake or lower case typical for Go (e.g., `internal/health.go`).
- CLI command names support `snake_case` and `kebab-case` (e.g., `create_node` and `create-node`).

## Testing Guidelines
- There are currently no `*_test.go` files.
- If you add tests, use the Go testing framework and run `go test ./...`.
- Name tests as `TestXxx` in `*_test.go` and keep behavior-focused coverage.

## Commit & Pull Request Guidelines
- Git history uses Conventional Commit-style prefixes (`feat:`, `chore(scope):`, `fix:`). Follow the same pattern with short, imperative summaries.
- PRs should include: a clear description, any relevant issue links, and testing notes (commands run).
- If your changes affect CLI behavior or templates, include a brief before/after note or example command.

## Configuration & Local Setup
- Copy `.env.example` to `.env` and set `COMFYUI_PATH=/path/to/your/ComfyUI`.
- Optional settings include `COMFY_RELOAD_EXTS`, `COMFY_RELOAD_DEBOUNCE`, and `COMFY_START_FLAGS`.
