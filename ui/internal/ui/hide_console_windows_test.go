//go:build windows

package ui

import (
	"os/exec"
	"testing"
)

func TestHideConsoleCmd_WindowsFlags(t *testing.T) {
	cmd := exec.Command("go", "version")
	hideConsoleCmd(cmd)
	if cmd.SysProcAttr == nil {
		t.Fatal("windows: SysProcAttr should be set")
	}
	if !cmd.SysProcAttr.HideWindow {
		t.Fatal("windows: HideWindow should be true")
	}
	if cmd.SysProcAttr.CreationFlags != 0x08000000 {
		t.Fatalf("windows: CreationFlags want CREATE_NO_WINDOW got %#x", cmd.SysProcAttr.CreationFlags)
	}
}
