---
title: Troubleshooting
nav_order: 8
---

# Troubleshooting

## "Python executable not found in venv or .venv"

Comfy Chair only recognizes venvs named exactly `venv` or `.venv` inside the ComfyUI directory. Rename yours to match, or run `comfy-chair install` against the same path to create one. Custom venv names are intentionally unsupported.

## Port 8188 is in use

Start commands check the port first and offer the next free one. If you did not expect a conflict, a previous ComfyUI may still be shutting down, or something else owns the port. `comfy-chair status` shows what Comfy Chair believes is running; `restart` waits for the old process to exit before starting the new one.

## Stale PID files

If ComfyUI crashed or the machine rebooted, `comfyui.pid` in the install directory can point at a dead or reused PID. `status` detects this and offers cleanup. `stop` verifies the PID actually belongs to a ComfyUI process before signalling anything, so a reused PID is treated as stale rather than killed.

## Reload never triggers

- Check that the directory is opted in: run `watch-nodes` and confirm the selection. Only directories in `reload_include_dirs` are watched.
- Check the extension list: only files matching `COMFY_RELOAD_EXTS` (default `.py,.js,.css`) count.
- Files ignored by `.gitignore`, `__pycache__`, and editor temp files never trigger.

## Reload restarts feel delayed

Changes arriving within `COMFY_RELOAD_DEBOUNCE` seconds of the last restart are batched into one deferred restart. Lower the debounce in `.env` if it feels sluggish; raise it if rapid saves cause too many restarts.

## Where are the logs?

Background starts write `comfyui.log` in the ComfyUI install directory. `reload` tails it live. Foreground starts stream to the terminal directly.

## The registry says my environments are gone

`comfy-installs.json` in the config directory is written atomically, so truncation from an interrupted write should not happen. If the file is unparseable, commands report the JSON error and refuse to overwrite it; inspect or restore the file rather than re-running install first.

## Scaffolded nodes look like the old V1 API

Templates are embedded in the binary at build time. A `comfy-chair` built before the V3 migration scaffolds V1 nodes forever, no matter what the repo's templates look like now. Check `comfy-chair version` and reinstall the binary from a current release or build.

## Something looks destructive and I want to check first

Add `--dry-run` (or `-n`) to any command. Destructive steps print their plan instead of executing.
