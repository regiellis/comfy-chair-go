---
title: Getting Started
nav_order: 2
---

# Getting Started

## Install

### From a release

Download the binary for your OS and architecture from the [releases page](https://github.com/regiellis/comfy-chair-go/releases), then put it on your PATH:

```bash
chmod +x ./comfy-chair-linux-amd64
sudo mv ./comfy-chair-linux-amd64 /usr/local/bin/comfy-chair
comfy-chair version
```

Binaries are built for Linux, macOS, and Windows (amd64) on every release with CGO disabled, so there are no runtime dependencies beyond the tools listed below.

### From source

```bash
git clone https://github.com/regiellis/comfy-chair-go.git
cd comfy-chair-go
go build -o comfy-chair .    # or: task build
```

Go 1.25 or newer. The `Taskfile` also provides `task build-dev` (debug symbols), `task build-all` (cross-compile into `dist/`), and `task install` (into `$GOBIN`).

## Prerequisites

- **ComfyUI** 0.10.0 or newer if you plan to use the scaffolded V3 nodes
- **git** on PATH, used for install, update, nightly, and downgrade
- **[uv](https://github.com/astral-sh/uv)** on PATH, used for Python dependency management during install and update
- **ffmpeg** (optional), used by the asset manager for video poster frames; icon tiles are shown without it

## First run

Run `comfy-chair` with no arguments to open the interactive TUI menu, or run any command directly. On first run:

1. If no `.env` exists, Comfy Chair prompts you to create one interactively.
2. `COMFYUI_PATH` is the one required variable. Point it at an existing ComfyUI install, or run `comfy-chair install` to clone and set one up from scratch, including venv creation and GPU-specific PyTorch.
3. Installed environments are registered in `comfy-installs.json` with a name like `lounge`, `den`, or `nook`. See [Environments](environments).

State files live in your OS config directory, for example `~/.config/comfy-chair/` on Linux. See [Configuration](configuration) for the full layout.

## A first node

```bash
comfy-chair create-node     # pick a template, name it, done
comfy-chair watch-nodes     # opt the node's directory into reload watching
comfy-chair reload          # start ComfyUI and auto-restart on changes
```

Edit the generated Python under your node's directory. Each save restarts ComfyUI after the debounce window, and the log tails directly into your terminal.
