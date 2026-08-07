---
title: Environments
nav_order: 4
---

# Environments

Comfy Chair manages any number of named ComfyUI installations from one binary. The registry is `comfy-installs.json` in the config directory.

## The model

Each environment records:

- **name / type**: `lounge` (main/stable), `den` (dev), `nook` (experimental), or a custom name
- **path**: the ComfyUI install directory
- **is_default**: one environment can be the default target
- **custom_nodes**: the nodes Comfy Chair has scaffolded or migrated there
- **reload_include_dirs**: which node directories the reload watcher follows

The names are just labels with suggested roles. Nothing stops you from using a single environment, or five.

## Choosing the active environment

Lifecycle and node commands need to know which environment to act on. Resolution order:

1. **`WORKING_COMFY_ENV`** in `.env`, when set and valid, wins without prompting. An unknown value warns and falls through to the picker rather than silently using something else.
2. With **exactly one** registered environment, it is used even if not marked default.
3. With **several** environments, an interactive picker appears, defaulting to the one marked `is_default`.

Set the pin from the TUI (*Set Working Environment*) or by editing `.env`. Setting it also updates `COMFYUI_PATH` to match.

## Moving things between environments

- `migrate-nodes` copies node directories (skipping any venv inside them) and installs each node's `requirements.txt` into the destination venv.
- `migrate-workflows` copies workflow JSONs from `user/default/workflows/`, preserving folder structure. Select individual files, whole folders, or everything.
- `migrate-images` copies media from `input/`, including one level of folders in the picker and a custom relative path option for anything deeper.

The asset manager also copies and moves generated assets between environments; see [Asset Manager](asset-manager).

## Removing an environment

`remove-env` only disconnects the entry from the registry. Comfy Chair never deletes an install directory for you outside the `install` command's explicit replace flow; it prints the platform's manual delete command instead.

## Registry safety

The registry is written atomically (temp file plus rename), so an interrupted write cannot truncate it. If the file is ever corrupted by outside means, commands report the parse error instead of treating your environments as missing or overwriting the file.
