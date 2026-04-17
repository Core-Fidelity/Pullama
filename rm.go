package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

func RemoveModel(name, modelsDir string, ui *UI) error {
	ref, err := ParseModelRef(name)
	if err != nil {
		return err
	}
	ui.Emit(&EvtRmStart{Model: name})

	manifestsDir := filepath.Join(modelsDir, "manifests")
	os.MkdirAll(manifestsDir, 0o755)
	lock, err := AcquireLock(filepath.Join(manifestsDir, ".pullm-rm.lock"))
	if err != nil {
		return err
	}
	defer lock.Release()

	targetPath := filepath.Join(manifestsDir, ref.ManifestPath())
	data, err := os.ReadFile(targetPath)
	if err != nil {
		if os.IsNotExist(err) {
			return ErrNotFound
		}
		return err
	}
	var targetManifest Manifest
	if err := json.Unmarshal(data, &targetManifest); err != nil {
		return ErrManifestCorrupt
	}

	targetDigests := map[string]bool{}
	for _, l := range targetManifest.Layers {
		targetDigests[l.Digest] = true
	}
	targetDigests[targetManifest.Config.Digest] = true

	activeDigests := map[string]bool{}
	var bytesReclaimed int64
	blobsDeleted := 0
	err = filepath.Walk(manifestsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		if strings.Contains(info.Name(), ".lock") || strings.HasPrefix(info.Name(), ".") {
			return nil
		}
		if path == targetPath {
			return nil
		}
		d, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		var m Manifest
		if err := json.Unmarshal(d, &m); err != nil {
			return ErrManifestCorrupt
		}
		activeDigests[m.Config.Digest] = true
		for _, l := range m.Layers {
			activeDigests[l.Digest] = true
		}
		return nil
	})
	if err != nil {
		return err
	}

	if err := os.Remove(targetPath); err != nil {
		return err
	}
	ui.Emit(&EvtRmManifestDeleted{Model: ref.Name, Tag: ref.Tag})

	for dir := filepath.Dir(targetPath); dir != manifestsDir; dir = filepath.Dir(dir) {
		if entries, _ := os.ReadDir(dir); len(entries) == 0 {
			os.Remove(dir)
		} else {
			break
		}
	}

	blobsDir := filepath.Join(modelsDir, "blobs")
	for digest := range targetDigests {
		if activeDigests[digest] {
			continue
		}
		dn := strings.Replace(digest, ":", "-", 1)
		blobPath := filepath.Join(blobsDir, dn)
		if info, _ := os.Stat(blobPath); info != nil {
			bytesReclaimed += info.Size()
			ui.Emit(&EvtRmBlobDeleted{Digest: digest, Size: info.Size()})
		}
		os.Remove(blobPath)
		os.Remove(filepath.Join(blobsDir, dn+".partial"))
		os.Remove(filepath.Join(blobsDir, dn+".lock"))
		DeleteCheckpoint(modelsDir, digest)
		blobsDeleted++
	}

	ui.Emit(&EvtRmCompleted{Model: name, BlobsDeleted: blobsDeleted, BytesReclaimed: bytesReclaimed})
	return nil
}