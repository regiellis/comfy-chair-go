Output gifs/demo-install.gif
Output mp4/demo-install.mp4

Set FontSize 16
Set Width 1200
Set Height 1000
Set LineHeight 1.3
Set TypingSpeed 100ms
Set Margin 20
Set MarginFill "#FACD15"
Set BorderRadius 10
Set Theme "catppuccin-mocha"


Sleep 1s

# Start the Comfy Chair install process
Type "./comfy-chair install"
Enter
Sleep 2s

# Select environment type (Test)
Down 2
Enter
Sleep 1s

# Enter install path (use default suggestion)
Type "/mnt/Hub/AI/COMFY_DEMO/"
Enter
Sleep 1s

# Confirm installation (if prompted)
Enter
Sleep 2s

# Wait for install to complete
Sleep 10s

# Show success message
Sleep 2s
