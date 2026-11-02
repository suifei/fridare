//go:build !windows

package rebuild

import "os/exec"

// hideConsoleCmd is a no-op outside Windows (no console window flash).
func hideConsoleCmd(cmd *exec.Cmd) {
	// no-op
}
