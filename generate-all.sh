#!/bin/bash

# Generate all demo recordings from tape files
# Requires VHS (https://github.com/charmbracelet/vhs) to be installed

set -e

echo "🎬 Generating Comfy Chair demo recordings..."
echo ""

# Check if vhs is installed
if ! command -v vhs &> /dev/null; then
    echo "❌ Error: VHS is not installed!"
    echo "Please install VHS first:"
    echo "  brew install vhs     # macOS"
    echo "  sudo snap install vhs # Linux"
    echo "  scoop install vhs    # Windows"
    exit 1
fi

# Count tape files
TOTAL=$(ls -1 ./demos/*.tape 2>/dev/null | wc -l)
if [ "$TOTAL" -eq 0 ]; then
    echo "❌ No .tape files found in current directory"
    exit 1
fi

echo "Found $TOTAL tape files to process"
echo ""

# Process each tape file
COUNT=0
for tape in *.tape; do
    COUNT=$((COUNT + 1))
    echo "[$COUNT/$TOTAL] Processing $tape..."
    
    # Extract output filename from tape file
    OUTPUT=$(grep "^Output" "$tape" | head -1 | awk '{print $2}')
    
    if [ -z "$OUTPUT" ]; then
        echo "  ⚠️  Warning: No output specified in $tape, skipping..."
        continue
    fi
    
    # Generate the recording
    if vhs < "$tape"; then
        echo "  ✅ Generated: $OUTPUT"
    else
        echo "  ❌ Failed to generate $OUTPUT"
    fi
    echo ""
done

echo "✨ Demo generation complete!"
echo ""
echo "Generated files:"
ls -la *.gif *.mp4 2>/dev/null || echo "No output files found"