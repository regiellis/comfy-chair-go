# Comfy Chair

![Go Version](https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go&logoColor=white)
![Styled with Lipgloss](https://img.shields.io/badge/UI-Lipgloss-F5A9B8)
![Prompts by Huh](https://img.shields.io/badge/Prompts-Huh-AF87FF)
![ComfyUI Required](https://img.shields.io/badge/ComfyUI-Required-FFD700?logo=python&logoColor=white)

---

>[!TIP]
> You do not need [Go (Golang)](https://go.dev/) installed to use Comfy Chair. The binary is self-contained and does not require any additional dependencies. You can download the latest release from the releases page and run it directly. You will need to have [UV](https://github.com/astral-sh/uv) installed for the install/update commands to work, but this is not a requirement for running Comfy Chair itself.

>[!IMPORTANT]
> I am aware of the open discussions, issues, PRs, and other solutions surrounding ComfyUI and Hot Reloading. This project is not intended to replace or compete with any existing tools or workflows. This was previously a collection of bash scripts I used when I developed my own custom nodes, and has now been unified into a single CLI tool.
>
> **This workflow is _opinionated_ and may not suit everyone;** it was created to suit my own needs. I am open to suggestions and improvements, but please understand that this is a personal project and I offer it as an alternative method to developing on ComfyUI.
>
> If you find it useful, great! It may not be for everyone, and that's perfectly fine. Use it at your own risk and feel free to fork it if you want to make changes or issue PRs.

>[!NOTE]
> For reference, here is my original discussion regarding the ComfyUI hot reloading issue: [#1](https://github.com/comfyanonymous/ComfyUI/discussions/5290)
> For a more in-depth discussion of the topic, see [#2](https://github.com/comfyanonymous/ComfyUI/pull/749) and [#3](https://github.com/comfyanonymous/ComfyUI/pull/796)
> Other projects that may be of interest: [ComfyUI-HotReloadHack](https://github.com/logtd/ComfyUI-HotReloadHack), [ComfyUI-LG_HotReload](https://github.com/LAOGOU-666/ComfyUI-LG_HotReload)

---

## ✨ Why Comfy Chair?

- 🚀 **V3 Node Scaffolding:** Instantly create ComfyUI custom nodes targeting the **V3 node schema** (`comfy_api.latest`: `IO.ComfyNode` subclasses with `define_schema()`, classmethod `execute()`, and a `ComfyExtension` entrypoint). Five templates: basic, advanced, API, model-loader, and web/API.
- 🏘️ **Multiple Environments:** Manage several named ComfyUI installs (`lounge`, `den`, `nook`, or custom) from one tool, with a default environment, per-command environment selection, and `WORKING_COMFY_ENV` pinning.
- 🛠️ **ComfyUI Management:** Start, stop, restart, update (stable, nightly, or downgrade to a tag), and install ComfyUI from the CLI.
- 🔄 **Live Reload:** Watch selected custom node directories and auto-restart ComfyUI on code changes, with a real trailing debounce, symlink support, and log tailing.
- 🗂️ **Asset Manager:** A local web UI for browsing and managing outputs, inputs, and workflows across every registered environment.
- 📦 **Node Packaging & Updates:** Pack nodes into zips; update nodes via `git pull` + requirements install into the correct venv.
- 🚚 **Migration Tools:** Copy custom nodes, workflows, and input images between environments.
- ⚡ **UV-Powered:** Python dependencies are managed with [uv](https://github.com/astral-sh/uv) for fast installs, with GPU-specific PyTorch selection.
- 💻 **Cross-Platform:** Linux, macOS, and Windows binaries built on every release.
- 🛡️ **Safety Rails:** Path traversal protection, command-injection safeguards, atomic config writes, PID identity checks before signalling, and a global `--dry-run` preview for destructive commands.

### Install in Realtime

![Install Demo](demos/demo-install.gif)

## 🚀 Quick Start

### Download a Release

1. Go to the [Releases Page](https://github.com/regiellis/comfy-chair-go/releases) and download the binary for your OS/arch.
2. Make it executable and move it to your PATH:

   ```bash
   chmod +x ./comfy-chair-linux-amd64
   sudo mv ./comfy-chair-linux-amd64 /usr/local/bin/comfy-chair
   comfy-chair help # Verify
   ```

### Build from Source

```bash
git clone https://github.com/regiellis/comfy-chair-go.git
cd comfy-chair-go
go build -o comfy-chair .   # or: task build
./comfy-chair help
```

Run `comfy-chair` with no arguments for the interactive TUI menu; every feature is reachable from there as well as from the CLI commands below.

## 📖 Commands

```bash
comfy-chair <command> [arguments...]
comfy-chair --help              # Full usage with all aliases
comfy-chair --dry-run <command> # Preview what a command will do (short: -n)
```

All commands accept both `snake_case` and `kebab-case` (e.g. `create_node` / `create-node`).

### ComfyUI Lifecycle

| Command    | Aliases              | Description                                          |
|------------|----------------------|------------------------------------------------------|
| start      | start_fg, start-fg   | Start ComfyUI in the foreground                      |
| background | start_bg, start-bg   | Start ComfyUI in the background (detached)           |
| stop       |                      | Stop ComfyUI                                         |
| restart    |                      | Stop, wait for exit, and start again in background   |
| status     |                      | Process + environment status, stale PID cleanup      |
| update     |                      | Update ComfyUI to the latest stable tag              |
| nightly    |                      | Update ComfyUI to the latest master commit           |
| downgrade  |                      | Check out an older ComfyUI release tag               |
| install    |                      | Install or reconfigure a ComfyUI environment         |
| reload     |                      | Watch selected node dirs and auto-restart on changes |

### Node Development

| Command      | Description                                                      |
|--------------|------------------------------------------------------------------|
| create-node  | Scaffold a new custom node from a V3 template                    |
| list-nodes   | List custom nodes with last-modified info                        |
| delete-node  | Delete a custom node (with confirmation + README preview)        |
| pack-node    | Zip a custom node for distribution                               |
| update-nodes | `git pull` + reinstall requirements for selected/all nodes       |
| watch-nodes  | Choose which node directories trigger reloads (opt-in, per env)  |

### Environments & Migration

| Command           | Description                                            |
|-------------------|--------------------------------------------------------|
| migrate-nodes     | Copy custom nodes between environments (+ requirements) |
| migrate-workflows | Copy workflow JSONs between environments               |
| migrate-images    | Copy input images/media between environments           |
| node-workflows    | Add/remove a node's workflows in the main workflows dir |
| remove-env        | Disconnect an environment from the registry (keeps files) |
| sync-env          | Sync `.env` with `.env.example` keys                   |

### Other

| Command     | Description                                                  |
|-------------|--------------------------------------------------------------|
| assets      | Launch the asset manager web UI (`--host`, `--port`)         |
| empty-trash | Empty asset-manager trash (`--env`, `--type`, `--yes`)       |
| version     | Print the Comfy Chair version                                |
| help        | Show usage (also `--help`, `-h`)                             |

## 🏘️ Multi-Environment Model

Comfy Chair manages any number of named ComfyUI installs, registered in `comfy-installs.json`:

- Each environment has a name/type (`lounge` = main/stable, `den` = dev, `nook` = experimental, or custom), a path, its own custom-node list, and reload watch dirs.
- One environment is the **default**; lifecycle commands prompt you to pick when several exist.
- Set `WORKING_COMFY_ENV=<name>` in `.env` (or use *Set Working Environment* in the TUI) to pin the active environment and skip the prompt.
- The migration commands move nodes, workflows, and images between environments.

## 🗂️ Asset Manager

`comfy-chair assets` starts a local web UI that browses `output/`, `input/`, and `user/default/workflows/` across **all** registered environments: bento-grid thumbnails (video poster frames via ffmpeg when present), day grouping with infinite scroll, folder navigation/create/move/rename, date and name filters, bulk selection (shift-range, select-all-matching), hash-based duplicate review, copy/move between environments, delete to `.trash` with an empty-trash button, PNG metadata viewing, and one-click extraction of the workflow embedded in a generated PNG.

- Binding: defaults to **all interfaces** (`0.0.0.0`) and auto-picks a port from 8189. Use `--host 127.0.0.1` to restrict to this machine, `--port N` to pin a port, or the `COMFY_ASSETS_HOST` / `COMFY_ASSETS_PORT` env vars.
- There is **no authentication**. On the default binding anyone on your network can manage your assets, and the CLI warns you about exactly that. Mutating requests are same-origin-only and thumbnails/metadata are generated with bounded resources, but treat the default binding as trusted-LAN only.
- Trash can also be emptied non-interactively: `comfy-chair empty-trash [--env NAME] [--type TYPE] [--yes]`.

## 🧩 Node Templates (V3 Schema)

`create-node` scaffolds from five embedded templates: **basic**, **advanced** (rich UI + utils), **api** (external API integrations), **model** (model loading), and **webapi** (floating panel + HTTP endpoints). All templates target the **ComfyUI V3 node schema** (`IO.ComfyNode` subclasses with `define_schema()`, typed `IO.*` inputs/outputs, classmethod `execute()`, and registration through a `ComfyExtension` with `comfy_entrypoint()`), which requires **ComfyUI >= 0.10.0**. Placeholders (node name, author, publisher ID) are filled from your prompts and `.env` (`CUSTOM_NODES_AUTHOR`, `CUSTOM_NODES_PUBID`), including in file names, and a starter example workflow is generated alongside the node.

## ⚙️ Configuration

State lives in your OS config directory (`~/.config/comfy-chair/` on Linux, the equivalent on macOS/Windows): `.env`, `comfy-installs.json`, and `comfy-performance-history.json`. Legacy binary-adjacent files are migrated there automatically on first use. Pid and log files live inside each ComfyUI install directory.

Copy `.env.example` to `.env` (or let the interactive setup create it) and set:

```env
# Required: path to the active ComfyUI install
COMFYUI_PATH=/path/to/your/ComfyUI
# Pin the active environment by name (optional)
WORKING_COMFY_ENV=lounge
# Comma-separated file extensions that trigger reloads (default: .py,.js,.css)
COMFY_RELOAD_EXTS=.py,.js,.css
# Debounce in seconds for reloads (default: 5)
COMFY_RELOAD_DEBOUNCE=5
# Extra flags passed to ComfyUI on start (space-separated)
COMFY_START_FLAGS=--highvram --cuda-malloc
# GPU type for torch install: nvidia, amd, intel, apple, directml, ascend, cambricon, cpu
GPU_TYPE=nvidia
# Optional override for Nvidia torch install args after `uv pip`
TORCH_INSTALL_CMD_NVIDIA=install --index-url https://download.pytorch.org/whl/nightly/cu130 torch torchvision torchaudio
# Python version for the venv (default: 3.13)
PYTHON_VERSION=3.13
# Defaults for node scaffolding
CUSTOM_NODES_AUTHOR=Your Name
CUSTOM_NODES_PUBID=your-pubid
# Asset manager binding fallbacks
COMFY_ASSETS_HOST=0.0.0.0
COMFY_ASSETS_PORT=8189
```

Paths in config support the portable placeholders `{HOME}` and `{USERPROFILE}`, and `~` is expanded.

### GPU-Specific PyTorch Installation

During install, Comfy Chair prompts for your GPU type and Python version, then installs the matching torch/torchvision/torchaudio in the venv:

- **Nvidia (CUDA)**: stable and nightly
- **AMD (ROCm, Linux)**: stable and nightly
- **Intel (Arc/XPU)**: nightly
- **DirectML (AMD/Windows)**: torch-directml
- **Apple Silicon / Ascend NPU / Cambricon MLU**: manual install, instructions printed
- **CPU only**: CPU wheels

The Nvidia default is `uv pip install --index-url https://download.pytorch.org/whl/nightly/cu130 torch torchvision torchaudio`. If PyTorch's index moves again, override `TORCH_INSTALL_CMD_NVIDIA` in `.env` instead of waiting for a new Comfy Chair build.

## 🔒 venv Detection (By Design)

Only virtual environments named exactly `venv` or `.venv` inside the ComfyUI directory are recognized, on every platform. **Custom venv names are intentionally unsupported.** If you see "Python executable not found in venv", make sure your venv uses one of those two names, or re-run the install command to create one.

## 🔄 Live Reload Notes

- Watching is **opt-in per directory**: run `watch-nodes` (or *Select Watched Nodes for Reload* in the TUI) to pick which node directories trigger restarts. The selection is stored per environment in `comfy-installs.json` as `reload_include_dirs`.
- Symlinked node directories are resolved and watched; directories created while watching are picked up automatically.
- Changes arriving inside the debounce window are coalesced into one restart, never dropped.
- `__pycache__`, `.git`, editor swap files, and anything matched by `.gitignore` are ignored.

## Example Workflow

```bash
comfy-chair install                 # set up an environment (GPU + Python prompts)
comfy-chair create-node             # scaffold a V3 node
comfy-chair watch-nodes             # opt the new node into reload watching
comfy-chair reload                  # develop with auto-restart + log tail
comfy-chair --dry-run delete-node   # preview a destructive command first
comfy-chair pack-node               # zip it for distribution
comfy-chair assets                  # manage generated assets in the browser
```

## 🤝 Contributing

Contributions are welcome! Please fork, branch, and submit a PR. Ensure your code is formatted (`gofmt`) and builds cleanly (`task build`), and that `go vet ./...` and `go test ./...` pass.

## 📜 License

This project is licensed under the GPL-3.0 License - see the [LICENSE](LICENSE) file for details.
