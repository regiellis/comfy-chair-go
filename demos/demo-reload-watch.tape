Output demos/demo-reload-watch.gif

Set FontSize 16
Set Width 1200
Set Height 900
Set LineHeight 1.3
Set TypingSpeed 100ms
Set Margin 20
Set MarginFill "#2E3440"
Set BorderRadius 10
Set Theme "nord"

Sleep 1s

# Start reload watching
Type "./comfy-chair reload"
Enter
Sleep 2s

# Show watching message
Sleep 3s

# Simulate file change detection
Type "# Watching for changes in custom_nodes/"
Sleep 1s
Type@200ms "..."
Sleep 2s

# Show restart message
Type "# Detected change in example_node.py"
Enter
Type "# Restarting ComfyUI..."
Enter
Sleep 3s

# Exit with Ctrl+C
Ctrl+C
Sleep 1s