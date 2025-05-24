//go:build windows
// +build windows

package main

import (
	"os/exec"
)

// Only build this file on Windows
func configureCmdSysProcAttr(cmd *exec.Cmd) {
	// No specific SysProcAttr settings for Windows in this context.
}
