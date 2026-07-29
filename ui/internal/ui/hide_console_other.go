//go:build !windows

package ui

import "os/exec"

// hideConsoleCmd 非 Windows 平台无需隐藏控制台
func hideConsoleCmd(cmd *exec.Cmd) {
	// no-op
}
