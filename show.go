package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type ModelDetails struct {
	Name           string
	Family         string
	ParameterSize  string
	Quantization   string
	Size           int64
	Layers         []LayerRow
}

func ShowModel(name, modelsDir string) (*ModelDetails, error) {
	ref, err := ParseModelRef(name)
	if err != nil {
		return nil, err
	}

	manifestPath := filepath.Join(modelsDir, "manifests", ref.ManifestPath())
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, ErrManifestCorrupt
	}

	// Read config blob
	digestName := strings.Replace(m.Config.Digest, ":", "-", 1)
	configPath := filepath.Join(modelsDir, "blobs", digestName)
	configData, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("cannot read config blob: %v", err)
	}

	var cfg struct {
		ModelFormat string   `json:"model_format"`
		ModelFamily string   `json:"model_family"`
		ModelType   string   `json:"model_type"`
		FileType    string   `json:"file_type"`
	}
	json.Unmarshal(configData, &cfg)

	var totalSize int64
	totalSize += m.Config.Size
	for _, l := range m.Layers {
		totalSize += l.Size
	}

	var layers []LayerRow
	layers = append(layers, LayerRow{MediaType: m.Config.MediaType, Digest: m.Config.Digest, Size: m.Config.Size})
	for _, l := range m.Layers {
		layers = append(layers, LayerRow{MediaType: l.MediaType, Digest: l.Digest, Size: l.Size})
	}

	return &ModelDetails{
		Name:          name,
		Family:        cfg.ModelFamily,
		ParameterSize: cfg.ModelType,
		Quantization:  cfg.FileType,
		Size:          totalSize,
		Layers:        layers,
	}, nil
}