package internal

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

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
