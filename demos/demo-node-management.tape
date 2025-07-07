Output demos/demo-node-management.gif

Set FontSize 16
Set Width 1200
Set Height 1000
Set LineHeight 1.3
Set TypingSpeed 90ms
Set Margin 20
Set MarginFill "#282A36"
Set BorderRadius 10
Set Theme "dracula"

Sleep 1s

# List custom nodes
Type "./comfy-chair list-nodes"
Enter
Sleep 3s

# Create a new node with advanced template
Type "./comfy-chair"
Enter
Sleep 2s

# Navigate to Node Management
Down
Enter
Sleep 1s

# Select Create New Node
Enter
Sleep 1s

# Select Advanced template
Down
Enter
Sleep 1s

# Enter node name
Type "MyAdvancedNode"
Enter
Sleep 1s

# Enter author
Type "Demo User"
Enter
Sleep 1s

# Enter PubID
Type "demo123"
Enter
Sleep 2s

# Show success
Sleep 3s

# List nodes again to show new node
Type "./comfy-chair list-nodes"
Enter
Sleep 3s