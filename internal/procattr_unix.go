//go:build !windows

package internal

import (
	"os/exec"
	"syscall"
)

// configureBackgroundSysProcAttr detaches a background child into its own
// session so terminal signals (SIGHUP on close, Ctrl+C) don't reach it.
func configureBackgroundSysProcAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}
