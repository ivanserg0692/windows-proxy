//go:build windows

package config

import (
	"syscall"
	"unsafe"
)

var moveFileEx = syscall.NewLazyDLL("kernel32.dll").NewProc("MoveFileExW")

func replaceFile(source, destination string) error {
	from, err := syscall.UTF16PtrFromString(source)
	if err != nil {
		return err
	}
	to, err := syscall.UTF16PtrFromString(destination)
	if err != nil {
		return err
	}
	const replaceExistingAndWriteThrough = 0x1 | 0x8
	ok, _, callErr := moveFileEx.Call(uintptr(unsafe.Pointer(from)), uintptr(unsafe.Pointer(to)), replaceExistingAndWriteThrough)
	if ok == 0 {
		return callErr
	}
	return nil
}
