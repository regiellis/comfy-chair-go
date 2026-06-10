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

Run tests with `go test ./...` (placeholder rendering in `nodes_test.go`, torch-install/sysinfo tests in `internal/`, and an httptest end-to-end suite in `internal/assets/`). Releases are cut by `.github/workflows/release.yml` via GoReleaser (linux/windows/darwin amd64, `CGO_ENABLED=0`).

Bump `AppVersion` in `internal/constants.go` when releasing.

## Architecture

Three Go packages:

- **root `package main`** — `main.go` (CLI entry, ComfyUI lifecycle, install, env management, migrations, asset-server command), `nodes.go` (node CRUD/scaffolding/packing), `reload.go` (fsnotify watcher + debounced restart), `procattr_unix.go` / `procattr_windows.go` (platform process attributes via build tags).
- **`internal/`** — shared modules: `cli.go` (CLIRouter), `core.go` (active-install resolution + env-confirmation wrapper), `menu.go` (interactive TUI), `install.go` (torch/GPU install, node requirements), `process.go`, `pidfile.go`, `health.go`, `performance.go`, `sysinfo_unix.go`/`sysinfo_windows.go`, `utils.go` (config registry, state-file resolution), `logger.go`, `constants.go`.
- **`internal/assets/`** — the asset manager web server (see below); decoupled from `internal`.

### Command dispatch flow (in `main()`)

1. `initPaths()` resolves config/paths.
2. `internal.NewCLIRouter(...)` + `router.SetupCLICommands(...)` registers command handlers — handlers are defined in `main.go` and **injected into the internal router as function values** (e.g. `startComfyUI`, `createNewNode`). Standalone commands (`assets`, `empty-trash`) are registered with `router.RegisterCommand` directly. `internal` owns routing/help/flags, `main` owns the implementations.
3. `router.Route(os.Args)` handles the command (unknown commands exit inside `Route`); with no command the **interactive TUI menu** (`internal/menu.go`) launches.

When adding a command, wire it in three places consistently: the handler in `main.go`, registration in `SetupCLICommands` (or `RegisterCommand` + the help-category list in `ShowHelp`), and (if user-facing) the menu via `MenuChoices`. Command names support both `snake_case` and `kebab-case` aliases.

### Multi-environment model (important)

Comfy Chair manages **multiple named ComfyUI installs** (`lounge`, `den`, `nook`), persisted in **`comfy-installs.json`** (path, type, `is_default`, `custom_nodes`, and `reload_include_dirs`). Lifecycle commands take a `*internal.ComfyInstall` parameter and run through `internal.RunWithEnvConfirmation(action, fn)`, which resolves/prompts for the target environment and passes it to the handler — handlers must act on the passed install, never re-resolve. `WORKING_COMFY_ENV` in `.env` pins the active environment.

### Configuration

- State files (`.env`, `comfy-installs.json`, `comfy-performance-history.json`) live in **`os.UserConfigDir()/comfy-chair/`** (e.g. `~/.config/comfy-chair/`), resolved via `internal.ResolveStateFile`, which one-time copy-migrates files from the legacy binary-adjacent location. Pid/log files (`comfyui.pid`, `comfyui.log`) live in the ComfyUI install directory.
- **`.env`** (loaded via `godotenv`): `COMFYUI_PATH` (required), `COMFY_RELOAD_EXTS`, `COMFY_RELOAD_DEBOUNCE`, `COMFY_START_FLAGS`, `GPU_TYPE`, `PYTHON_VERSION`, `TORCH_INSTALL_CMD_NVIDIA`, `CUSTOM_NODES_AUTHOR`, `CUSTOM_NODES_PUBID`, `WORKING_COMFY_ENV`. Missing required vars trigger interactive setup. Copy from `.env.example`.
- **`comfy-installs.json`**: the multi-environment registry described above.
- Paths in config support portable placeholders `{HOME}` / `{USERPROFILE}` (resolved in `internal.ExpandUserPath`).

### venv detection (by design constraint)

Only venvs named exactly **`venv`** or **`.venv`** inside the ComfyUI dir are recognized (`FindVenvPython`). Custom venv names are intentionally unsupported — do not add support for them. Python deps are managed with **`uv`**, and PyTorch install is GPU-specific (`GPU_TYPE`).

### Node scaffolding

`create-node` copies a template tree from **`templates/`** (`node`, `advanced-node`, `api-node`, `model-node`, `webapi-node`) and substitutes placeholders defined in `internal/constants.go`: `{{NodeName}}`, `{{NodeNameLower}}`, `{{NodeDesc}}`, `{{Author}}`, `{{PubID}}` (the latter two from `.env`). Filenames themselves contain placeholders (e.g. `{{NodeNameLower}}.py`) and are renamed during copy. Templates target the **ComfyUI V3 node schema** (`comfy_api.latest`: `IO.ComfyNode` subclasses with `define_schema()`/classmethod `execute()`, registered via a `ComfyExtension` + `comfy_entrypoint()`), which requires ComfyUI >= 0.10.0. Templates are embedded via `go:embed` — **rebuild the binary after editing them**.

### Reload

`reload`/`watch_nodes` watches `custom_nodes` with `fsnotify`, debounced by `COMFY_RELOAD_DEBOUNCE`, filtered to `COMFY_RELOAD_EXTS`. Watching is **opt-in per directory**: only dirs in the install's `reload_include_dirs` are watched; symlinks are resolved.

### Asset manager

`comfy-chair assets [--host H] [--port N]` starts a web UI (default `0.0.0.0` — all interfaces, with a no-auth warning; auto-port from 8189; embedded via `go:embed` in **`internal/assets/`**) that browses `output/`, `input/`, and `user/default/workflows/` across **all registered environments**. `--host 127.0.0.1` restricts to localhost; `COMFY_ASSETS_HOST`/`COMFY_ASSETS_PORT` env vars are fallbacks. The package is decoupled from `internal` (it has its own `Env` type; `main.go` maps `ComfyInstall` → `assets.Env`). The frontend is **server-rendered Go templates swapped by htmx** (vendored in `web/vendor/` with anime.js; no bundler) plus a slim `app.js` glue layer for selection state, lightbox, and transitions. Views: `/grid` (day-grouped bento fragments with infinite scroll + folder bar), `/detail`, `/dupes` (hash-based duplicate review backed by a persistent hash cache). Bulk JSON actions accept either explicit `items` or a `filter` (acts on every match server-side). Capabilities: cached thumbnails (`~/.config/comfy-chair/thumb-cache/`), PNG `tEXt` metadata extraction (embedded prompt/workflow), copy/move between environments, delete-to-`.trash` (cleared via the web UI button or `comfy-chair empty-trash`, also in the TUI under Other Tools), and extract-workflow-from-PNG. Video poster frames use ffmpeg when present, degrading to icon tiles otherwise. All file access goes through `resolveAssetPath`, which rejects paths escaping the env/type roots.

## Conventions

- gofmt-clean, idiomatic Go, standard error handling. Files use lower-case names.
- **Dry-run**: commands honor a global `--dry-run`/`-n` flag (parsed in `internal/cli.go`) — preserve this when adding destructive operations.
- **Security-sensitive**: this CLI executes external processes and manipulates user paths. Maintain the existing path-traversal protection, input sanitization, and command-injection safeguards (see `internal/utils.go`, `internal/process.go`, and `internal/assets/assets.go`) when touching node names, paths, or shell-outs.
- Conventional Commit prefixes (`feat:`, `fix:`, `chore(scope):`) with short imperative summaries.
- `specs/` holds the project's own coding guidelines (`.coding-rules.yaml`, `.coding-guildlines.yaml`, `.coding-nodes.md`) — consult `.coding-nodes.md` for ComfyUI node conventions when scaffolding/editing node templates.
