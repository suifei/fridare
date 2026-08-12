//go:build !windows

package rebuild

import (
	"fmt"
	"syscall"
)

func freeDiskGB(path string) (float64, error) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return 0, fmt.Errorf("statfs %s: %w", path, err)
	}
	// Prefer Bavail (free for non-root)
	free := float64(st.Bavail) * float64(st.Bsize)
	return free / (1024 * 1024 * 1024), nil
}
