---
title: Home
nav_order: 1
---

# Comfy Chair

A single-binary CLI for managing ComfyUI installations and developing custom nodes. It handles lifecycle control (start, stop, restart, update, install), scaffolds V3-schema custom nodes from templates, live-reloads ComfyUI when your node code changes, packages nodes for distribution, and ships a web UI for managing generated assets across every environment you register.

Comfy Chair shells out to `git`, `uv`, and your ComfyUI venv's Python. It does not embed or replace ComfyUI.

## Where to start

- [Getting Started](getting-started): install the binary and run first-time setup
- [Command Reference](commands): every command, alias, and flag
- [Environments](environments): manage several ComfyUI installs from one tool
- [Node Development](node-development): scaffolding, templates, live reload, packaging
- [Asset Manager](asset-manager): the web UI for outputs, inputs, and workflows
- [Configuration](configuration): state files and every `.env` variable
- [Troubleshooting](troubleshooting): the common failure modes and what they mean

## The short version

```bash
comfy-chair install       # set up a ComfyUI environment
comfy-chair create-node   # scaffold a V3 custom node
comfy-chair reload        # develop with auto-restart
comfy-chair assets        # manage generated assets in the browser
comfy-chair               # or just run it bare for the TUI menu
```

This project is opinionated and personal. It grew out of the bash scripts I used while developing my own custom nodes, and I publish it as an alternative workflow rather than a replacement for anything. Use it at your own risk, and fork freely.
