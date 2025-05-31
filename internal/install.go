package internal

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

var defaultCustomNodes = []struct {
	Name string
	Repo string
}{
	{"ComfyUI-Manager", "https://github.com/Comfy-Org/ComfyUI-Manager.git"},
	{"ComfyUI-Crystools", "https://github.com/crystian/ComfyUI-Crystools.git"},
	{"rgthree-comfy", "https://github.com/rgthree/rgthree-comfy"},
}

// DefaultCustomNodes returns the list of default custom nodes and their repos.
func DefaultCustomNodes() []struct {
	Name string
	Repo string
} {
	return defaultCustomNodes
}

// Exported version for use in migration logic
var DefaultCustomNodesList = defaultCustomNodes

// Export installNodeRequirements for migration use
var InstallNodeRequirements = installNodeRequirements

// InstallComfyUI performs the installation logic. It takes all required dependencies as parameters.
// UI, error handling, and user prompts should be handled by the caller (main.go).
func InstallComfyUI(
	appPaths *Paths,
	findSystemPython func() (string, error),
	saveComfyUIPathToEnv func(string) error,
	initPaths func() error,
	executeCommand func(string, []string, string, string, bool) (*os.Process, error),
	promptInstall func(defaultInstallPath, foundSystemPython string) (installPath, systemPythonExec string, proceed bool),
	promptConfirmPython func(systemPythonExec, outputPyVersion string) bool,
) error {
	// ...existing code from installComfyUI, refactored to use parameters and return errors instead of printing...
	// All UI and error printing should be handled by the caller.

	// In the install logic, after setting up the environment and before finishing:
	for _, node := range defaultCustomNodes {
		nodePath := filepath.Join(ExpandUserPath(appPaths.ComfyUIDir), "custom_nodes", node.Name)
		if _, err := os.Stat(nodePath); os.IsNotExist(err) {
			fmt.Println(InfoStyle.Render("Cloning default node: " + node.Name))
			cmd := exec.Command("git", "clone", node.Repo, nodePath)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			_ = cmd.Run()
		}
		// After cloning, install requirements if present
		reqFile := filepath.Join(nodePath, "requirements.txt")
		venvPython, err := FindVenvPython(ExpandUserPath(appPaths.ComfyUIDir))
		if err == nil {
			venvBin := filepath.Join(filepath.Dir(filepath.Dir(venvPython)), "bin")
			uvPath := filepath.Join(venvBin, "uv")
			if _, err := os.Stat(uvPath); err == nil {
				// Ensure pip is installed in the venv
				cmdPip := exec.Command(uvPath, "pip", "install", "-U", "pip")
				cmdPip.Dir = ExpandUserPath(appPaths.ComfyUIDir)
				cmdPip.Env = append(os.Environ(), "PATH="+venvBin+":"+os.Getenv("PATH"))
				cmdPip.Stdout = os.Stdout
				cmdPip.Stderr = os.Stderr
				_ = cmdPip.Run()
				// Install requirements if present
				if _, err := os.Stat(reqFile); err == nil {
					cmdReq := exec.Command(uvPath, "pip", "install", "-r", reqFile)
					cmdReq.Dir = nodePath
					cmdReq.Env = append(os.Environ(), "PATH="+venvBin+":"+os.Getenv("PATH"))
					cmdReq.Stdout = os.Stdout
					cmdReq.Stderr = os.Stderr
					if err := cmdReq.Run(); err != nil {
						// Fallback to pip if uv fails
						pipPath := filepath.Join(venvBin, "pip")
						if _, err := os.Stat(pipPath); err == nil {
							cmdPip2 := exec.Command(pipPath, "install", "-r", reqFile)
							cmdPip2.Dir = nodePath
							cmdPip2.Env = append(os.Environ(), "PATH="+venvBin+":"+os.Getenv("PATH"))
							cmdPip2.Stdout = os.Stdout
							cmdPip2.Stderr = os.Stderr
							_ = cmdPip2.Run()
						}
					}
				}
			}
		}
	}

	// After venv/uv setup, install comfy-cli:
	venvPython, err := FindVenvPython(ExpandUserPath(appPaths.ComfyUIDir))
	if err == nil {
		venvBin := filepath.Join(filepath.Dir(filepath.Dir(venvPython)), "bin")
		uvPath := filepath.Join(venvBin, "uv")
		if _, err := os.Stat(uvPath); err == nil {
			cmd := exec.Command(uvPath, "pip", "install", "comfy-cli")
			cmd.Dir = ExpandUserPath(appPaths.ComfyUIDir)
			cmd.Env = append(os.Environ(), "PATH="+venvBin+":"+os.Getenv("PATH"))
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			_ = cmd.Run()
		}
	}

	return nil
}

// CloneComfyUI clones the ComfyUI repository into the specified installPath.
func CloneComfyUI(comfyUIRepoURL, installPath string, executeCommand func(string, []string, string, string, bool) (*os.Process, error)) error {
	parentDir := filepath.Dir(installPath)
	repoDirName := filepath.Base(installPath)
	_, err := executeCommand("git", []string{"clone", comfyUIRepoURL, repoDirName}, parentDir, "", false)
	return err
}

// CopyAndInstallCustomNodes copies selected custom node directories from src to dst, skipping venv/.venv, and installs requirements with uv or pip.
func CopyAndInstallCustomNodes(srcCustomNodes, dstCustomNodes, venvPath string, nodeNames []string) error {
	srcCustomNodes = ExpandUserPath(srcCustomNodes)
	dstCustomNodes = ExpandUserPath(dstCustomNodes)
	venvPath = ExpandUserPath(venvPath)
	for _, node := range nodeNames {
		if node == "venv" || node == ".venv" {
			continue
		}
		srcDir := filepath.Join(srcCustomNodes, node)
		dstDir := filepath.Join(dstCustomNodes, node)
		// Copy directory recursively
		err := copyDir(srcDir, dstDir)
		if err != nil {
			return fmt.Errorf("failed to copy node %s: %w", node, err)
		}
		// Install requirements if present
		reqFile := filepath.Join(dstDir, "requirements.txt")
		if _, err := os.Stat(reqFile); err == nil {
			if err := installNodeRequirements(venvPath, dstDir, reqFile); err != nil {
				return fmt.Errorf("failed to install requirements for node %s: %w", node, err)
			}
		}
	}
	return nil
}

// copyDir recursively copies a directory tree from src to dst.
func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, info.Mode())
		}
		fSrc, err := os.Open(path)
		if err != nil {
			return err
		}
		defer fSrc.Close()
		fDst, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode())
		if err != nil {
			return err
		}
		defer fDst.Close()
		_, err = io.Copy(fDst, fSrc)
		return err
	})
}

// installNodeRequirements tries uv pip install -r requirements.txt, falls back to pip if uv is not found.
func installNodeRequirements(venvPath, nodeDir, reqFile string) error {
	venvBin := filepath.Join(venvPath, "bin")
	if strings.Contains(venvPath, "\\") || strings.Contains(venvPath, ":\\") {
		venvBin = filepath.Join(venvPath, "Scripts") // Windows
	}
	uvPath := filepath.Join(venvBin, "uv")
	if _, err := os.Stat(uvPath); err != nil {
		if uvSys, err := exec.LookPath("uv"); err == nil {
			uvPath = uvSys
		} else {
			uvPath = ""
		}
	}
	pipPath := filepath.Join(venvBin, "pip")
	if _, err := os.Stat(pipPath); err != nil {
		if pipSys, err := exec.LookPath("pip"); err == nil {
			pipPath = pipSys
		}
	}
	var installErr error
	if uvPath != "" {
		cmdUv := exec.Command(uvPath, "pip", "install", "-r", reqFile)
		cmdUv.Dir = nodeDir
		cmdUv.Env = append(os.Environ(), "PATH="+venvBin+":"+os.Getenv("PATH"), "VIRTUAL_ENV="+venvPath)
		installErr = cmdUv.Run()
		if installErr == nil {
			return nil
		}
	}
	// Fallback to pip
	cmdPip := exec.Command(pipPath, "install", "-r", reqFile)
	cmdPip.Dir = nodeDir
	cmdPip.Env = append(os.Environ(), "PATH="+venvBin+":"+os.Getenv("PATH"), "VIRTUAL_ENV="+venvPath)
	return cmdPip.Run()
}
