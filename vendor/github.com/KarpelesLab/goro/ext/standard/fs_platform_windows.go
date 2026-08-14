//go:build windows

package standard

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"
)

func platformAccess(path string, mode uint32) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if mode == 0x1 {
		extension := strings.ToLower(filepath.Ext(path))
		if info.IsDir() || extension == ".exe" || extension == ".com" || extension == ".bat" || extension == ".cmd" {
			return nil
		}
		return syscall.EACCES
	}
	if mode == 0x2 && !info.IsDir() {
		file, err := os.OpenFile(path, os.O_WRONLY, 0)
		if err != nil {
			return err
		}
		return file.Close()
	}
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	return file.Close()
}

func platformDevice(os.FileInfo) (int64, bool) { return 0, false }

func platformUmask(int) int { return 0 }

func platformDiskSpace(path string) (float64, float64, error) {
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	procedure := kernel32.NewProc("GetDiskFreeSpaceExW")
	wide, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return 0, 0, err
	}
	var available, total, free uint64
	result, _, callErr := procedure.Call(
		uintptr(unsafe.Pointer(wide)), uintptr(unsafe.Pointer(&available)),
		uintptr(unsafe.Pointer(&total)), uintptr(unsafe.Pointer(&free)),
	)
	if result == 0 {
		if callErr != nil && !errors.Is(callErr, syscall.Errno(0)) {
			return 0, 0, callErr
		}
		return 0, 0, syscall.EINVAL
	}
	return float64(available), float64(total), nil
}

func platformFileStat(info os.FileInfo) fileStat {
	result := fallbackFileStat(info)
	if data, ok := info.Sys().(*syscall.Win32FileAttributeData); ok && data != nil {
		result.atime = data.LastAccessTime.Nanoseconds() / 1e9
		result.ctime = data.CreationTime.Nanoseconds() / 1e9
	}
	return result
}

func platformResourceUsage(bool) (resourceUsage, error) { return resourceUsage{}, nil }

func platformNice(int) error { return syscall.EWINDOWS }

func platformSyslog(int, string) error { return syscall.EWINDOWS }
