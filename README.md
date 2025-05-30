# Comfy Chair

![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?logo=go&logoColor=white)
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

- 🚀 **Rapid Node Scaffolding:** Instantly create new ComfyUI custom nodes with an opinionated template structure.
- 🛠️ **UV for Fast Python Dependency Management:** Uses uv to manage Python dependencies for ComfyUI and custom nodes resulting in faster installs and updates.
- 🛠️ **ComfyUI Management:** Start, stop, restart, update, and install ComfyUI from the CLI
- 🔄 **Live Reload:** Watch your custom_nodes directory and auto-restart ComfyUI on code changes.
- 📦 **Node Packaging:** Pack nodes for distribution with a single command.
- 🧹 **Clean & List:** Easily list and delete custom nodes.
- 💻 **Cross-Platform:** Works on Linux, macOS, and Windows. Binaries are built for all major OS/arch on every release.
- 🧑‍💻 **Developer Focused:** Built by a developer, for developers. No more manual file copying or error-prone node setup.

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
cd comfy-chair
go build -o comfy-chair .
./comfy-chair help
```

### Taskfile (Local Dev)

```bash
task build   # Build binary
./comfy-chair # Run
```

## 🛠️ Features

- **create-node**: Scaffold a new custom node (with validation, templating, and input sanitization)
- **list-nodes**: List all custom nodes
- **delete-node**: Delete a custom node
- **pack-node**: Pack a custom node into a zip file
- **reload**: Watch for changes in custom_nodes and auto-restart ComfyUI
- **start/background/stop/restart/update/install**: Manage ComfyUI lifecycle
- **Interactive TUI**: Use the CLI with no arguments for a beautiful menu

## Prerequisites

- [Go 1.24](https://go.dev/) (for building from source)
- **ComfyUI** (Python, installed locally)
- **Python 3** (for ComfyUI venv)
- [UV](https://github.com/astral-sh/uv) (for fast Python dependency management; required for install/update commands)
- A `.env` file in the project directory with a `COMFYUI_PATH` variable pointing to your ComfyUI installation, e.g.:

  ```env
  COMFYUI_PATH=/path/to/your/ComfyUI
  ```

  > If a `.env` file is not found, Comfy Chair will prompt you to create one or guide you through the setup interactively.

## 🏁 First Run & .env Setup

Comfy Chair requires a `.env` file in your project directory with a `COMFYUI_PATH` variable pointing to your ComfyUI installation, for example:

```env
COMFYUI_PATH=/path/to/your/ComfyUI
```

If a `.env` file is not found on first run, Comfy Chair will prompt you to create one or guide you through the setup interactively. This ensures your environment is always configured correctly for ComfyUI management and node development.

---

## 🔒 Virtual Environment Support: venv and .venv

Comfy Chair **automatically detects and uses Python virtual environments** named either `venv` or `.venv` in your ComfyUI directory. This is handled by the internal `FindVenvPython` logic, which is robust and cross-platform. **Custom venv directory names are not supported by design**—this ensures maximum reliability and compatibility with future features and tooling.

- If neither `venv` nor `.venv` is found, you will be prompted to (re)install or set up your environment.
- All CLI commands (start, stop, update, status, node management, etc.) use this detection logic.
- If you move or rename your venv, make sure it is named `venv` or `.venv`.

---

## 🛠️ Troubleshooting

- **Missing venv:** If you see errors about missing Python executables, ensure you have a `venv` or `.venv` directory in your ComfyUI folder. Use the CLI's install/reconfigure option to set it up.
- **Custom venv names:** Not supported. Only `venv` and `.venv` are recognized.

---

## 🆕 Recent Improvements & Features

- **Opt-in Node Watching for Reloads:**
  - Use the new `watch_nodes` command or select "Select Watched Nodes for Reload" from the interactive menu to choose which custom node directories should trigger reloads. Only the selected directories are watched; all others are ignored by default.
  - Your selection is saved to `.env` as `COMFY_RELOAD_INCLUDE_DIRS` (comma-separated list).
  - Symlinked directories are supported and will be resolved and watched cross-platform.
- **Command Aliases:** All commands support both `snake_case` and `kebab-case` (e.g., `create_node` and `create-node`).
- **--help Flag & Usage:** Use `--help`, `-h`, or `help` to show a detailed usage guide with all commands and aliases.
- **.env Validation:** On startup, required `.env` variables are checked; missing variables trigger a warning and setup guidance.
- **Configurable Live Reload:**
  - `COMFY_RELOAD_EXTS`: Comma-separated file extensions to watch for reloads (default: `.py,.js,.css`).
  - `COMFY_RELOAD_DEBOUNCE`: Debounce time in seconds for reloads (default: `5`).
- **Node Management Enhancements:**
  - Confirmation prompt before deleting a node.
  - Node description (`README.md`) shown before deletion.
  - Node existence check and overwrite prompt on creation.
  - Improved list output: shows last modified time and highlights active nodes.
  - Pack command prints a success message with the output file.
- **Status Command:** Reports ComfyUI process and environment status, and prompts for stale PID cleanup if needed.
- **Cross-Platform & TUI:** Works on Linux, macOS, and Windows. The interactive TUI is available by running `comfy-chair` with no arguments.
- **Developer Experience:** Improved error messages, input validation, and onboarding for new users.

---

## ⚙️ Configuration (.env Options)

Copy `.env.example` to `.env` and set the following variables:

```env
COMFYUI_PATH=/path/to/your/ComfyUI
# Comma-separated list of file extensions to watch for reloads (default: .py,.js,.css)
COMFY_RELOAD_EXTS=.py,.js,.css
# Debounce time in seconds for reloads (default: 5)
COMFY_RELOAD_DEBOUNCE=5
# Comma-separated list of custom node directories to watch for reloads (opt-in, default: empty)
COMFY_RELOAD_INCLUDE_DIRS=
```

- If `.env` is missing or incomplete, Comfy Chair will prompt you to set it up interactively.

---

## 📖 Usage (Expanded)

```bash
comfy-chair <command> [arguments...]
comfy-chair --help   # Show all commands and aliases
```

### Command Aliases

| Command         | Aliases                | Description                                 |
|-----------------|-----------------------|---------------------------------------------|
| start           | start_fg, start-fg     | Start ComfyUI in foreground                 |
| background      | start_bg, start-bg     | Start ComfyUI in background                 |
| stop            |                       | Stop ComfyUI                                |
| restart         |                       | Restart ComfyUI                             |
| update          |                       | Update ComfyUI                              |
| reload          |                       | Watch for changes and reload ComfyUI        |
| watch_nodes     |                       | Select which custom nodes to watch for reload (opt-in) |
| create_node     | create-node            | Scaffold a new custom node                  |
| list_nodes      | list-nodes             | List all custom nodes                       |
| delete_node     | delete-node            | Delete a custom node                        |
| pack_node       | pack-node              | Pack a custom node into a zip file          |
| install         |                       | Install or reconfigure ComfyUI              |
| status          |                       | Show ComfyUI status and environment         |
| help            | --help, -h             | Show this help message                      |

---

## 📝 Status Command

Run `comfy-chair status` to see:

- ComfyUI process status (running, stopped, or stale PID)
- Environment configuration and .env validation
- Prompt to clean up stale PID files if detected

## Example Workflow

```bash
comfy-chair create-node
comfy-chair list-nodes
comfy-chair reload
comfy-chair pack-node
```

## 🤝 Contributing

Contributions are welcome! Please fork, branch, and submit a PR. Ensure your code is formatted (`go fmt ./...`) and builds cleanly (`task build`).

## 📜 License

This project is licensed under the GPL-3.0 License - see the [LICENSE](LICENSE) file for details.

---
