package main

import (
	"os"
	"path/filepath"
)

// atomicWrite writes data to path atomically: write .tmp → fsync → rename → fsync parent dir.
func atomicWrite(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return err
	}
	return fsyncDir(dir)
}

func fsyncDir(path string) error {
	d, err := os.Open(path)
	if err != nil {
		return err
	}
	err = d.Sync()
	d.Close()
	return err
}