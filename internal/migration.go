package internal

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/huh"
)

// MigrationEntry represents a file or directory that can be migrated
type MigrationEntry struct {
	Display string // e.g. "folder/file.jpg" or "folder/"
	Path    string // relative to source directory
	IsDir   bool
}

// selectEnvironments prompts the user to select source and target environments for migration
func selectEnvironments(cfg *GlobalConfig, operationType string) (*ComfyInstall, *ComfyInstall, error) {
	if len(cfg.Installs) < 2 {
		return nil, nil, fmt.Errorf("at least 2 environments are required for migration. Currently have %d", len(cfg.Installs))
	}

	// Create environment options
	envOptions := make([]huh.Option[string], len(cfg.Installs))
	for i, install := range cfg.Installs {
		envOptions[i] = huh.NewOption(fmt.Sprintf("%s (%s)", install.Type, install.Path), string(install.Type))
	}

	var sourceEnv, targetEnv string
	form := huh.NewForm(huh.NewGroup(
		huh.NewSelect[string]().
			Title(fmt.Sprintf("Select source environment to %s FROM:", operationType)).
			Options(envOptions...).
			Value(&sourceEnv),
		huh.NewSelect[string]().
			Title(fmt.Sprintf("Select target environment to %s TO:", operationType)).
			Options(envOptions...).
			Value(&targetEnv),
	)).WithTheme(huh.ThemeCharm())

	if err := form.Run(); err != nil {
		return nil, nil, fmt.Errorf("failed to get environment selection: %w", err)
	}

	if sourceEnv == "" || targetEnv == "" {
		return nil, nil, fmt.Errorf("both source and target environments must be selected")
	}

	if sourceEnv == targetEnv {
		return nil, nil, fmt.Errorf("source and target environments cannot be the same")
	}

	sourceInstall := cfg.FindInstallByType(InstallType(sourceEnv))
	targetInstall := cfg.FindInstallByType(InstallType(targetEnv))

	if sourceInstall == nil {
		return nil, nil, fmt.Errorf("source environment '%s' not found", sourceEnv)
	}
	if targetInstall == nil {
		return nil, nil, fmt.Errorf("target environment '%s' not found", targetEnv)
	}

	return sourceInstall, targetInstall, nil
}

// displayMigrationSummary shows the results of a migration operation
func displayMigrationSummary(title string, results []string) {
	fmt.Println(TitleStyle.Render(title))
	if len(results) == 0 {
		fmt.Println(InfoStyle.Render("No items were migrated."))
		return
	}

	for _, result := range results {
		fmt.Println(result)
	}
}

// isMediaFile checks if a file has a media extension
func isMediaFile(filename string, extensions []string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	for _, validExt := range extensions {
		if ext == strings.ToLower(validExt) {
			return true
		}
	}
	return false
}

// copyFile copies a single file from src to dst
func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	if _, err := io.Copy(destFile, sourceFile); err != nil {
		return err
	}

	// Copy permissions
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	return os.Chmod(dst, srcInfo.Mode())
}

// MigrateInputImages migrates input images/videos/audio between environments
func MigrateInputImages() {
	fmt.Println(TitleStyle.Render("Migrate Input Images/Videos/Audio"))

	cfg, err := LoadGlobalConfig()
	if err != nil {
		fmt.Println(ErrorStyle.Render(fmt.Sprintf("Failed to load configuration: %v", err)))
		PromptReturnToMenu()
		return
	}

	sourceInstall, targetInstall, err := selectEnvironments(cfg, "migrate input media")
	if err != nil {
		fmt.Println(ErrorStyle.Render(err.Error()))
		PromptReturnToMenu()
		return
	}

	// Define media extensions
	mediaExtensions := []string{".jpg", ".jpeg", ".png", ".gif", ".bmp", ".tiff", ".webp", ".mp4", ".avi", ".mov", ".mkv", ".wmv", ".mp3", ".wav", ".flac", ".ogg"}

	// Scan source input directory
	sourceInputDir := ExpandUserPath(filepath.Join(sourceInstall.Path, "input"))
	if _, err := os.Stat(sourceInputDir); os.IsNotExist(err) {
		fmt.Println(ErrorStyle.Render(fmt.Sprintf("Source input directory does not exist: %s", sourceInputDir)))
		PromptReturnToMenu()
		return
	}

	var entries []MigrationEntry
	err = filepath.Walk(sourceInputDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors, continue walking
		}

		relPath, err := filepath.Rel(sourceInputDir, path)
		if err != nil {
			return nil
		}

		if relPath == "." {
			return nil // Skip root directory
		}

		if info.IsDir() {
			entries = append(entries, MigrationEntry{
				Display: relPath + "/",
				Path:    relPath,
				IsDir:   true,
			})
		} else if isMediaFile(info.Name(), mediaExtensions) {
			entries = append(entries, MigrationEntry{
				Display: relPath,
				Path:    relPath,
				IsDir:   false,
			})
		}

		return nil
	})

	if err != nil {
		fmt.Println(ErrorStyle.Render(fmt.Sprintf("Failed to scan source directory: %v", err)))
		PromptReturnToMenu()
		return
	}

	if len(entries) == 0 {
		fmt.Println(InfoStyle.Render("No media files found in source input directory."))
		PromptReturnToMenu()
		return
	}

	// Create selection options
	options := []huh.Option[string]{huh.NewOption("[ALL] - Migrate all media files", "[ALL]")}
	for _, entry := range entries {
		options = append(options, huh.NewOption(entry.Display, entry.Path))
	}
	options = append(options, huh.NewOption("[CUSTOM] - Enter custom path", "[CUSTOM]"))

	var selected []string
	selectForm := huh.NewForm(huh.NewGroup(
		huh.NewMultiSelect[string]().
			Title("Select media files/folders to migrate:").
			Options(options...).
			Value(&selected),
	)).WithTheme(huh.ThemeCharm())

	if err := selectForm.Run(); err != nil || len(selected) == 0 {
		fmt.Println(InfoStyle.Render("Migration cancelled."))
		PromptReturnToMenu()
		return
	}

	// Handle custom path option
	if len(selected) == 1 && selected[0] == "[CUSTOM]" {
		var customPath string
		customForm := huh.NewForm(huh.NewGroup(
			huh.NewInput().
				Title("Enter relative path from input directory (e.g., 'subfolder/image.jpg'):").
				Value(&customPath),
		)).WithTheme(huh.ThemeCharm())

		if err := customForm.Run(); err != nil || strings.TrimSpace(customPath) == "" {
			fmt.Println(InfoStyle.Render("Migration cancelled."))
			PromptReturnToMenu()
			return
		}

		selected = []string{strings.TrimSpace(customPath)}
	}

	// Handle [ALL] selection
	if len(selected) == 1 && selected[0] == "[ALL]" {
		selected = make([]string, len(entries))
		for i, entry := range entries {
			selected[i] = entry.Path
		}
	}

	// Perform migration
	targetInputDir := ExpandUserPath(filepath.Join(targetInstall.Path, "input"))
	if err := os.MkdirAll(targetInputDir, 0755); err != nil {
		fmt.Println(ErrorStyle.Render(fmt.Sprintf("Failed to create target input directory: %v", err)))
		PromptReturnToMenu()
		return
	}

	var results []string
	for _, selectedPath := range selected {
		if selectedPath == "[ALL]" || selectedPath == "[CUSTOM]" {
			continue // Skip special options
		}

		srcPath := filepath.Join(sourceInputDir, selectedPath)
		dstPath := filepath.Join(targetInputDir, selectedPath)

		// Ensure destination directory exists
		if err := os.MkdirAll(filepath.Dir(dstPath), 0755); err != nil {
			results = append(results, ErrorStyle.Render(fmt.Sprintf("✗ Failed to create directory for %s: %v", selectedPath, err)))
			continue
		}

		// Check if source exists
		srcInfo, err := os.Stat(srcPath)
		if err != nil {
			results = append(results, ErrorStyle.Render(fmt.Sprintf("✗ Source not found: %s", selectedPath)))
			continue
		}

		// Copy file or directory
		if srcInfo.IsDir() {
			if err := copyDir(srcPath, dstPath); err != nil {
				results = append(results, ErrorStyle.Render(fmt.Sprintf("✗ Failed to copy directory %s: %v", selectedPath, err)))
			} else {
				results = append(results, SuccessStyle.Render(fmt.Sprintf("✓ Copied directory: %s", selectedPath)))
			}
		} else {
			if err := copyFile(srcPath, dstPath); err != nil {
				results = append(results, ErrorStyle.Render(fmt.Sprintf("✗ Failed to copy file %s: %v", selectedPath, err)))
			} else {
				results = append(results, SuccessStyle.Render(fmt.Sprintf("✓ Copied file: %s", selectedPath)))
			}
		}
	}

	displayMigrationSummary("Migration Results", results)
	PromptReturnToMenu()
}

// MigrateWorkflows migrates workflows between environments
func MigrateWorkflows() {
	fmt.Println(TitleStyle.Render("Migrate Workflows"))

	cfg, err := LoadGlobalConfig()
	if err != nil {
		fmt.Println(ErrorStyle.Render(fmt.Sprintf("Failed to load configuration: %v", err)))
		PromptReturnToMenu()
		return
	}

	sourceInstall, targetInstall, err := selectEnvironments(cfg, "migrate workflows")
	if err != nil {
		fmt.Println(ErrorStyle.Render(err.Error()))
		PromptReturnToMenu()
		return
	}

	// Scan source workflows directory
	sourceWorkflowsDir := ExpandUserPath(filepath.Join(sourceInstall.Path, "user", "default", "workflows"))
	if _, err := os.Stat(sourceWorkflowsDir); os.IsNotExist(err) {
		fmt.Println(ErrorStyle.Render(fmt.Sprintf("Source workflows directory does not exist: %s", sourceWorkflowsDir)))
		PromptReturnToMenu()
		return
	}

	entries, err := os.ReadDir(sourceWorkflowsDir)
	if err != nil {
		fmt.Println(ErrorStyle.Render(fmt.Sprintf("Failed to read source workflows directory: %v", err)))
		PromptReturnToMenu()
		return
	}

	var workflows []MigrationEntry
	for _, entry := range entries {
		if strings.HasSuffix(strings.ToLower(entry.Name()), ".json") {
			workflows = append(workflows, MigrationEntry{
				Display: entry.Name(),
				Path:    entry.Name(),
				IsDir:   false,
			})
		}
	}

	if len(workflows) == 0 {
		fmt.Println(InfoStyle.Render("No workflow files found in source directory."))
		PromptReturnToMenu()
		return
	}

	// Create selection options
	options := []huh.Option[string]{huh.NewOption("[ALL] - Migrate all workflows", "[ALL]")}
	for _, workflow := range workflows {
		options = append(options, huh.NewOption(workflow.Display, workflow.Path))
	}

	var selected []string
	selectForm := huh.NewForm(huh.NewGroup(
		huh.NewMultiSelect[string]().
			Title("Select workflows to migrate:").
			Options(options...).
			Value(&selected),
	)).WithTheme(huh.ThemeCharm())

	if err := selectForm.Run(); err != nil || len(selected) == 0 {
		fmt.Println(InfoStyle.Render("Migration cancelled."))
		PromptReturnToMenu()
		return
	}

	// Handle [ALL] selection
	if len(selected) == 1 && selected[0] == "[ALL]" {
		selected = make([]string, len(workflows))
		for i, workflow := range workflows {
			selected[i] = workflow.Path
		}
	}

	// Perform migration
	targetWorkflowsDir := ExpandUserPath(filepath.Join(targetInstall.Path, "user", "default", "workflows"))
	if err := os.MkdirAll(targetWorkflowsDir, 0755); err != nil {
		fmt.Println(ErrorStyle.Render(fmt.Sprintf("Failed to create target workflows directory: %v", err)))
		PromptReturnToMenu()
		return
	}

	var results []string
	for _, selectedPath := range selected {
		if selectedPath == "[ALL]" {
			continue // Skip special option
		}

		srcPath := filepath.Join(sourceWorkflowsDir, selectedPath)
		dstPath := filepath.Join(targetWorkflowsDir, selectedPath)

		if err := copyFile(srcPath, dstPath); err != nil {
			results = append(results, ErrorStyle.Render(fmt.Sprintf("✗ Failed to copy %s: %v", selectedPath, err)))
		} else {
			results = append(results, SuccessStyle.Render(fmt.Sprintf("✓ Copied workflow: %s", selectedPath)))
		}
	}

	displayMigrationSummary("Migration Results", results)
	PromptReturnToMenu()
}

// MigrateCustomNodes migrates custom nodes between environments
func MigrateCustomNodes() {
	fmt.Println(TitleStyle.Render("Migrate Custom Nodes"))

	cfg, err := LoadGlobalConfig()
	if err != nil {
		fmt.Println(ErrorStyle.Render(fmt.Sprintf("Failed to load configuration: %v", err)))
		PromptReturnToMenu()
		return
	}

	sourceInstall, targetInstall, err := selectEnvironments(cfg, "migrate custom nodes")
	if err != nil {
		fmt.Println(ErrorStyle.Render(err.Error()))
		PromptReturnToMenu()
		return
	}

	// Scan source custom_nodes directory
	sourceNodesDir := ExpandUserPath(filepath.Join(sourceInstall.Path, "custom_nodes"))
	if _, err := os.Stat(sourceNodesDir); os.IsNotExist(err) {
		fmt.Println(ErrorStyle.Render(fmt.Sprintf("Source custom_nodes directory does not exist: %s", sourceNodesDir)))
		PromptReturnToMenu()
		return
	}

	entries, err := os.ReadDir(sourceNodesDir)
	if err != nil {
		fmt.Println(ErrorStyle.Render(fmt.Sprintf("Failed to read source custom_nodes directory: %v", err)))
		PromptReturnToMenu()
		return
	}

	var nodes []MigrationEntry
	for _, entry := range entries {
		if entry.IsDir() {
			nodes = append(nodes, MigrationEntry{
				Display: entry.Name() + "/",
				Path:    entry.Name(),
				IsDir:   true,
			})
		}
	}

	if len(nodes) == 0 {
		fmt.Println(InfoStyle.Render("No custom nodes found in source directory."))
		PromptReturnToMenu()
		return
	}

	// Create selection options
	options := []huh.Option[string]{huh.NewOption("[ALL] - Migrate all custom nodes", "[ALL]")}
	for _, node := range nodes {
		options = append(options, huh.NewOption(node.Display, node.Path))
	}

	var selected []string
	selectForm := huh.NewForm(huh.NewGroup(
		huh.NewMultiSelect[string]().
			Title("Select custom nodes to migrate:").
			Options(options...).
			Value(&selected),
	)).WithTheme(huh.ThemeCharm())

	if err := selectForm.Run(); err != nil || len(selected) == 0 {
		fmt.Println(InfoStyle.Render("Migration cancelled."))
		PromptReturnToMenu()
		return
	}

	// Handle [ALL] selection
	if len(selected) == 1 && selected[0] == "[ALL]" {
		selected = make([]string, len(nodes))
		for i, node := range nodes {
			selected[i] = node.Path
		}
	}

	// Use existing CopyAndInstallCustomNodes function
	targetNodesDir := ExpandUserPath(filepath.Join(targetInstall.Path, "custom_nodes"))
	if err := os.MkdirAll(targetNodesDir, 0755); err != nil {
		fmt.Println(ErrorStyle.Render(fmt.Sprintf("Failed to create target custom_nodes directory: %v", err)))
		PromptReturnToMenu()
		return
	}

	var results []string
	for _, selectedPath := range selected {
		if selectedPath == "[ALL]" {
			continue // Skip special option
		}

		// Use the existing CopyAndInstallCustomNodes function
		nodeList := []string{selectedPath}
		targetNodesDir := ExpandUserPath(filepath.Join(targetInstall.Path, "custom_nodes"))
		venvPath := ExpandUserPath(filepath.Join(targetInstall.Path, "venv"))
		if _, err := os.Stat(venvPath); os.IsNotExist(err) {
			venvPath = ExpandUserPath(filepath.Join(targetInstall.Path, ".venv"))
		}
		if err := CopyAndInstallCustomNodes(sourceNodesDir, targetNodesDir, venvPath, nodeList); err != nil {
			results = append(results, ErrorStyle.Render(fmt.Sprintf("✗ Failed to migrate %s: %v", selectedPath, err)))
		} else {
			results = append(results, SuccessStyle.Render(fmt.Sprintf("✓ Migrated custom node: %s", selectedPath)))
		}
	}

	displayMigrationSummary("Migration Results", results)
	PromptReturnToMenu()
}