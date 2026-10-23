package ui

import (
	"os/exec"
	"runtime"
	"testing"
)

func TestHideConsoleCmd_DoesNotPanic(t *testing.T) {
	cmd := exec.Command("go", "version")
	hideConsoleCmd(cmd)
	if runtime.GOOS != "windows" && cmd.SysProcAttr != nil {
		t.Fatal("non-windows: SysProcAttr should remain nil")
	}
}
