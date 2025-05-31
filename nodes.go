package main

import (
	"archive/zip"
	"bufio"
	"embed"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/huh"
	"github.com/regiellis/comfyui-chair-go/internal"
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
	fmt.Println(internal.TitleStyle.Render("Create New ComfyUI Node (Scaffold)"))
	if !appPaths.IsConfigured {
		fmt.Println(internal.ErrorStyle.Render("ComfyUI path is not configured. Please run 'Install/Reconfigure ComfyUI' first."))
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
		fmt.Println(internal.InfoStyle.Render("Node creation cancelled."))
		return
	}

	nodeName = sanitizeNodeInput(nodeName)
	if !isValidNodeName(nodeName) {
		fmt.Println(internal.ErrorStyle.Render("Node name is invalid. It must not be empty or contain spaces, slashes, or special characters."))
		return
	}

	customNodesDir := internal.ExpandUserPath(filepath.Join(appPaths.ComfyUIDir, "custom_nodes"))
	nodeDir := internal.ExpandUserPath(filepath.Join(customNodesDir, nodeName))

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

	// Update comfy-installs.json (add node to CustomNodes)
	cfg, err := internal.LoadGlobalConfig()
	if err == nil {
		inst := cfg.FindDefaultInstall()
		if inst != nil {
			// Only add if not already present
			found := false
			for _, n := range inst.CustomNodes {
				if n == nodeName {
					found = true
					break
				}
			}
			if !found {
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

	// Parse active nodes from .env
	activeNodes := map[string]bool{}
	if envMap, err := internal.ReadEnvFile(internal.ExpandUserPath(appPaths.EnvFile)); err == nil {
		if dirs, ok := envMap["COMFY_RELOAD_INCLUDE_DIRS"]; ok {
			dirs = strings.Trim(dirs, "[]\"")
			for _, d := range strings.Split(dirs, ",") {
				d = strings.TrimSpace(d)
				if d != "" {
					activeNodes[d] = true
				}
			}
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
	selectPrompt := huh.NewSelect[string]().
		Title("Select a node to view its README.md (markdown rendered):").
		Options(func() []huh.Option[string] {
			opts := make([]huh.Option[string], len(nodeNames))
			for i, name := range nodeNames {
				opts[i] = huh.NewOption(name, name)
			}
			return opts
		}()...).
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
			fmt.Println(internal.InfoStyle.Render("Trying uv pip install -r requirements.txt ..."))
			cmdUv := exec.Command(uvPath, "pip", "install", "-r", relReqFile)
			cmdUv.Dir = internal.ExpandUserPath(appPaths.ComfyUIDir)
			cmdUv.Env = append(os.Environ(), "PATH="+venvBin+":"+os.Getenv("PATH"), "VIRTUAL_ENV="+venvPath)
			cmdUv.Stdout = os.Stdout
			cmdUv.Stderr = os.Stderr
			installErr = cmdUv.Run()
			if installErr == nil {
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
