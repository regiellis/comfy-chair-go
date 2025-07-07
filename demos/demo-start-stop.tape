Output demos/demo-start-stop.gif

Set FontSize 16
Set Width 1200
Set Height 800
Set LineHeight 1.3
Set TypingSpeed 80ms
Set Margin 20
Set MarginFill "#1E1E2E"
Set BorderRadius 10
Set Theme "catppuccin-mocha"

Sleep 1s

# Show status first
Type "./comfy-chair status"
Enter
Sleep 3s

# Start ComfyUI in background
Type "./comfy-chair start-bg"
Enter
Sleep 3s

# Check status again
Type "./comfy-chair status"
Enter
Sleep 3s

# Stop ComfyUI
Type "./comfy-chair stop"
Enter
Sleep 2s

# Verify it stopped
Type "./comfy-chair status"
Enter
Sleep 2s