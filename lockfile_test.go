package main

import (
	"path/filepath"
	"testing"
)

func TestLockAcquireRelease(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.lock")

	lock, err := AcquireLock(path)
	if err != nil {
		t.Fatal(err)
	}

	// Second acquire should fail
	_, err = AcquireLock(path)
	if err != ErrLockBusy {
		t.Errorf("second acquire: got %v, want ErrLockBusy", err)
	}

	// Release and reacquire
	if err := lock.Release(); err != nil {
		t.Fatal(err)
	}
	lock2, err := AcquireLock(path)
	if err != nil {
		t.Errorf("reacquire after release: %v", err)
	}
	lock2.Release()
}