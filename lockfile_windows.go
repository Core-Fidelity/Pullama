//go:build windows

package main

import (
	"os"
	"syscall"
	"unsafe"
)

var (
	kernel32       = syscall.NewLazyDLL("kernel32.dll")
	procLockFileEx = kernel32.NewProc("LockFileEx")
)

func AcquireLock(path string) (*Lock, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, err
	}
	fd := syscall.Handle(f.Fd())
	var ol syscall.Overlapped
	r1, _, _ := procLockFileEx.Call(
		uintptr(fd), 0x00000003, 0, 0xFFFFFFFF, 0xFFFFFFFF,
		uintptr(unsafe.Pointer(&ol)),
	)
	if r1 == 0 {
		f.Close()
		return nil, ErrLockBusy
	}
	return &Lock{fd: int(fd), path: path}, nil
}

func releaseLock(fd int) error {
	return syscall.Close(syscall.Handle(fd))
}