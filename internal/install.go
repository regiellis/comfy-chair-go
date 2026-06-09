package internal

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	torchInstallCmdNvidiaKey     = "TORCH_INSTALL_CMD_NVIDIA"
	defaultTorchInstallCmdNvidia = "install --index-url https://download.pytorch.org/whl/nightly/cu130 torch torchvision torchaudio"
)

func defaultTorchInstallCommand(gpuType string) []string {
	switch strings.ToLower(strings.TrimSpace(gpuType)) {
	case "nvidia":
		return strings.Fields(defaultTorchInstallCmdNvidia)
	case "amd":
		return []string{"install", "torch", "torchvision", "torchaudio", "--index-url", "https://download.pytorch.org/whl/rocm6.3"}
	case "intel":
		return []string{"install", "--pre", "torch", "torchvision", "torchaudio", "--index-url", "https://download.pytorch.org/whl/nightly/xpu"}
	case "directml":
		return []string{"install", "torch-directml"}
	case "cpu":
		return []string{"install", "torch", "torchvision", "torchaudio"}
	default:
		return nil
	}
}

func resolveTorchInstallCommand(gpuType string, envVars map[string]string) ([]string, string) {
	switch strings.ToLower(strings.TrimSpace(gpuType)) {
	case "nvidia":
		if custom := strings.TrimSpace(envVars[torchInstallCmdNvidiaKey]); custom != "" {
			return strings.Fields(custom), torchInstallCmdNvidiaKey
		}
		return defaultTorchInstallCommand(gpuType), ""
	default:
		return defaultTorchInstallCommand(gpuType), ""
	}
}

// InstallTorchForGPU installs the GPU-appropriate torch/torchvision/torchaudio
// build into the venv at venvPath, replacing the default wheels pulled in by
// requirements.txt. GPU types without an automated install path (apple,
// ascend, cambricon) only print manual-install guidance.
func InstallTorchForGPU(comfyDir, venvPath, gpuType string, envVars map[string]string) error {
	gpuType = strings.ToLower(strings.TrimSpace(gpuType))
	if gpuType == "" {
		Log.Info("GPU_TYPE not set; keeping the default PyTorch from requirements.txt.")
		return nil
	}

	switch gpuType {
	case "apple":
		Log.Info("Apple Silicon: follow the official PyTorch instructions for the Metal backend: https://developer.apple.com/metal/pytorch/")
		return nil
	case "ascend":
		Log.Info("Ascend NPU: follow the official torch-npu install instructions: https://www.hiascend.com/software/modelzoo/tool/torch-npu")
		return nil
	case "cambricon":
		Log.Info("Cambricon MLU: follow the official torch_mlu install instructions: https://www.cambricon.com/")
		return nil
	}

	torchCmd, torchCmdSource := resolveTorchInstallCommand(gpuType, envVars)
	if len(torchCmd) == 0 {
		Log.Warning("Unknown GPU type %q; skipping GPU-specific PyTorch install.", gpuType)
		return nil
	}

	venvBin := filepath.Join(venvPath, "bin")
	if runtime.GOOS == "windows" {
		venvBin = filepath.Join(venvPath, "Scripts")
	}
	uvPath := filepath.Join(venvBin, "uv")
	if stat, err := os.Stat(uvPath); err != nil || stat.IsDir() {
		uvSys, err := exec.LookPath("uv")
		if err != nil {
			return fmt.Errorf("uv not found; install uv and re-run the install to get the GPU-specific PyTorch build")
		}
		uvPath = uvSys
	}

	if torchCmdSource != "" {
		Log.Info("Installing PyTorch for your GPU using %s from .env...", torchCmdSource)
	} else {
		Log.Info("Installing PyTorch for your GPU (%s)...", gpuType)
	}
	cmd := exec.Command(uvPath, append([]string{"pip"}, torchCmd...)...)
	cmd.Dir = ExpandUserPath(comfyDir)
	cmd.Env = append(os.Environ(),
		"VIRTUAL_ENV="+ExpandUserPath(venvPath),
		"PATH="+venvBin+string(os.PathListSeparator)+os.Getenv("PATH"),
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("PyTorch install failed: %w (you may need to install manually; see the README)", err)
	}
	Log.Success("PyTorch installed successfully for your GPU.")
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

// EnsurePipCompatibility ensures pip works correctly in uv-managed environments
func EnsurePipCompatibility(venvPath, uvPath string) error {
	if uvPath == "" {
		return nil // Not a uv environment
	}

	venvBin := filepath.Join(venvPath, "bin")
	if strings.Contains(venvPath, "\\") || strings.Contains(venvPath, ":\\") {
		venvBin = filepath.Join(venvPath, "Scripts") // Windows
	}

	// Run uv pip install -U pip to ensure pip compatibility
	fmt.Println(InfoStyle.Render("Ensuring pip compatibility in uv environment..."))
	cmd := exec.Command(uvPath, "pip", "install", "-U", "pip")
	cmd.Env = append(os.Environ(), "PATH="+venvBin+":"+os.Getenv("PATH"), "VIRTUAL_ENV="+venvPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		fmt.Println(WarningStyle.Render(fmt.Sprintf("Warning: Could not ensure pip compatibility: %v", err)))
		return err
	}

	fmt.Println(SuccessStyle.Render("pip compatibility ensured"))
	return nil
}

// DetectAndFixPipUvConflict detects pip/uv conflicts and attempts to fix them
func DetectAndFixPipUvConflict(err error, venvPath, uvPath string) error {
	if err == nil || uvPath == "" {
		return err
	}

	errorMsg := err.Error()
	// Check for common pip/uv conflict indicators
	conflictIndicators := []string{
		"externally-managed-environment",
		"pip._internal",
		"ModuleNotFoundError: No module named 'pip'",
		"pip: command not found",
		"ImportError: No module named pip",
		"distutils.util",
		"setuptools",
	}

	isConflict := false
	for _, indicator := range conflictIndicators {
		if strings.Contains(strings.ToLower(errorMsg), strings.ToLower(indicator)) {
			isConflict = true
			break
		}
	}

	if !isConflict {
		return err
	}

	fmt.Println(WarningStyle.Render("Detected pip/uv compatibility issue, attempting to fix..."))

	// Attempt to fix by ensuring pip compatibility
	if fixErr := EnsurePipCompatibility(venvPath, uvPath); fixErr != nil {
		fmt.Println(ErrorStyle.Render(fmt.Sprintf("Failed to fix pip/uv compatibility: %v", fixErr)))
		return err // Return original error
	}

	return nil // Fixed successfully
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
		// Proactively ensure pip compatibility in uv environment
		if err := EnsurePipCompatibility(venvPath, uvPath); err != nil {
			fmt.Println(WarningStyle.Render("Warning: Could not ensure pip compatibility, proceeding anyway"))
		}

		cmdUv := exec.Command(uvPath, "pip", "install", "-r", reqFile)
		cmdUv.Dir = nodeDir
		cmdUv.Env = append(os.Environ(), "PATH="+venvBin+":"+os.Getenv("PATH"), "VIRTUAL_ENV="+venvPath)
		installErr = cmdUv.Run()

		if installErr != nil {
			// Attempt to detect and fix pip/uv conflicts
			if fixedErr := DetectAndFixPipUvConflict(installErr, venvPath, uvPath); fixedErr == nil {
				// Retry after fixing
				fmt.Println(InfoStyle.Render("Retrying requirements installation after fixing compatibility..."))
				cmdUvRetry := exec.Command(uvPath, "pip", "install", "-r", reqFile)
				cmdUvRetry.Dir = nodeDir
				cmdUvRetry.Env = append(os.Environ(), "PATH="+venvBin+":"+os.Getenv("PATH"), "VIRTUAL_ENV="+venvPath)
				if retryErr := cmdUvRetry.Run(); retryErr == nil {
					fmt.Println(SuccessStyle.Render("Requirements installed successfully after fixing compatibility"))
					return nil
				}
			}
		} else {
			return nil
		}
	}
	// Fallback to pip
	cmdPip := exec.Command(pipPath, "install", "-r", reqFile)
	cmdPip.Dir = nodeDir
	cmdPip.Env = append(os.Environ(), "PATH="+venvBin+":"+os.Getenv("PATH"), "VIRTUAL_ENV="+venvPath)
	return cmdPip.Run()
}
