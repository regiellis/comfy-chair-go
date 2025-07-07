# Comfy Chair Demos

This directory contains demo recordings showcasing various features of the Comfy Chair CLI tool. These demos are created using [VHS](https://github.com/charmbracelet/vhs) (formerly known as `tape`).

## Available Demos

### Installation & Setup
- **demo-install.tape** - Basic ComfyUI installation process
- **demo-environment-setup.tape** - Setting up multiple environments (Lounge/Den/Nook)

### Node Management
- **demo-create-node.tape** - Creating a new custom node from templates
- **demo-node-management.tape** - Advanced node creation and listing
- **demo-update-nodes.tape** - Updating custom nodes with git

### Process Control
- **demo-start-stop.tape** - Starting and stopping ComfyUI
- **demo-reload-watch.tape** - Auto-reload on file changes

### Advanced Features
- **demo-migration.tape** - Migrating nodes and workflows between environments
- **demo-health-diagnostics.tape** - Running health checks and diagnostics
- **demo-performance-monitor.tape** - Real-time performance monitoring

## Generating GIFs/Videos

To generate the demo recordings, you need to install VHS:

```bash
# Install VHS
brew install vhs  # macOS
# or
sudo snap install vhs  # Linux
# or
scoop install vhs  # Windows
```

Then run:

```bash
# Generate a specific demo
vhs < demo-install.tape

# Generate all demos
for tape in *.tape; do
    vhs < "$tape"
done
```

## Creating New Demos

To create a new demo:

1. Copy an existing `.tape` file as a template
2. Modify the commands and timing
3. Set appropriate dimensions and theme
4. Run `vhs < your-demo.tape` to generate the output

### Tape File Structure

```tape
Output demos/my-demo.gif    # Output file
Set FontSize 16             # Font settings
Set Width 1200             
Set Height 800
Set Theme "dracula"         # Color theme

Sleep 1s                    # Initial pause
Type "command"              # Type a command
Enter                       # Press Enter
Sleep 2s                    # Wait 2 seconds
Down                        # Arrow key navigation
Space                       # Space for selection
Ctrl+C                      # Keyboard shortcuts
```

### Available Themes

- catppuccin-mocha
- dracula
- github-dark
- nord
- one-dark
- tokyo-night
- tokyo-night-storm

### Tips

- Use realistic typing speeds (80-100ms)
- Add appropriate sleep times for readability
- Keep demos focused on one feature
- Use clear, descriptive filenames
- Test the generated output before committing