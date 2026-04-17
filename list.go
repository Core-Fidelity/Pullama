package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type ModelInfo struct {
	Name     string
	Size     int64
	Modified time.Time
}

func ListModels(modelsDir string) ([]ModelInfo, error) {
	manifestsDir := filepath.Join(modelsDir, "manifests")
	var models []ModelInfo

	err := filepath.Walk(manifestsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if info.IsDir() {
			return nil
		}

		// Read manifest to compute size
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		var m Manifest
		if err := json.Unmarshal(data, &m); err != nil {
			// Skip non-manifest files (e.g. .DS_Store)
			return nil
		}

		// Build model name from path relative to manifests/
		rel, _ := filepath.Rel(manifestsDir, path)
		// rel = host/ns/model/tag → name = host/ns/model:tag
		parts := strings.Split(rel, string(filepath.Separator))
		var name string
		if len(parts) >= 4 {
			name = strings.Join(parts[:3], "/") + ":" + parts[3]
		} else {
			name = rel
		}

		var totalSize int64
		if m.Config.Size > 0 {
			totalSize += m.Config.Size
		}
		for _, l := range m.Layers {
			totalSize += l.Size
		}

		models = append(models, ModelInfo{
			Name:     name,
			Size:     totalSize,
			Modified: info.ModTime(),
		})
		return nil
	})

	return models, err
}