---
title: Asset Manager
nav_order: 6
---

# Asset Manager

```bash
comfy-chair assets [--host H] [--port N]
```

A local web UI for the files ComfyUI generates and consumes: `output/`, `input/`, and `user/default/workflows/` across **every registered environment** at once. The frontend is server-rendered and swapped with htmx; there is no build step and no external requests.

## Views

- **Grid**: day-grouped bento layout with infinite scroll, folder navigation bar, name and date filters, and an environment/type switcher. Video files get poster-frame thumbnails when ffmpeg is installed, icon tiles otherwise.
- **Detail**: full-size preview with file metadata, and for generated PNGs the embedded prompt and workflow extracted from the file's metadata chunks.
- **Duplicates**: hash-based duplicate review backed by a persistent hash cache, for reclaiming space after heavy generation sessions.

## Actions

- Select assets individually, by shift-range, or select everything matching the current filter; bulk actions can also act server-side on the whole filtered set.
- Copy or move assets between environments and types.
- Rename files and create or manage folders.
- Extract the workflow JSON embedded in a generated PNG to the workflows folder.
- Delete to a `.trash` folder per environment. Empty it from the UI button, from the TUI (*Other Tools*), or non-interactively:

```bash
comfy-chair empty-trash [--env NAME] [--type output|input|workflows] [--yes]
```

## Binding and security

- Default binding is **all interfaces** (`0.0.0.0`) with the first free port from 8189. The CLI prints a warning about this.
- `--host 127.0.0.1` restricts the UI to the local machine. `COMFY_ASSETS_HOST` and `COMFY_ASSETS_PORT` act as fallbacks for the flags.
- There is **no authentication**. Anyone who can reach the port can manage your assets, so keep the default binding to trusted networks.
- Within that model the server is defensive: all file access is validated against the environment roots (including symlink resolution), state-changing requests are same-origin JSON only, served files carry `nosniff` and non-media types download as attachments, and thumbnailing and metadata parsing run with bounded time, concurrency, and memory.

## Caches

Thumbnails are cached under the config directory (`thumb-cache/`), keyed by file identity, so regenerating them is rare. The duplicate view's hash cache prunes entries for files that no longer exist.
