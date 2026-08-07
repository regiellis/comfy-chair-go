---
title: Command Reference
nav_order: 3
---

# Command Reference

```bash
comfy-chair <command> [arguments...]
```

Every command accepts both `snake_case` and `kebab-case` spellings (`create_node` and `create-node` are the same command). Run `comfy-chair --help` for the built-in summary.

## Global flags

| Flag | Short | Meaning |
|------|-------|---------|
| `--dry-run` | `-n` | Preview what a command would do without changing anything. Destructive operations print their plan instead of executing. |
| `--help` | `-h` | Show usage. |

## Lifecycle

These commands operate on one environment. With several environments registered you are prompted to pick one, unless `WORKING_COMFY_ENV` pins the choice.

| Command | Aliases | What it does |
|---------|---------|--------------|
| `start` | `start_fg` | Start ComfyUI in the foreground, streaming output to the terminal. |
| `background` | `start_bg` | Start ComfyUI detached in its own session, logging to `comfyui.log` in the install directory. It survives closing the terminal. |
| `stop` | | Send SIGTERM (kill on Windows) to the recorded PID and remove the PID file. The PID is verified to belong to a ComfyUI process before signalling, so a stale file never kills something unrelated. |
| `restart` | | Stop, wait for the old process to actually exit, then start in the background. |
| `status` | | Report process state, port, environment configuration, and offer stale-PID cleanup. |
| `update` | | Fetch and check out the latest stable ComfyUI tag, then update dependencies with uv. Uncommitted changes are stashed first with your confirmation. |
| `nightly` | | Update to the latest master commit instead of a tag. |
| `downgrade` | | Pick an older release tag and check it out. |
| `install` | | Install a new ComfyUI environment or reconfigure an existing one: clone, venv creation, GPU-specific PyTorch, default custom nodes (Manager, Crystools, rgthree), and registry entry. |
| `reload` | | Watch the opted-in node directories and restart ComfyUI on changes. See [Node Development](node-development). |

Port conflicts are detected before start: if 8188 is taken you are offered the next free port.

## Node development

| Command | What it does |
|---------|--------------|
| `create-node` | Scaffold a new custom node from one of five V3 templates. Prompts for template, name, author, and publisher ID. |
| `list-nodes` | List the custom nodes in the active environment with last-modified times. |
| `delete-node` | Delete a node directory after showing its README and confirming. Honors `--dry-run`. |
| `pack-node` | Zip a node for distribution. A failed pack removes the partial archive. |
| `update-nodes` | For selected or all nodes: `git pull` when the node is a git repo, then install its `requirements.txt` into the venv with uv (falling back to the venv's own `python -m pip`). |
| `watch-nodes` | Choose which node directories trigger reloads. The selection is stored per environment as `reload_include_dirs`. |

## Environments and migration

| Command | What it does |
|---------|--------------|
| `migrate-nodes` | Copy custom nodes from one environment to another and install their requirements there. |
| `migrate-workflows` | Copy workflow JSON files (individually or by folder) between environments. |
| `migrate-images` | Copy input images, video, and audio between environments, including whole folders or a custom relative path. |
| `node-workflows` | Add or remove a node's bundled workflows in the environment's main workflows folder, tracked so removal is exact. |
| `remove-env` | Disconnect an environment from `comfy-installs.json`. Files on disk are untouched; the command prints the manual delete command instead. |
| `sync-env` | Reconcile your `.env` with `.env.example`, adding any missing keys. |

## Other

| Command | What it does |
|---------|--------------|
| `assets` | Start the asset manager web UI. Flags: `--host H` (default `0.0.0.0`), `--port N` (default: first free port from 8189). See [Asset Manager](asset-manager). |
| `empty-trash` | Permanently delete the asset manager's `.trash` contents. Flags: `--env NAME`, `--type output|input|workflows`, `--yes` to skip confirmation. |
| `version` | Print the Comfy Chair version. |
| `help` | Show usage. |

## Exit behavior

CLI commands run and exit; there is no lingering prompt. The interactive TUI (run `comfy-chair` bare) wraps the same commands in a menu and returns to it after each operation.
