Output demos/demo-migration.gif

Set FontSize 16
Set Width 1200
Set Height 1000
Set LineHeight 1.3
Set TypingSpeed 100ms
Set Margin 20
Set MarginFill "#0D1117"
Set BorderRadius 10
Set Theme "github-dark-dimmed"

Sleep 1s

# Start migration workflow
Type "./comfy-chair migrate-nodes"
Enter
Sleep 2s

# Select source environment (Lounge)
Enter
Sleep 1s

# Select destination environment (Den)
Down
Enter
Sleep 1s

# Select nodes to migrate (multi-select)
Space  # Select first node
Sleep 500ms
Down
Space  # Select second node
Sleep 500ms
Down
Space  # Select third node
Sleep 1s
Enter
Sleep 2s

# Confirm migration
Type "y"
Enter
Sleep 2s

# Show migration progress
Sleep 3s

# Show success
Sleep 2s

# Migrate workflows
Type "./comfy-chair migrate-workflows"
Enter
Sleep 2s

# Select source
Enter
Sleep 1s

# Select destination
Down
Enter
Sleep 2s

# Show workflow migration
Sleep 3s