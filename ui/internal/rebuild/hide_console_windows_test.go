//go:build windows

package rebuild

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

func TestExecRunner_SetsNoWindowOnWindows(t *testing.T) {
	// Construct the same way ExecRunner does and assert attributes before Start.
	cmd := exec.Command("go", "version")
	hideConsoleCmd(cmd)
	if cmd.SysProcAttr == nil || !cmd.SysProcAttr.HideWindow {
		t.Fatal("ExecRunner path must set HideWindow on Windows")
	}
	if cmd.SysProcAttr.CreationFlags&0x08000000 == 0 {
		t.Fatal("ExecRunner path must set CREATE_NO_WINDOW")
	}
}
