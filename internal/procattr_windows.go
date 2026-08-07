//go:build windows

package internal

import (
	"os/exec"
	"syscall"
)

// configureBackgroundSysProcAttr detaches a background child from the CLI's
// console so it survives the terminal closing.
func configureBackgroundSysProcAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP | 0x00000008} // DETACHED_PROCESS
}
