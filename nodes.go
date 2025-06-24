package main

import (
	"archive/zip"
	"bufio"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/huh"
	"github.com/regiellis/comfyui-chair-go/internal"
)

//go:embed all:templates/node
var nodeTemplateFS embed.FS

// placeholderRegex for efficient placeholder removal
var placeholderRegex = regexp.MustCompile(`\{\{[^}]*\}\}`)

// Enhanced replacePlaceholders: removes unreplaced vars and ensures quoted strings for Python/JS
func replacePlaceholders(content string, values map[string]string) string {
	// Apply replacements first
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
	// Remove remaining placeholders in one pass using regex
	return placeholderRegex.ReplaceAllString(content, "")
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
	// Check for path traversal sequences
	if strings.Contains(name, "..") || strings.Contains(name, "./") || strings.Contains(name, ".\\") {
		return false
	}
	// Check for absolute paths
	if filepath.IsAbs(name) {
		return false
	}
	// Check for invalid characters (including path separators)
	if strings.ContainsAny(name, " /\\.:*?\"<>|") {
		return false
	}
	// Only allow alphanumeric, underscore, and hyphen
	matched, _ := regexp.MatchString(`^[a-zA-Z0-9_-]+$`, name)
	if !matched {
		return false
	}
	return true
}

// createNewNode prompts the user for node details and scaffolds a new custom node in ComfyUI's custom_nodes directory.
func createNewNode() {
	fmt.Println(internal.TitleStyle.Render("Create New ComfyUI Node (Scaffold)"))
	if !appPaths.IsConfigured {
		fmt.Println(internal.ErrorStyle.Render("ComfyUI path is not configured. Please run 'Install/Reconfigure ComfyUI' first."))
		return
	}

	// Load .env for defaults
	envVars, _ := internal.ReadEnvFile(appPaths.EnvFile)
	authorDefault := envVars["CUSTOM_NODES_AUTHOR"]
	pubidDefault := envVars["CUSTOM_NODES_PUBID"]

	// Prompt for node name, author, pubid
	var nodeName, author, pubid string
	author = authorDefault
	pubid = pubidDefault
	form := huh.NewForm(huh.NewGroup(
		huh.NewInput().Title("Node Name").Value(&nodeName),
		huh.NewInput().Title("Author").Value(&author).Placeholder("Your Name"),
		huh.NewInput().Title("PubID").Value(&pubid).Placeholder("Your PubID"),
	)).WithTheme(huh.ThemeCharm())
	if internal.HandleFormError(form.Run(), "Node creation") {
		return
	}
	if nodeName == "" {
		fmt.Println(internal.InfoStyle.Render("Node creation cancelled (no name provided)."))
		return
	}

	nodeName = sanitizeNodeInput(nodeName)
	if !isValidNodeName(nodeName) {
		fmt.Println(internal.ErrorStyle.Render("Node name is invalid. It must not be empty or contain spaces, slashes, or special characters."))
		return
	}

	customNodesDir := internal.ExpandUserPath(filepath.Join(appPaths.ComfyUIDir, internal.CustomNodesDir))
	nodeDir := internal.ExpandUserPath(filepath.Join(customNodesDir, nodeName))
	
	// Additional security check: ensure nodeDir is within customNodesDir
	cleanCustomNodesDir := filepath.Clean(customNodesDir)
	cleanNodeDir := filepath.Clean(nodeDir)
	if !strings.HasPrefix(cleanNodeDir, cleanCustomNodesDir+string(filepath.Separator)) {
		fmt.Println(internal.ErrorStyle.Render("Invalid node directory path detected. Node creation cancelled for security reasons."))
		return
	}

	values := map[string]string{
		internal.NodeNamePlaceholder:      nodeName,
		internal.NodeNameLowerPlaceholder: strings.ToLower(nodeName),
		internal.NodeDescPlaceholder:      "",
		internal.AuthorPlaceholder:        author,
		"{{License}}":                     "",
		"{{Dependencies}}":                "",
		internal.PubIDPlaceholder:         pubid,
		"{{DisplayName}}":                 "",
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
			fmt.Println(internal.InfoStyle.Render("Node creation cancelled."))
			return
		}
		if err := os.RemoveAll(nodeDir); err != nil {
			fmt.Println(internal.ErrorStyle.Render(fmt.Sprintf("Failed to remove existing node '%s': %v", nodeName, err)))
			return
		}
	}

	if err := copyNodeTemplate(nodeDir, values); err != nil {
		fmt.Println(internal.ErrorStyle.Render(fmt.Sprintf("Failed to scaffold node: %v", err)))
		return
	}
	fmt.Println(internal.SuccessStyle.Render(fmt.Sprintf("Node '%s' created in %s", nodeName, nodeDir)))

	// Auto-generate minimal example workflow JSON for the new node
	exampleDir := filepath.Join(nodeDir, "example_workflows")
	_ = os.MkdirAll(exampleDir, 0755)
	examplePath := filepath.Join(exampleDir, nodeName+"_example.json")
	// Minimal workflow: one node, one output, default input values
	type Workflow struct {
		Name  string           `json:"name"`
		Nodes []map[string]any `json:"nodes"`
		Links [][]any          `json:"links"`
		Extra map[string]any   `json:"extra,omitempty"`
	}
	// Use placeholder/defaults for inputs
	inputs := map[string]any{
		"input_text":     "Hello, Comfy!",
		"input_number":   42,
		"input_bool":     true,
		"input_choice":   "option1",
		"input_optional": "Optional value",
	}
	mainNode := map[string]any{
		"id":      1,
		"type":    nodeName,
		"inputs":  inputs,
		"outputs": map[string]any{"output": 1},
	}
	workflow := Workflow{
		Name:  nodeName + " Example Workflow",
		Nodes: []map[string]any{mainNode},
		Links: [][]any{},
		Extra: map[string]any{"description": "Auto-generated example workflow for node '" + nodeName + "'. Edit as needed."},
	}
	if data, err := json.MarshalIndent(workflow, "", "  "); err == nil {
		_ = os.WriteFile(examplePath, data, 0644)
		fmt.Println(internal.InfoStyle.Render("Example workflow created at: " + examplePath))
	} else {
		fmt.Println(internal.WarningStyle.Render("Failed to create example workflow JSON: " + err.Error()))
	}

	// Update comfy-installs.json (add node to CustomNodes)
	cfg, err := internal.LoadGlobalConfig()
	if err == nil {
		inst := cfg.FindDefaultInstall()
		if inst != nil {
			// Only add if not already present
			if !slices.Contains(inst.CustomNodes, nodeName) {
				inst.CustomNodes = append(inst.CustomNodes, nodeName)
				_ = internal.SaveGlobalConfig(cfg)
			}
		}
	}
}

func listCustomNodes() {
	fmt.Println(internal.TitleStyle.Render("List Custom Nodes"))
	if !appPaths.IsConfigured {
		fmt.Println(internal.ErrorStyle.Render("ComfyUI path is not configured. Please run 'Install/Reconfigure ComfyUI' first."))
		return
	}

	customNodesDir := internal.ExpandUserPath(filepath.Join(appPaths.ComfyUIDir, "custom_nodes"))
	files, err := os.ReadDir(customNodesDir)
	if err != nil {
		fmt.Println(internal.ErrorStyle.Render(fmt.Sprintf("Failed to read custom nodes directory: %v", err)))
		return
	}

	if len(files) == 0 {
		fmt.Println(internal.InfoStyle.Render("No custom nodes found."))
		return
	}

	// Parse active nodes from comfy-installs.json
	activeNodes := map[string]bool{}
	if inst, err := internal.GetActiveComfyInstall(); err == nil && inst != nil {
		for _, d := range inst.ReloadIncludeDirs {
			activeNodes[d] = true
		}
	}

	type nodeRow struct {
		Name    string
		ModTime string
		Active  bool
	}
	var nodeRows []nodeRow
	for _, file := range files {
		if file.IsDir() {
			name := file.Name()
			info, err := os.Stat(filepath.Join(customNodesDir, name))
			modTime := "?"
			if err == nil {
				modTime = info.ModTime().Format("2006-01-02 15:04:05")
			}
			row := nodeRow{
				Name:    name,
				ModTime: modTime,
				Active:  activeNodes[name],
			}
			nodeRows = append(nodeRows, row)
		}
	}

	// Print table header (no URL, no description)
	header := fmt.Sprintf("%-2s %-32s %20s", " ", "Node Name", "Last Modified")
	fmt.Println(internal.InfoStyle.Render(header))
	fmt.Println(strings.Repeat("-", 60))
	for _, row := range nodeRows {
		activeMark := "  "
		name := row.Name
		if row.Active {
			activeMark = internal.SuccessStyle.Render("★ ")
			name = internal.SuccessStyle.Render(name)
		}
		fmt.Printf("%s %-32s %20s\n", activeMark, name, row.ModTime)
	}

	// Prompt user to select a node to view README
	var nodeNames []string
	for _, row := range nodeRows {
		nodeNames = append(nodeNames, row.Name)
	}
	var selected string
	opts := make([]huh.Option[string], len(nodeNames))
	for i, name := range nodeNames {
		opts[i] = huh.NewOption(name, name)
	}
	selectPrompt := huh.NewSelect[string]().
		Title("Select a node to view its README.md (markdown rendered):").
		Options(opts...).
		Value(&selected)
	if err := selectPrompt.Run(); err != nil || selected == "" {
		fmt.Println(internal.InfoStyle.Render("No node selected. Exiting."))
		return
	}
	readmePath := filepath.Join(customNodesDir, selected, "README.md")
	if data, err := os.ReadFile(readmePath); err == nil {
		fmt.Println(internal.InfoStyle.Render(fmt.Sprintf("\nREADME.md for %s:\n", selected)))
		if out, err := glamour.Render(string(data), "dark"); err == nil {
			fmt.Println(out)
		} else {
			fmt.Println(string(data))
		}
	} else {
		fmt.Println(internal.WarningStyle.Render("README.md not found for this node."))
	}
}

func deleteCustomNode() {
	fmt.Println(internal.TitleStyle.Render("Delete Custom Node"))
	if !appPaths.IsConfigured {
		fmt.Println(internal.ErrorStyle.Render("ComfyUI path is not configured. Please run 'Install/Reconfigure ComfyUI' first."))
		return
	}

	customNodesDir := internal.ExpandUserPath(filepath.Join(appPaths.ComfyUIDir, "custom_nodes"))
	files, err := os.ReadDir(customNodesDir)
	if err != nil {
		fmt.Println(internal.ErrorStyle.Render(fmt.Sprintf("Failed to read custom nodes directory: %v", err)))
		return
	}

	if len(files) == 0 {
		fmt.Println(internal.InfoStyle.Render("No custom nodes found."))
		return
	}

	var nodeNames []string
	for _, file := range files {
		if file.IsDir() {
			nodeNames = append(nodeNames, file.Name())
		}
	}

	if len(nodeNames) == 0 {
		fmt.Println(internal.InfoStyle.Render("No custom nodes found."))
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
		fmt.Println(internal.InfoStyle.Render("Node deletion cancelled."))
		return
	}

	// 2. Show node description if README.md exists
	readmePath := filepath.Join(customNodesDir, nodeName, "README.md")
	if data, err := os.ReadFile(readmePath); err == nil {
		fmt.Println(internal.InfoStyle.Render("Node Description:"))
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
		fmt.Println(internal.InfoStyle.Render("Node deletion cancelled."))
		return
	}

	nodeDir := internal.ExpandUserPath(filepath.Join(customNodesDir, nodeName))
	if err := os.RemoveAll(nodeDir); err != nil {
		fmt.Println(internal.ErrorStyle.Render(fmt.Sprintf("Failed to delete node '%s': %v", nodeName, err)))
		return
	}
	fmt.Println(internal.SuccessStyle.Render(fmt.Sprintf("Node '%s' deleted successfully.", nodeName)))

	// Update comfy-installs.json (remove node from CustomNodes)
	cfg, err := internal.LoadGlobalConfig()
	if err == nil {
		inst := cfg.FindDefaultInstall()
		if inst != nil {
			newNodes := make([]string, 0, len(inst.CustomNodes))
			for _, n := range inst.CustomNodes {
				if n != nodeName {
					newNodes = append(newNodes, n)
				}
			}
			inst.CustomNodes = newNodes
			_ = internal.SaveGlobalConfig(cfg)
		}
	}
}

func packNode() {
	fmt.Println(internal.TitleStyle.Render("Pack Custom Node"))
	if !appPaths.IsConfigured {
		fmt.Println(internal.ErrorStyle.Render("ComfyUI path is not configured. Please run 'Install/Reconfigure ComfyUI' first."))
		return
	}

	customNodesDir := internal.ExpandUserPath(filepath.Join(appPaths.ComfyUIDir, "custom_nodes"))
	files, err := os.ReadDir(customNodesDir)
	if err != nil {
		fmt.Println(internal.ErrorStyle.Render(fmt.Sprintf("Failed to read custom nodes directory: %v", err)))
		return
	}

	if len(files) == 0 {
		fmt.Println(internal.InfoStyle.Render("No custom nodes found."))
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
		fmt.Println(internal.InfoStyle.Render("Node packing cancelled."))
		return
	}

	nodeDir := internal.ExpandUserPath(filepath.Join(customNodesDir, nodeName))
	packedFilePath := internal.ExpandUserPath(filepath.Join(customNodesDir, nodeName+".zip"))
	err = zipDirectory(nodeDir, packedFilePath)
	if err != nil {
		fmt.Println(internal.ErrorStyle.Render(fmt.Sprintf("Failed to pack node '%s': %v", nodeName, err)))
		return
	}
	fmt.Println(internal.SuccessStyle.Render(fmt.Sprintf("Node '%s' packed successfully: %s", nodeName, packedFilePath)))
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

	return filepath.Walk(internal.ExpandUserPath(srcDir), func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		relPath, err := filepath.Rel(internal.ExpandUserPath(srcDir), path)
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

// updateCustomNodes updates selected or all custom nodes: git pull and install requirements.txt using uv or pip in the venv.
func updateCustomNodes() {
	fmt.Println(internal.TitleStyle.Render("Update Custom Node(s)"))
	if !appPaths.IsConfigured {
		fmt.Println(internal.ErrorStyle.Render("ComfyUI path is not configured. Please run 'Install/Reconfigure ComfyUI' first."))
		return
	}

	customNodesDir := internal.ExpandUserPath(filepath.Join(appPaths.ComfyUIDir, "custom_nodes"))
	venvPython, err := internal.FindVenvPython(internal.ExpandUserPath(appPaths.ComfyUIDir))
	if err != nil {
		fmt.Println(internal.ErrorStyle.Render(fmt.Sprintf("Python executable not found in 'venv' or '.venv' under %s. Please ensure ComfyUI is installed correctly and the venv is set up (via the 'Install' option).", appPaths.ComfyUIDir)))
		return
	}
	venvPath := filepath.Dir(filepath.Dir(venvPython))
	venvBin := filepath.Join(venvPath, "bin")

	files, err := os.ReadDir(customNodesDir)
	if err != nil {
		fmt.Println(internal.ErrorStyle.Render(fmt.Sprintf("Failed to read custom nodes directory: %v", err)))
		return
	}

	var nodeNames []string
	for _, file := range files {
		if file.IsDir() {
			nodeNames = append(nodeNames, file.Name())
		}
	}

	if len(nodeNames) == 0 {
		fmt.Println(internal.InfoStyle.Render("No custom nodes found."))
		return
	}

	var selected []string
	selectPrompt := huh.NewMultiSelect[string]().
		Title("Select custom nodes to update (space to select, enter to confirm):").
		Options(append([]huh.Option[string]{huh.NewOption("[ALL]", "[ALL]")}, func() []huh.Option[string] {
			opts := make([]huh.Option[string], len(nodeNames))
			for i, name := range nodeNames {
				opts[i] = huh.NewOption(name, name)
			}
			return opts
		}()...)...).
		Value(&selected)
	if err := selectPrompt.Run(); err != nil || len(selected) == 0 {
		fmt.Println(internal.InfoStyle.Render("Node update cancelled."))
		return
	}

	if len(selected) == 1 && selected[0] == "[ALL]" {
		selected = nodeNames
	}

	for _, node := range selected {
		nodeDir := internal.ExpandUserPath(filepath.Join(customNodesDir, node))
		reqFile := internal.ExpandUserPath(filepath.Join(nodeDir, "requirements.txt"))
		if _, err := os.Stat(reqFile); err != nil {
			fmt.Println(internal.WarningStyle.Render(fmt.Sprintf("requirements.txt not found for node '%s', skipping.", node)))
			continue
		}

		fmt.Println(internal.TitleStyle.Render(fmt.Sprintf("Updating %s", node)))

		// 1. git pull if .git exists
		if _, err := os.Stat(filepath.Join(nodeDir, ".git")); err == nil {
			fmt.Println(internal.InfoStyle.Render("Running git pull..."))
			cmdGit := exec.Command("git", "pull")
			cmdGit.Dir = nodeDir
			cmdGit.Stdout = os.Stdout
			cmdGit.Stderr = os.Stderr
			if err := cmdGit.Run(); err != nil {
				fmt.Println(internal.WarningStyle.Render(fmt.Sprintf("git pull failed in %s: %v", nodeDir, err)))
			}
		}

		// 2. Try uv pip install -r requirements.txt (run from ComfyUI root so uv detects venv)
		uvBinPath := filepath.Join(venvBin, "uv")
		uvPath := ""
		if _, err := os.Stat(uvBinPath); err == nil {
			uvPath = uvBinPath
		} else if uvSys, err := exec.LookPath("uv"); err == nil {
			uvPath = uvSys
		}

		pipPath := filepath.Join(venvBin, "pip")
		if _, err := os.Stat(pipPath); err != nil {
			if pipSys, err := exec.LookPath("pip"); err == nil {
				pipPath = pipSys
			}
		}

		var installErr error
		// Run uv from ComfyUI root, pass requirements.txt as relative path
		relReqFile, _ := filepath.Rel(appPaths.ComfyUIDir, reqFile)
		if uvPath != "" {
			// Proactively ensure pip compatibility in uv environment
			if err := internal.EnsurePipCompatibility(venvPath, uvPath); err != nil {
				fmt.Println(internal.WarningStyle.Render("Warning: Could not ensure pip compatibility, proceeding anyway"))
			}
			
			fmt.Println(internal.InfoStyle.Render("Trying uv pip install -r requirements.txt ..."))
			cmdUv := exec.Command(uvPath, "pip", "install", "-r", relReqFile)
			cmdUv.Dir = internal.ExpandUserPath(appPaths.ComfyUIDir)
			cmdUv.Env = append(os.Environ(), "PATH="+venvBin+":"+os.Getenv("PATH"), "VIRTUAL_ENV="+venvPath)
			cmdUv.Stdout = os.Stdout
			cmdUv.Stderr = os.Stderr
			installErr = cmdUv.Run()
			
			if installErr != nil {
				// Attempt to detect and fix pip/uv conflicts
				if fixedErr := internal.DetectAndFixPipUvConflict(installErr, venvPath, uvPath); fixedErr == nil {
					// Retry after fixing
					fmt.Println(internal.InfoStyle.Render("Retrying requirements installation after fixing compatibility..."))
					cmdUvRetry := exec.Command(uvPath, "pip", "install", "-r", relReqFile)
					cmdUvRetry.Dir = internal.ExpandUserPath(appPaths.ComfyUIDir)
					cmdUvRetry.Env = append(os.Environ(), "PATH="+venvBin+":"+os.Getenv("PATH"), "VIRTUAL_ENV="+venvPath)
					cmdUvRetry.Stdout = os.Stdout
					cmdUvRetry.Stderr = os.Stderr
					if retryErr := cmdUvRetry.Run(); retryErr == nil {
						fmt.Println(internal.SuccessStyle.Render("Requirements installed successfully after fixing compatibility"))
						installErr = nil // Mark as successful
					}
				}
			} else {
				fmt.Println(internal.SuccessStyle.Render("uv pip install succeeded."))
			}
		}
		if uvPath == "" || installErr != nil {
			fmt.Println(internal.InfoStyle.Render("Falling back to pip install -r requirements.txt ..."))
			cmdPip := exec.Command(pipPath, "install", "-r", reqFile)
			cmdPip.Dir = nodeDir
			cmdPip.Env = append(os.Environ(), "PATH="+venvBin+":"+os.Getenv("PATH"), "VIRTUAL_ENV="+venvPath)
			cmdPip.Stdout = os.Stdout
			cmdPip.Stderr = os.Stderr
			if err := cmdPip.Run(); err != nil {
				fmt.Println(internal.WarningStyle.Render(fmt.Sprintf("pip install failed in %s: %v", nodeDir, err)))
			} else {
				fmt.Println(internal.SuccessStyle.Render("pip install succeeded."))
			}
		}
	}
}

// addOrRemoveNodeWorkflows allows the user to add or remove workflows associated with a custom node to/from the main workflows folder.
// It keeps track of which workflows were added for each node in a tracking file (e.g., .node_workflows.json).
func addOrRemoveNodeWorkflows() {
	fmt.Println(internal.TitleStyle.Render("Add/Remove Custom Node Workflows in Main Workflows Folder"))
	if !appPaths.IsConfigured {
		fmt.Println(internal.ErrorStyle.Render("ComfyUI path is not configured. Please run 'Install/Reconfigure ComfyUI' first."))
		return
	}

	customNodesDir := internal.ExpandUserPath(filepath.Join(appPaths.ComfyUIDir, "custom_nodes"))
	mainWorkflowsDir := internal.ExpandUserPath(filepath.Join(appPaths.ComfyUIDir, "user", "default", "workflows"))
	trackingFile := internal.ExpandUserPath(filepath.Join(appPaths.ComfyUIDir, ".node_workflows.json"))

	// 1. List custom nodes
	files, err := os.ReadDir(customNodesDir)
	if err != nil {
		fmt.Println(internal.ErrorStyle.Render(fmt.Sprintf("Failed to read custom nodes directory: %v", err)))
		return
	}
	var nodeNames []string
	for _, file := range files {
		if file.IsDir() {
			nodeNames = append(nodeNames, file.Name())
		}
	}
	if len(nodeNames) == 0 {
		fmt.Println(internal.InfoStyle.Render("No custom nodes found."))
		return
	}

	// 2. Select node
	var nodeName string
	options := make([]huh.Option[string], len(nodeNames))
	for i, name := range nodeNames {
		options[i] = huh.NewOption(name, name)
	}
	selectPrompt := huh.NewSelect[string]().
		Title("Select a custom node:").
		Options(options...).
		Value(&nodeName)
	if err := selectPrompt.Run(); err != nil || nodeName == "" {
		fmt.Println(internal.InfoStyle.Render("Operation cancelled."))
		return
	}

	nodeDir := internal.ExpandUserPath(filepath.Join(customNodesDir, nodeName))
	workflowDirs := []string{"example_workflows", "workflows"}
	var workflowFiles []string
	var workflowPaths []string
	for _, dir := range workflowDirs {
		wfDir := filepath.Join(nodeDir, dir)
		files, err := os.ReadDir(wfDir)
		if err != nil {
			continue
		}
		for _, f := range files {
			if !f.IsDir() && strings.HasSuffix(f.Name(), ".json") {
				workflowFiles = append(workflowFiles, f.Name())
				workflowPaths = append(workflowPaths, filepath.Join(wfDir, f.Name()))
			}
		}
	}
	if len(workflowFiles) == 0 {
		fmt.Println(internal.InfoStyle.Render("No workflows found in node's workflow directories."))
		return
	}

	// 3. Add or Remove?
	var action string
	formAction := huh.NewForm(huh.NewGroup(
		huh.NewSelect[string]().
			Title("Would you like to add or remove workflows for this node?").
			Options(
				huh.NewOption("Add to main workflows", "add"),
				huh.NewOption("Remove from main workflows", "remove"),
			).
			Value(&action),
	)).WithTheme(huh.ThemeCharm())
	if err := formAction.Run(); err != nil || action == "" {
		fmt.Println(internal.InfoStyle.Render("Operation cancelled."))
		return
	}

	// 4. Select workflows
	var selected []string
	formWf := huh.NewForm(huh.NewGroup(
		huh.NewMultiSelect[string]().
			Title("Select workflows:").
			OptionsFunc(func() []huh.Option[string] {
				opts := make([]huh.Option[string], len(workflowFiles))
				for i, n := range workflowFiles {
					opts[i] = huh.NewOption(n, n)
				}
				return opts
			}, nil).
			Value(&selected),
	)).WithTheme(huh.ThemeCharm())
	if err := formWf.Run(); err != nil || len(selected) == 0 {
		fmt.Println(internal.InfoStyle.Render("No workflows selected. Operation cancelled."))
		return
	}

	// 5. Load or create tracking file
	type nodeWorkflowMap map[string][]string
	var tracking nodeWorkflowMap
	{
		data, err := os.ReadFile(trackingFile)
		if err == nil {
			_ = json.Unmarshal(data, &tracking)
		} else {
			tracking = make(nodeWorkflowMap)
		}
	}

	// 6. Add or Remove workflows
	if action == "add" {
		added := []string{}
		for i, wf := range workflowFiles {
			for _, sel := range selected {
				if wf == sel {
					src := workflowPaths[i]
					dst := filepath.Join(mainWorkflowsDir, nodeName+"__"+wf)
					input, err := os.ReadFile(src)
					if err != nil {
						fmt.Println(internal.ErrorStyle.Render(fmt.Sprintf("Failed to read %s: %v", src, err)))
						continue
					}
					if err := os.WriteFile(dst, input, 0644); err != nil {
						fmt.Println(internal.ErrorStyle.Render(fmt.Sprintf("Failed to write %s: %v", dst, err)))
						continue
					}
					added = append(added, dst)
				}
			}
		}
		tracking[nodeName] = append(tracking[nodeName], selected...)
		data, _ := json.MarshalIndent(tracking, "", "  ")
		_ = os.WriteFile(trackingFile, data, 0644)
		fmt.Println(internal.SuccessStyle.Render(fmt.Sprintf("Added workflows to main workflows folder: %v", added)))
	} else if action == "remove" {
		removed := []string{}
		for _, wf := range selected {
			filename := nodeName + "__" + wf
			path := filepath.Join(mainWorkflowsDir, filename)
			if err := os.Remove(path); err == nil {
				removed = append(removed, filename)
			}
		}
		// Remove from tracking
		if wfs, ok := tracking[nodeName]; ok {
			newWfs := []string{}
			for _, wf := range wfs {
				if !slices.Contains(selected, wf) {
					newWfs = append(newWfs, wf)
				}
			}
			tracking[nodeName] = newWfs
			data, _ := json.MarshalIndent(tracking, "", "  ")
			_ = os.WriteFile(trackingFile, data, 0644)
		}
		fmt.Println(internal.SuccessStyle.Render(fmt.Sprintf("Removed workflows from main workflows folder: %v", removed)))
	}
}
