//go:build windows

package config

import (
	"syscall"
	"unsafe"
)

var moveFileEx = syscall.NewLazyDLL("kernel32.dll").NewProc("MoveFileExW")

func replaceFile(source, destination string) error {
	sourceUTF16, err := syscall.UTF16PtrFromString(source)
	if err != nil {
		return err
	}
	destinationUTF16, err := syscall.UTF16PtrFromString(destination)
	if err != nil {
		return err
	}
	const replaceExistingAndWriteThrough = 0x1 | 0x8
	ok, _, callErr := moveFileEx.Call(uintptr(unsafe.Pointer(sourceUTF16)), uintptr(unsafe.Pointer(destinationUTF16)), replaceExistingAndWriteThrough)
	if ok == 0 {
		return callErr
	}
	return nil
}
