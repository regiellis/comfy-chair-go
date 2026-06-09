package internal

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// newStubVenv creates a temp venv layout whose bin/uv records its arguments
// to a file, so tests can assert the exact command InstallTorchForGPU runs.
func newStubVenv(t *testing.T) (venvPath, argsFile string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("stub shell script not runnable on windows")
	}
	venvPath = t.TempDir()
	binDir := filepath.Join(venvPath, "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		t.Fatal(err)
	}
	argsFile = filepath.Join(venvPath, "uv-args.txt")
	stub := "#!/bin/sh\necho \"$@\" > " + argsFile + "\n"
	if err := os.WriteFile(filepath.Join(binDir, "uv"), []byte(stub), 0755); err != nil {
		t.Fatal(err)
	}
	return venvPath, argsFile
}

func recordedArgs(t *testing.T, argsFile string) string {
	t.Helper()
	data, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatalf("uv stub was not invoked: %v", err)
	}
	return strings.TrimSpace(string(data))
}

func TestInstallTorchForGPU_AMD(t *testing.T) {
	venvPath, argsFile := newStubVenv(t)
	if err := InstallTorchForGPU(t.TempDir(), venvPath, "amd", map[string]string{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := recordedArgs(t, argsFile)
	want := "pip install torch torchvision torchaudio --index-url https://download.pytorch.org/whl/rocm6.3"
	if got != want {
		t.Errorf("uv args = %q, want %q", got, want)
	}
}

func TestInstallTorchForGPU_NvidiaEnvOverride(t *testing.T) {
	venvPath, argsFile := newStubVenv(t)
	envVars := map[string]string{
		"TORCH_INSTALL_CMD_NVIDIA": "install --index-url https://example.test/custom torch",
	}
	if err := InstallTorchForGPU(t.TempDir(), venvPath, "nvidia", envVars); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := recordedArgs(t, argsFile)
	want := "pip install --index-url https://example.test/custom torch"
	if got != want {
		t.Errorf("uv args = %q, want %q", got, want)
	}
}

func TestInstallTorchForGPU_GuidanceOnlyTypes(t *testing.T) {
	for _, gpu := range []string{"apple", "ascend", "cambricon", ""} {
		// No venv/uv exists at all; guidance-only types and empty must not error.
		if err := InstallTorchForGPU(t.TempDir(), filepath.Join(t.TempDir(), "novenv"), gpu, map[string]string{}); err != nil {
			t.Errorf("gpuType %q: unexpected error: %v", gpu, err)
		}
	}
}
