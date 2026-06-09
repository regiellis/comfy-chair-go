# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

Comfy Chair is a single-binary Go CLI (Charm `huh`/`lipgloss` TUI + `fsnotify`) for managing ComfyUI installations and developing custom nodes: lifecycle control (start/stop/restart/update/install), node scaffolding from templates, live-reload on file changes, and node packaging. It shells out to `git`, `uv`, and the ComfyUI venv's Python; it does not embed ComfyUI.

## Build / run / test

```bash
task build          # go build -o comfy-chair .  (preferred)
task build-dev      # build with debug symbols (-gcflags="all=-N -l")
task build-all      # cross-compile linux/darwin/windows amd64 into dist/
task run            # run ./comfy-chair
task install        # go install into $GOBIN
go build -o comfy-chair .   # direct build, no Taskfile

go vet ./...        # vet
gofmt -l .          # check formatting (code must be gofmt-clean)
```

There are **no `*_test.go` files** in the repo. If you add tests, use the standard framework and run `go test ./...`. Releases are cut by `.github/workflows/release.yml` via GoReleaser (linux/windows/darwin amd64, `CGO_ENABLED=0`).

Bump `AppVersion` in `internal/constants.go` when releasing.

## Architecture

Two Go packages:

- **root `package main`** — `main.go` (~91k: CLI entry, ComfyUI lifecycle, install, env management, migrations), `nodes.go` (node CRUD/scaffolding/packing), `reload.go` (fsnotify watcher + debounced restart), `procattr_unix.go` / `procattr_windows.go` (platform process attributes via build tags).
- **`internal/`** — shared modules: `cli.go` (CLIRouter), `core.go` (active-install resolution + env-confirmation wrapper), `menu.go` (interactive TUI), `install.go`, `process.go`, `pidfile.go`, `health.go`, `performance.go`, `migration.go`, `pathutil.go`, `utils.go`, `logger.go`, `constants.go`.

### Command dispatch flow (in `main()`)

1. `initPaths()` resolves config/paths.
2. `internal.NewCLIRouter(...)` + `router.SetupCLICommands(...)` registers command handlers — note handlers are defined in `main.go` and **injected into the internal router as function values** (e.g. `startComfyUIWithEnv`, `createNewNode`). This is the primary indirection to understand: `internal` owns routing/help/flags, `main` owns the implementations.
3. `router.Route(os.Args)` handles the command; if it returns `false`, a **legacy `switch` in `main.go` is the fallback**, and with no args the **interactive TUI menu** (`internal/menu.go`) launches.

When adding a command, wire it in three places consistently: the handler in `main.go`, registration in `SetupCLICommands`, and (if user-facing) the menu. All command names support both `snake_case` and `kebab-case` aliases.

### Multi-environment model (important)

Comfy Chair manages **multiple named ComfyUI installs** (`lounge`, `den`, `nook` — see `*InstallName` constants), persisted in **`comfy-installs.json`** (path, type, `is_default`, `custom_nodes`, and `reload_include_dirs`). Most lifecycle operations resolve the active install via `internal.GetActiveComfyInstall()` and run through `internal.RunWithEnvConfirmation(action, fn)`, which prompts the user to confirm which environment the action targets. The `*WithEnv` handler variants in `main.go` operate on a resolved `*internal.ComfyInstall`. `WORKING_COMFY_ENV` in `.env` can pin the active environment.

### Configuration

- State files (`.env`, `comfy-installs.json`, `comfy-performance-history.json`) live in **`os.UserConfigDir()/comfy-chair/`** (e.g. `~/.config/comfy-chair/`), resolved via `internal.ResolveStateFile`, which one-time copy-migrates files from the legacy binary-adjacent location. Pid/log files (`comfyui.pid`, `comfyui.log`) live in the ComfyUI install directory.
- **`.env`** (loaded via `godotenv`): `COMFYUI_PATH` (required), `COMFY_RELOAD_EXTS`, `COMFY_RELOAD_DEBOUNCE`, `COMFY_START_FLAGS`, `GPU_TYPE`, `PYTHON_VERSION`, `TORCH_INSTALL_CMD_NVIDIA`, `CUSTOM_NODES_AUTHOR`, `CUSTOM_NODES_PUBID`, `WORKING_COMFY_ENV`. Missing required vars trigger interactive setup. Copy from `.env.example`.
- **`comfy-installs.json`**: the multi-environment registry described above.
- Paths in config support portable placeholders `{HOME}` / `{USERPROFILE}` (resolved in `internal/pathutil.go`).

### venv detection (by design constraint)

Only venvs named exactly **`venv`** or **`.venv`** inside the ComfyUI dir are recognized (`FindVenvPython`). Custom venv names are intentionally unsupported — do not add support for them. Python deps are managed with **`uv`**, and PyTorch install is GPU-specific (`GPU_TYPE`).

### Node scaffolding

`create-node` copies a template tree from **`templates/`** (`node`, `advanced-node`, `api-node`, `model-node`, `webapi-node`) and substitutes placeholders defined in `internal/constants.go`: `{{NodeName}}`, `{{NodeNameLower}}`, `{{NodeDesc}}`, `{{Author}}`, `{{PubID}}` (the latter two from `.env`). Filenames themselves contain placeholders (e.g. `{{NodeNameLower}}.py`) and are renamed during copy.

### Reload

`reload`/`watch_nodes` watches `custom_nodes` with `fsnotify`, debounced by `COMFY_RELOAD_DEBOUNCE`, filtered to `COMFY_RELOAD_EXTS`. Watching is **opt-in per directory**: only dirs in the install's `reload_include_dirs` are watched; symlinks are resolved.

## Conventions

- gofmt-clean, idiomatic Go, standard error handling. Files use lower-case names.
- **Dry-run**: commands honor a global `--dry-run`/`-n` flag (parsed in `internal/cli.go`) — preserve this when adding destructive operations.
- **Security-sensitive**: this CLI executes external processes and manipulates user paths. Maintain the existing path-traversal protection, input sanitization, and command-injection safeguards (see `internal/pathutil.go`, `internal/utils.go`) when touching node names, paths, or shell-outs.
- Conventional Commit prefixes (`feat:`, `fix:`, `chore(scope):`) with short imperative summaries.
- `specs/` holds the project's own coding guidelines (`.coding-rules.yaml`, `.coding-guildlines.yaml`, `.coding-nodes.md`) — consult `.coding-nodes.md` for ComfyUI node conventions when scaffolding/editing node templates.
