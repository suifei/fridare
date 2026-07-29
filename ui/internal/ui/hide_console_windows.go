//go:build windows

package ui

import (
	"os/exec"
	"syscall"
)

// hideConsoleCmd 隐藏 Windows 子进程控制台窗口
func hideConsoleCmd(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: 0x08000000, // CREATE_NO_WINDOW
	}
}
