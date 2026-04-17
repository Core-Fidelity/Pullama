package main

import (
	"os"
	"path/filepath"
)

func Clean(modelsDir string, ui *UI) (int, error) {
	ui.Emit(&EvtCleanStart{})
	removed := 0
	var bytesReclaimed int64

	pullmDir := filepath.Join(modelsDir, ".pullm")
	if entries, err := os.ReadDir(pullmDir); err == nil {
		for _, e := range entries {
			if filepath.Ext(e.Name()) == ".json" {
				p := filepath.Join(pullmDir, e.Name())
				if info, _ := os.Stat(p); info != nil {
					bytesReclaimed += info.Size()
					ui.Emit(&EvtCleanFileRemoved{Path: p, Kind: "checkpoint", Size: info.Size()})
				}
				os.Remove(p)
				removed++
			}
		}
	}

	blobsDir := filepath.Join(modelsDir, "blobs")
	if entries, err := os.ReadDir(blobsDir); err == nil {
		for _, e := range entries {
			name := e.Name()
			kind := ""
			if filepath.Ext(name) == ".partial" {
				kind = "partial"
			} else if filepath.Ext(name) == ".lock" {
				kind = "lock"
			}
			if kind != "" {
				p := filepath.Join(blobsDir, name)
				if info, _ := os.Stat(p); info != nil {
					bytesReclaimed += info.Size()
					ui.Emit(&EvtCleanFileRemoved{Path: p, Kind: kind, Size: info.Size()})
				}
				os.Remove(p)
				removed++
			}
		}
	}

	ui.Emit(&EvtCleanCompleted{Removed: removed, BytesReclaimed: bytesReclaimed})
	return removed, nil
}