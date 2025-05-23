package main

import (
	"archive/zip"
	"bufio"
	"embed"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/charmbracelet/huh"
)

//go:embed all:templates/node
var nodeTemplateFS embed.FS

// Enhanced replacePlaceholders: removes unreplaced vars and ensures quoted strings for Python/JS
func replacePlaceholders(content string, values map[string]string) string {
	for k, v := range values {
		if v != "" {
			if (strings.HasPrefix(k, "{{") && strings.HasSuffix(k, "}}")) && (strings.Contains(content, "\""+k+"\"")) {
				v = strconv.Quote(v)
				content = strings.ReplaceAll(content, "\""+k+"\"", v)
			} else {
				content = strings.ReplaceAll(content, k, v)
			}
		}
	}
	// Remove any unreplaced {{...}} placeholders
	for {
		start := strings.Index(content, "{{")
		if start == -1 {
			break
		}
		end := strings.Index(content[start:], "}}")
		if end == -1 {
			break
		}
		end += start + 2
		// Remove the whole {{...}}
		content = content[:start] + content[end:]
	}
	return content
}

// NewZipWriter returns a new zip.Writer for the given file.
func NewZipWriter(f *os.File) *zip.Writer {
	return zip.NewWriter(f)
}

func copyNodeTemplate(dstDir string, values map[string]string) error {
	templateRoot := "templates/node"
	return fs.WalkDir(nodeTemplateFS, templateRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relPath := strings.TrimPrefix(path, templateRoot+"/")
		if relPath == "" {
			return nil
		}
		dstPath := filepath.Join(dstDir, relPath)
		if d.IsDir() {
			return os.MkdirAll(dstPath, 0755)
		}
		// For files with placeholders in the name
		for k, v := range values {
			if strings.Contains(dstPath, k) {
				dstPath = strings.ReplaceAll(dstPath, k, v)
			}
		}
		data, err := nodeTemplateFS.ReadFile(path)
		if err != nil {
			return err
		}
		content := replacePlaceholders(string(data), values)
		return os.WriteFile(dstPath, []byte(content), 0644)
	})
}

// input sanitization and validation for node creation
func sanitizeNodeInput(input string) string {
	return strings.TrimSpace(strings.ReplaceAll(input, " ", "_"))
}

func isValidNodeName(name string) bool {
	if name == "" {
		return false
	}
	if strings.ContainsAny(name, " /\\.:") {
		return false
	}
	return true
}

// createNewNode prompts the user for node details and scaffolds a new custom node in ComfyUI's custom_nodes directory.
func createNewNode() {
	fmt.Println(titleStyle.Render("Create New ComfyUI Node (Scaffold)"))
	if !appPaths.isConfigured {
		fmt.Println(errorStyle.Render("ComfyUI path is not configured. Please run 'Install/Reconfigure ComfyUI' first."))
		return
	}

	// Prompt for node details
	var nodeName, nodeDesc, author, license, deps, publisherId, displayName string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().Title("Node Name (no spaces, e.g. MyNode)").Value(&nodeName),
			huh.NewInput().Title("Short Description").Value(&nodeDesc),
			huh.NewInput().Title("Author").Value(&author),
			huh.NewInput().Title("License (e.g. MIT, Apache-2.0)").Value(&license),
			huh.NewInput().Title("Python Dependencies (comma-separated, optional)").Value(&deps),
			huh.NewInput().Title("PublisherId (for ComfyUI plugin)").Value(&publisherId),
			huh.NewInput().Title("DisplayName (for ComfyUI plugin)").Value(&displayName),
		),
	).WithTheme(huh.ThemeCharm())
	if err := form.Run(); err != nil {
		fmt.Println(infoStyle.Render("Node creation cancelled."))
		return
	}

	nodeName = sanitizeNodeInput(nodeName)
	if !isValidNodeName(nodeName) {
		fmt.Println(errorStyle.Render("Node name is invalid. It must not be empty or contain spaces, slashes, or special characters."))
		return
	}

	customNodesDir := filepath.Join(appPaths.comfyUIDir, "custom_nodes")
	nodeDir := filepath.Join(customNodesDir, nodeName)

	values := map[string]string{
		"{{NodeName}}":      nodeName,
		"{{NodeNameLower}}": strings.ToLower(nodeName),
		"{{NodeDesc}}":      nodeDesc,
		"{{Author}}":        author,
		"{{License}}":       license,
		"{{Dependencies}}":  strings.ReplaceAll(deps, ",", "\n"),
		"{{PublisherId}}":   publisherId,
		"{{DisplayName}}":   displayName,
		"{{Version}}":       "1.0.0",
	}

	// 1. Node existence check before creation
	if _, err := os.Stat(nodeDir); err == nil {
		var confirm string
		fmt.Printf("A node named '%s' already exists. Overwrite? [y/N]: ", nodeName)
		scan := bufio.NewScanner(os.Stdin)
		if scan.Scan() {
			confirm = strings.TrimSpace(strings.ToLower(scan.Text()))
		}
		if confirm != "y" && confirm != "yes" {
			fmt.Println(infoStyle.Render("Node creation cancelled."))
			return
		}
		if err := os.RemoveAll(nodeDir); err != nil {
			fmt.Println(errorStyle.Render(fmt.Sprintf("Failed to remove existing node '%s': %v", nodeName, err)))
			return
		}
	}

	if err := copyNodeTemplate(nodeDir, values); err != nil {
		fmt.Println(errorStyle.Render(fmt.Sprintf("Failed to scaffold node: %v", err)))
		return
	}
	fmt.Println(successStyle.Render(fmt.Sprintf("Node '%s' created in %s", nodeName, nodeDir)))
}

func listCustomNodes() {
	fmt.Println(titleStyle.Render("List Custom Nodes"))
	if !appPaths.isConfigured {
		fmt.Println(errorStyle.Render("ComfyUI path is not configured. Please run 'Install/Reconfigure ComfyUI' first."))
		return
	}

	customNodesDir := filepath.Join(appPaths.comfyUIDir, "custom_nodes")
	files, err := os.ReadDir(customNodesDir)
	if err != nil {
		fmt.Println(errorStyle.Render(fmt.Sprintf("Failed to read custom nodes directory: %v", err)))
		return
	}

	if len(files) == 0 {
		fmt.Println(infoStyle.Render("No custom nodes found."))
		return
	}

	fmt.Println("Custom Nodes:")
	for _, file := range files {
		if file.IsDir() {
			info, err := os.Stat(filepath.Join(customNodesDir, file.Name()))
			if err == nil {
				modTime := info.ModTime().Format("2006-01-02 15:04:05")
				fmt.Printf("- %s (Last modified: %s)", file.Name(), modTime)
				readmePath := filepath.Join(customNodesDir, file.Name(), "README.md")
				if data, err := os.ReadFile(readmePath); err == nil {
					desc := strings.SplitN(string(data), "\n", 2)[0]
					if len(desc) > 0 {
						fmt.Printf(" — %s", strings.TrimSpace(desc))
					}
				}
				fmt.Println()
			} else {
				fmt.Println("- " + file.Name())
			}
		}
	}
}

func deleteCustomNode() {
	fmt.Println(titleStyle.Render("Delete Custom Node"))
	if !appPaths.isConfigured {
		fmt.Println(errorStyle.Render("ComfyUI path is not configured. Please run 'Install/Reconfigure ComfyUI' first."))
		return
	}

	customNodesDir := filepath.Join(appPaths.comfyUIDir, "custom_nodes")
	files, err := os.ReadDir(customNodesDir)
	if err != nil {
		fmt.Println(errorStyle.Render(fmt.Sprintf("Failed to read custom nodes directory: %v", err)))
		return
	}

	if len(files) == 0 {
		fmt.Println(infoStyle.Render("No custom nodes found."))
		return
	}

	var nodeNames []string
	for _, file := range files {
		if file.IsDir() {
			nodeNames = append(nodeNames, file.Name())
		}
	}

	if len(nodeNames) == 0 {
		fmt.Println(infoStyle.Render("No custom nodes found."))
		return
	}

	// 1. Confirmation Before Deletion
	var nodeName string
	options := make([]huh.Option[string], len(nodeNames))
	for i, name := range nodeNames {
		options[i] = huh.NewOption(name, name)
	}
	selectPrompt := huh.NewSelect[string]().
		Title("Select a custom node to delete:").
		Options(options...).
		Value(&nodeName)
	if err := selectPrompt.Run(); err != nil || nodeName == "" {
		fmt.Println(infoStyle.Render("Node deletion cancelled."))
		return
	}

	// 2. Show node description if README.md exists
	readmePath := filepath.Join(customNodesDir, nodeName, "README.md")
	if data, err := os.ReadFile(readmePath); err == nil {
		fmt.Println(infoStyle.Render("Node Description:"))
		fmt.Println(strings.TrimSpace(string(data)))
	}

	// 3. Confirm deletion
	var confirm string
	fmt.Printf("Are you sure you want to delete node '%s'? [y/N]: ", nodeName)
	scan := bufio.NewScanner(os.Stdin)
	if scan.Scan() {
		confirm = strings.TrimSpace(strings.ToLower(scan.Text()))
	}
	if confirm != "y" && confirm != "yes" {
		fmt.Println(infoStyle.Render("Node deletion cancelled."))
		return
	}

	nodeDir := filepath.Join(customNodesDir, nodeName)
	if err := os.RemoveAll(nodeDir); err != nil {
		fmt.Println(errorStyle.Render(fmt.Sprintf("Failed to delete node '%s': %v", nodeName, err)))
		return
	}
	fmt.Println(successStyle.Render(fmt.Sprintf("Node '%s' deleted successfully.", nodeName)))
}

func packNode() {
	fmt.Println(titleStyle.Render("Pack Custom Node"))
	if !appPaths.isConfigured {
		fmt.Println(errorStyle.Render("ComfyUI path is not configured. Please run 'Install/Reconfigure ComfyUI' first."))
		return
	}

	customNodesDir := filepath.Join(appPaths.comfyUIDir, "custom_nodes")
	files, err := os.ReadDir(customNodesDir)
	if err != nil {
		fmt.Println(errorStyle.Render(fmt.Sprintf("Failed to read custom nodes directory: %v", err)))
		return
	}

	if len(files) == 0 {
		fmt.Println(infoStyle.Render("No custom nodes found."))
		return
	}

	var nodeNames []string
	for _, file := range files {
		if file.IsDir() {
			nodeNames = append(nodeNames, file.Name())
		}
	}

	var nodeName string
	options := make([]huh.Option[string], len(nodeNames))
	for i, name := range nodeNames {
		options[i] = huh.NewOption(name, name)
	}
	selectPrompt := huh.NewSelect[string]().
		Title("Select a custom node to pack:").
		Options(options...).
		Value(&nodeName)
	if err := selectPrompt.Run(); err != nil || nodeName == "" {
		fmt.Println(infoStyle.Render("Node packing cancelled."))
		return
	}

	nodeDir := filepath.Join(customNodesDir, nodeName)
	packedFilePath := filepath.Join(customNodesDir, nodeName+".zip")
	err = zipDirectory(nodeDir, packedFilePath)
	if err != nil {
		fmt.Println(errorStyle.Render(fmt.Sprintf("Failed to pack node '%s': %v", nodeName, err)))
		return
	}
	fmt.Println(successStyle.Render(fmt.Sprintf("Node '%s' packed successfully: %s", nodeName, packedFilePath)))
}

// zipDirectory zips the contents of srcDir into destZip.
func zipDirectory(srcDir, destZip string) error {
	zipFile, err := os.Create(destZip)
	if err != nil {
		return err
	}
	defer zipFile.Close()

	zipWriter := NewZipWriter(zipFile)
	defer zipWriter.Close()

	return filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		relPath, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		if info.IsDir() {
			if relPath == "." {
				return nil
			}
			_, err := zipWriter.Create(relPath + "/")
			return err
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

		w, err := zipWriter.Create(relPath)
		if err != nil {
			return err
		}
		_, err = io.Copy(w, file)
		return err
	})
}
