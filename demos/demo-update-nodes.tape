Output demos/demo-update-nodes.gif

Set FontSize 16
Set Width 1200
Set Height 900
Set LineHeight 1.3
Set TypingSpeed 100ms
Set Margin 20
Set MarginFill "#2B2D42"
Set BorderRadius 10
Set Theme "one-dark"

Sleep 1s

# Update custom nodes
Type "./comfy-chair update-nodes"
Enter
Sleep 2s

# Select specific nodes to update (multi-select)
Space  # Select ComfyUI-Manager
Sleep 500ms
Down
Down
Space  # Select ComfyUI-AnimateDiff
Sleep 500ms
Down
Space  # Select ComfyUI-ControlNet
Sleep 1s
Enter
Sleep 2s

# Show update progress
Type "# Updating selected nodes..."
Enter
Sleep 1s

# Show git pull for each node
Type "# Updating ComfyUI-Manager..."
Enter
Type "  Already up to date."
Enter
Sleep 1s

Type "# Updating ComfyUI-AnimateDiff..."
Enter
Type "  Pulling latest changes..."
Enter
Type "  3 files changed, 45 insertions(+)"
Enter
Sleep 1s

Type "# Updating ComfyUI-ControlNet..."
Enter
Type "  Pulling latest changes..."
Enter
Type "  1 file changed, 12 insertions(+)"
Enter
Sleep 2s

# Show completion
Type "# Update complete! 2 nodes updated."
Enter
Sleep 2s