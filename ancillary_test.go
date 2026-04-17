package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFixtureManifest(t *testing.T, modelsDir, host, ns, model, tag string, layers []ManifestLayer) {
	t.Helper()
	manifest := Manifest{
		SchemaVersion: 2,
		MediaType:     "application/vnd.docker.distribution.manifest.v2+json",
		Config:        layers[0],
		Layers:        layers[1:],
	}
	data, _ := json.Marshal(manifest)
	path := filepath.Join(modelsDir, "manifests", host, ns, model, tag)
	os.MkdirAll(filepath.Dir(path), 0o755)
	os.WriteFile(path, data, 0o644)
}

func writeTestBlob(t *testing.T, modelsDir string, data []byte) string {
	t.Helper()
	h := sha256.New()
	h.Write(data)
	digest := "sha256:" + hex.EncodeToString(h.Sum(nil))
	digestName := strings.Replace(digest, ":", "-", 1)
	blobsDir := filepath.Join(modelsDir, "blobs")
	os.MkdirAll(blobsDir, 0o755)
	os.WriteFile(filepath.Join(blobsDir, digestName), data, 0o644)
	return digest
}

func TestListModels(t *testing.T) {
	dir := t.TempDir()
	writeFixtureManifest(t, dir, "registry.ollama.ai", "library", "llama3.2", "latest",
		[]ManifestLayer{
			{Digest: "sha256:config", Size: 100},
			{Digest: "sha256:layer1", Size: 1000},
		})
	writeFixtureManifest(t, dir, "registry.ollama.ai", "library", "mistral", "7b",
		[]ManifestLayer{
			{Digest: "sha256:config2", Size: 200},
			{Digest: "sha256:layer2", Size: 2000},
		})

	models, err := ListModels(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(models) != 2 {
		t.Errorf("got %d models, want 2", len(models))
	}
}

func TestShowModel(t *testing.T) {
	dir := t.TempDir()
	configData := []byte(`{"model_family":"llama","model_type":"7B","file_type":"Q4_0"}`)
	configDigest := writeTestBlob(t, dir, configData)
	layerDigest := writeTestBlob(t, dir, []byte("layer data"))

	writeFixtureManifest(t, dir, "registry.ollama.ai", "library", "test", "latest",
		[]ManifestLayer{
			{Digest: configDigest, Size: int64(len(configData))},
			{Digest: layerDigest, Size: 10},
		})

	details, err := ShowModel("test", dir)
	if err != nil {
		t.Fatal(err)
	}
	if details.Family != "llama" {
		t.Errorf("family = %q, want llama", details.Family)
	}
}

func TestShowModelNotFound(t *testing.T) {
	dir := t.TempDir()
	_, err := ShowModel("nonexistent", dir)
	if err != ErrNotFound {
		t.Errorf("got %v, want ErrNotFound", err)
	}
}

func TestRemoveModelSharedBlob(t *testing.T) {
	dir := t.TempDir()

	// Create a shared blob
	sharedData := []byte("shared")
	sharedDigest := writeTestBlob(t, dir, sharedData)

	// Two models referencing the same blob
	writeFixtureManifest(t, dir, "registry.ollama.ai", "library", "model1", "latest",
		[]ManifestLayer{
			{Digest: sharedDigest, Size: int64(len(sharedData))},
		})
	writeFixtureManifest(t, dir, "registry.ollama.ai", "library", "model2", "latest",
		[]ManifestLayer{
			{Digest: sharedDigest, Size: int64(len(sharedData))},
		})

	err := RemoveModel("model1", dir, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Shared blob should still exist
	digestName := strings.Replace(sharedDigest, ":", "-", 1)
	if _, err := os.Stat(filepath.Join(dir, "blobs", digestName)); err != nil {
		t.Errorf("shared blob should still exist: %v", err)
	}
}

func TestRemoveModelUniqueBlob(t *testing.T) {
	dir := t.TempDir()

	uniqueData := []byte("unique")
	uniqueDigest := writeTestBlob(t, dir, uniqueData)

	writeFixtureManifest(t, dir, "registry.ollama.ai", "library", "model1", "latest",
		[]ManifestLayer{
			{Digest: uniqueDigest, Size: int64(len(uniqueData))},
		})

	err := RemoveModel("model1", dir, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Unique blob should be deleted
	digestName := strings.Replace(uniqueDigest, ":", "-", 1)
	if _, err := os.Stat(filepath.Join(dir, "blobs", digestName)); !os.IsNotExist(err) {
		t.Error("unique blob should be deleted")
	}
}

func TestRemoveModelCorruptManifestAborts(t *testing.T) {
	dir := t.TempDir()

	// Target manifest is valid
	writeFixtureManifest(t, dir, "registry.ollama.ai", "library", "model1", "latest",
		[]ManifestLayer{
			{Digest: "sha256:abc", Size: 100},
		})

	// Another manifest is corrupt
	manifestsDir := filepath.Join(dir, "manifests", "registry.ollama.ai", "library", "model2")
	os.MkdirAll(manifestsDir, 0o755)
	os.WriteFile(filepath.Join(manifestsDir, "latest"), []byte("not json"), 0o644)

	err := RemoveModel("model1", dir, nil)
	if err == nil {
		t.Error("expected error when corrupt manifest exists")
	}
}

func TestClean(t *testing.T) {
	dir := t.TempDir()
	blobsDir := filepath.Join(dir, "blobs")
	pullmDir := filepath.Join(dir, ".pullm")
	os.MkdirAll(blobsDir, 0o755)
	os.MkdirAll(pullmDir, 0o755)

	// Create disposable files
	os.WriteFile(filepath.Join(blobsDir, "sha256-abc.partial"), []byte("partial"), 0o644)
	os.WriteFile(filepath.Join(blobsDir, "sha256-abc.lock"), []byte(""), 0o644)
	os.WriteFile(filepath.Join(pullmDir, "sha256-abc.json"), []byte("{}"), 0o644)

	// Create an authoritative blob
	os.WriteFile(filepath.Join(blobsDir, "sha256-real"), []byte("real data"), 0o644)

	removed, err := Clean(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	if removed != 3 {
		t.Errorf("removed %d, want 3", removed)
	}

	// Authoritative blob should still exist
	if _, err := os.Stat(filepath.Join(blobsDir, "sha256-real")); err != nil {
		t.Errorf("authoritative blob should not be removed: %v", err)
	}

	// Idempotent
	removed2, _ := Clean(dir, nil)
	if removed2 != 0 {
		t.Errorf("second clean removed %d, want 0", removed2)
	}
}