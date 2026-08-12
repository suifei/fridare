//go:build windows

package rebuild

import (
	"fmt"
	"path/filepath"
	"syscall"
	"unsafe"
)

func freeDiskGB(path string) (float64, error) {
	// Use volume root of path
	vol := filepath.VolumeName(path)
	if vol == "" {
		vol = path
	}
	if !hasTrailingSlash(vol) {
		vol += `\`
	}

	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	getDiskFreeSpaceExW := kernel32.NewProc("GetDiskFreeSpaceExW")

	var freeBytesAvailable, totalNumberOfBytes, totalNumberOfFreeBytes uint64
	pathPtr, err := syscall.UTF16PtrFromString(vol)
	if err != nil {
		return 0, err
	}
	r1, _, e1 := getDiskFreeSpaceExW.Call(
		uintptr(unsafe.Pointer(pathPtr)),
		uintptr(unsafe.Pointer(&freeBytesAvailable)),
		uintptr(unsafe.Pointer(&totalNumberOfBytes)),
		uintptr(unsafe.Pointer(&totalNumberOfFreeBytes)),
	)
	if r1 == 0 {
		if e1 != nil {
			return 0, fmt.Errorf("GetDiskFreeSpaceExW(%s): %v", vol, e1)
		}
		return 0, fmt.Errorf("GetDiskFreeSpaceExW(%s) failed", vol)
	}
	return float64(freeBytesAvailable) / (1024 * 1024 * 1024), nil
}

func hasTrailingSlash(s string) bool {
	return len(s) > 0 && (s[len(s)-1] == '\\' || s[len(s)-1] == '/')
}
