---
title: Configuration
nav_order: 7
---

# Configuration

## Where state lives

Config and state files live in your OS config directory under `comfy-chair/`:

| Platform | Location |
|----------|----------|
| Linux | `~/.config/comfy-chair/` |
| macOS | `~/Library/Application Support/comfy-chair/` |
| Windows | `%AppData%\comfy-chair\` |

Files there:

- **`.env`**: your settings, loaded on every run
- **`comfy-installs.json`**: the environment registry
- **`comfy-performance-history.json`**: startup timing history
- **`thumb-cache/`**: asset manager thumbnails

Older versions kept these next to the binary; they are migrated to the config directory automatically on first use. Pid and log files (`comfyui.pid`, `comfyui.log`) live inside each ComfyUI install directory instead, since they belong to that environment.

## .env reference

`COMFYUI_PATH` is required; everything else has a default. Missing required values trigger interactive setup. `sync-env` adds any keys you are missing from `.env.example`.

| Variable | Default | Meaning |
|----------|---------|---------|
| `COMFYUI_PATH` | (required) | Path to the active ComfyUI install. Kept in sync when you set a working environment. |
| `WORKING_COMFY_ENV` | unset | Pin the active environment by name and skip the environment picker. |
| `COMFY_RELOAD_EXTS` | `.py,.js,.css` | Comma-separated file extensions that trigger reloads. |
| `COMFY_RELOAD_DEBOUNCE` | `5` | Seconds of quiet required between reload restarts; changes inside the window coalesce. |
| `COMFY_START_FLAGS` | unset | Extra flags passed to ComfyUI on start, space-separated. When unset, `--cuda-malloc` is added. |
| `GPU_TYPE` | prompted | `nvidia`, `amd`, `intel`, `apple`, `directml`, `ascend`, `cambricon`, or `cpu`. Drives the PyTorch install. |
| `TORCH_INSTALL_CMD_NVIDIA` | see below | Override for the Nvidia torch install arguments after `uv pip`. |
| `PYTHON_VERSION` | `3.13` | Python version used when creating the venv. |
| `CUSTOM_NODES_AUTHOR` | unset | Default author for scaffolded nodes. |
| `CUSTOM_NODES_PUBID` | unset | Default publisher ID for scaffolded nodes. |
| `COMFY_ASSETS_HOST` | `0.0.0.0` | Asset manager bind host when `--host` is not given. |
| `COMFY_ASSETS_PORT` | `8189` | Asset manager preferred port when `--port` is not given. |

Paths in `.env` and the registry support `{HOME}` and `{USERPROFILE}` placeholders, and `~` expands, so configs stay portable across machines.

## GPU-specific PyTorch

`install` (and dependency updates) install the right torch build for `GPU_TYPE`:

- **nvidia**: `uv pip install --index-url https://download.pytorch.org/whl/nightly/cu130 torch torchvision torchaudio` by default. Override with `TORCH_INSTALL_CMD_NVIDIA` when PyTorch's index moves, rather than waiting for a new Comfy Chair release.
- **amd**: ROCm wheels (Linux).
- **intel**: XPU nightly wheels.
- **directml**: `torch-directml` (Windows).
- **cpu**: CPU-only wheels.
- **apple / ascend / cambricon**: instructions are printed for the manual steps those platforms require.

## venv detection

Only venvs named exactly `venv` or `.venv` inside the ComfyUI directory are recognized. This is a deliberate constraint; custom venv names are not supported. On Windows the venv's `Scripts/` directory is used, elsewhere `bin/`.

## Dry-run

Every command honors the global `--dry-run` / `-n` flag. Destructive steps (deleting directories, killing processes, rewriting the registry, running installs) print what they would do instead of doing it.
