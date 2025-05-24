//go:build windows

package main

import (
	"os/exec"
)

func configureCmdSysProcAttr(cmd *exec.Cmd) {
	// No specific SysProcAttr settings for Windows in this context.
	// Windows does not use Setsid.
}
