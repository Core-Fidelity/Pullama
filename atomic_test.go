package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAtomicWrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.json")

	if err := atomicWrite(path, []byte("hello")); err != nil {
		t.Fatal(err)
	}

	// .tmp should not exist
	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Error(".tmp file still exists after atomicWrite")
	}

	// path should contain data
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "hello" {
		t.Errorf("got %q, want %q", data, "hello")
	}
}

func TestAtomicWriteCrashBeforeRename(t *testing.T) {
	// Simulate crash after .tmp write but before rename:
	// .tmp exists, original untouched
	dir := t.TempDir()
	path := filepath.Join(dir, "test.json")

	// Write initial content
	if err := atomicWrite(path, []byte("first")); err != nil {
		t.Fatal(err)
	}

	// Create a .tmp manually (simulating crash)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte("crashed"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Original should still have "first"
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "first" {
		t.Errorf("got %q, want %q", data, "first")
	}
}