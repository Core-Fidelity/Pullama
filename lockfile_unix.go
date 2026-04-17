//go:build !windows

package main

import (
	"os"
	"syscall"
)

func AcquireLock(path string) (*Lock, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, err
	}
	fd := int(f.Fd())
	if err := syscall.Flock(fd, syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		return nil, ErrLockBusy
	}
	return &Lock{fd: fd, path: path}, nil
}

func releaseLock(fd int) error {
	return syscall.Close(fd)
}