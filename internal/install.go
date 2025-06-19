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
func InstallComfyUI(
	appPaths *Paths,
	findSystemPython func() (string, error),
	saveComfyUIPathToEnv func(string) error,
	initPaths func() error,
	executeCommand func(string, []string, string, string, bool) (*os.Process, error),
	promptInstall func(defaultInstallPath, foundSystemPython string) (installPath, systemPythonExec string, proceed bool),
	promptConfirmPython func(systemPythonExec, outputPyVersion string) bool,
) error {

	// In the install logic, after setting up the environment and before finishing:
	for _, node := range defaultCustomNodes {
		nodePath := filepath.Join(ExpandUserPath(appPaths.ComfyUIDir), "custom_nodes", node.Name)
		if _, err := os.Stat(nodePath); os.IsNotExist(err) {
			fmt.Println(InfoStyle.Render("Cloning default node: " + node.Name))
			cmd := exec.Command("git", "clone", node.Repo, nodePath)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			if err := cmd.Run(); err != nil {
				fmt.Println(ErrorStyle.Render(fmt.Sprintf("Failed to clone node %s: %v", node.Name, err)))
				continue
			}
			fmt.Println(SuccessStyle.Render(fmt.Sprintf("Successfully cloned node: %s", node.Name)))
		}
		// After cloning, install requirements if present
		reqFile := filepath.Join(nodePath, "requirements.txt")
		venvPython, err := FindVenvPython(ExpandUserPath(appPaths.ComfyUIDir))
		if err == nil {
			venvBin := filepath.Join(filepath.Dir(filepath.Dir(venvPython)), "bin")
			uvPath := filepath.Join(venvBin, "uv")
			if _, err := os.Stat(uvPath); err == nil {
				// Ensure pip is installed in the venv
				fmt.Println(InfoStyle.Render("Updating pip in virtual environment..."))
				cmdPip := exec.Command(uvPath, "pip", "install", "-U", "pip")
				cmdPip.Dir = ExpandUserPath(appPaths.ComfyUIDir)
				cmdPip.Env = append(os.Environ(), "PATH="+venvBin+":"+os.Getenv("PATH"))
				cmdPip.Stdout = os.Stdout
				cmdPip.Stderr = os.Stderr
				if err := cmdPip.Run(); err != nil {
					fmt.Println(ErrorStyle.Render(fmt.Sprintf("Failed to update pip: %v", err)))
				} else {
					fmt.Println(SuccessStyle.Render("Successfully updated pip"))
				}
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
							fmt.Println(InfoStyle.Render("Falling back to pip for requirements installation..."))
							cmdPip2 := exec.Command(pipPath, "install", "-r", reqFile)
							cmdPip2.Dir = nodePath
							cmdPip2.Env = append(os.Environ(), "PATH="+venvBin+":"+os.Getenv("PATH"))
							cmdPip2.Stdout = os.Stdout
							cmdPip2.Stderr = os.Stderr
							if err := cmdPip2.Run(); err != nil {
								fmt.Println(ErrorStyle.Render(fmt.Sprintf("Failed to install requirements with pip fallback: %v", err)))
							} else {
								fmt.Println(SuccessStyle.Render("Successfully installed requirements with pip"))
							}
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
			fmt.Println(InfoStyle.Render("Installing comfy-cli..."))
			cmd := exec.Command(uvPath, "pip", "install", "comfy-cli")
			cmd.Dir = ExpandUserPath(appPaths.ComfyUIDir)
			cmd.Env = append(os.Environ(), "PATH="+venvBin+":"+os.Getenv("PATH"))
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			if err := cmd.Run(); err != nil {
				fmt.Println(ErrorStyle.Render(fmt.Sprintf("Failed to install comfy-cli: %v", err)))
			} else {
				fmt.Println(SuccessStyle.Render("Successfully installed comfy-cli"))
			}
		}
	}

	// --- Install torch/torchvision/torchaudio for selected GPU ---
	envVars, _ := ReadEnvFile(filepath.Join(appPaths.CliDir, ".env"))
	gpuType := envVars["GPU_TYPE"]
	pythonVersion := envVars["PYTHON_VERSION"]
	if pythonVersion == "" {
		pythonVersion = "3.12"
	}
	venvPython, err = FindVenvPython(ExpandUserPath(appPaths.ComfyUIDir))
	if err == nil {
		venvBin := filepath.Join(filepath.Dir(filepath.Dir(venvPython)), "bin")
		pipPath := filepath.Join(venvBin, "pip")
		var torchCmd []string
		switch strings.ToLower(gpuType) {
		case "nvidia":
			torchCmd = []string{"install", "torch", "torchvision", "torchaudio", "--extra-index-url", "https://download.pytorch.org/whl/cu128"}
		case "amd":
			torchCmd = []string{"install", "torch", "torchvision", "torchaudio", "--index-url", "https://download.pytorch.org/whl/rocm6.3"}
		case "intel":
			torchCmd = []string{"install", "--pre", "torch", "torchvision", "torchaudio", "--index-url", "https://download.pytorch.org/whl/nightly/xpu"}
		case "apple":
			fmt.Println(InfoStyle.Render("Apple Silicon: Please follow the official PyTorch nightly install instructions for Metal backend. See: https://developer.apple.com/metal/pytorch/"))
		case "directml":
			torchCmd = []string{"install", "torch-directml"}
		case "ascend":
			fmt.Println(InfoStyle.Render("Ascend NPU: Please follow the official torch-npu install instructions. See: https://www.hiascend.com/software/modelzoo/tool/torch-npu"))
		case "cambricon":
			fmt.Println(InfoStyle.Render("Cambricon MLU: Please follow the official torch_mlu install instructions. See: https://www.cambricon.com/"))
		case "cpu":
			torchCmd = []string{"install", "torch", "torchvision", "torchaudio"}
		}
		if len(torchCmd) > 0 {
			fmt.Println(InfoStyle.Render("Installing PyTorch for your GPU in the venv..."))
			cmd := exec.Command(pipPath, torchCmd...)
			cmd.Dir = ExpandUserPath(appPaths.ComfyUIDir)
			cmd.Env = append(os.Environ(), "PATH="+venvBin+":"+os.Getenv("PATH"))
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			if err := cmd.Run(); err != nil {
				fmt.Println(WarningStyle.Render("PyTorch install failed. You may need to install manually. See README for details."))
			} else {
				fmt.Println(SuccessStyle.Render("PyTorch installed successfully for your GPU."))
			}
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
