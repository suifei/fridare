//go:build windows

package rebuild

import (
	"os/exec"
	"syscall"
)

// hideConsoleCmd prevents black console flashes when GUI/e2e spawns
// docker / git / python / cmd children on Windows (CREATE_NO_WINDOW).
func hideConsoleCmd(cmd *exec.Cmd) {
	if cmd == nil {
		return
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: 0x08000000, // CREATE_NO_WINDOW
	}
}
