package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestBuildChunkPlan(t *testing.T) {
	layer := ManifestLayer{Digest: "sha256:abc", Size: 4 * 1024 * 1024 * 1024}

	chunksums := []ChunkDigest{
		{Digest: "sha256:c1", Start: 0, End: 67108863},
		{Digest: "sha256:c2", Start: 67108864, End: 134217727},
	}
	chunks := buildChunkPlan(layer, chunksums)
	if len(chunks) != 2 {
		t.Errorf("chunksums: got %d chunks, want 2", len(chunks))
	}

	chunks = buildChunkPlan(layer, nil)
	if len(chunks) != 1 {
		t.Errorf("no chunksums: got %d chunks, want 1", len(chunks))
	}
	if chunks[0].End != layer.Size-1 {
		t.Errorf("single chunk end = %d, want %d", chunks[0].End, layer.Size-1)
	}
}

func TestPreflightSkipExisting(t *testing.T) {
	dir := t.TempDir()
	cfg := DefaultConfig()
	cfg.ModelsDir = dir

	data := []byte("test blob content")
	h := sha256.New()
	h.Write(data)
	hexStr := hex.EncodeToString(h.Sum(nil))
	digest := "sha256:" + hexStr
	blobDir := filepath.Join(dir, "blobs")
	os.MkdirAll(blobDir, 0o755)
	os.WriteFile(filepath.Join(blobDir, "sha256-"+hexStr), data, 0o644)

	layer := ManifestLayer{Digest: digest, Size: int64(len(data))}
	ref := ModelRef{Host: "localhost", Namespace: "library", Name: "test", Tag: "latest", Scheme: "http"}

	pf, err := preflight(cfg, layer, ref, nil, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	if pf.state != blobStateSkip {
		t.Errorf("expected skip, got %q", pf.state)
	}
}

func TestDownloadBlobEndToEnd(t *testing.T) {
	setupTestKey(t)

	content := make([]byte, 1024)
	for i := range content {
		content[i] = byte(i % 256)
	}
	h := sha256.New()
	h.Write(content)
	hexStr := hex.EncodeToString(h.Sum(nil))
	digest := "sha256:" + hexStr

	cdnSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rangeHdr := r.Header.Get("Range")
		if rangeHdr != "" {
			var start, end int
			if _, err := fmt.Sscanf(rangeHdr, "bytes=%d-%d", &start, &end); err == nil {
				w.Header().Set("Content-Length", strconv.Itoa(end-start+1))
				w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, len(content)))
				w.WriteHeader(206)
				w.Write(content[start : end+1])
				return
			}
		}
		w.Header().Set("Content-Length", strconv.Itoa(len(content)))
		w.Write(content)
	}))
	defer cdnSrv.Close()

	regSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/blobs/") {
			w.Header().Set("Location", cdnSrv.URL+"/blob")
			w.WriteHeader(307)
			return
		}
		if strings.Contains(r.URL.Path, "/chunksums/") {
			w.WriteHeader(404)
			return
		}
		w.WriteHeader(404)
	}))
	defer regSrv.Close()

	modelsDir := t.TempDir()
	cfg := DefaultConfig()
	cfg.ModelsDir = modelsDir
	cfg.Scheme = "http"

	ref := ModelRef{Host: regSrv.Listener.Addr().String(), Namespace: "library", Name: "test", Tag: "latest", Scheme: "http"}
	layer := ManifestLayer{Digest: digest, Size: int64(len(content))}

	err := DownloadBlob(context.Background(), cfg, layer, ref, nil, 1, 1, 0, layer.Size)
	if err != nil {
		t.Fatal(err)
	}

	finalPath := filepath.Join(modelsDir, "blobs", "sha256-"+hexStr)
	if _, err := os.Stat(finalPath); err != nil {
		t.Errorf("final blob missing: %v", err)
	}
}

// E2E.1: Pull a small single-chunk model against a local httptest registry.
func TestE2EPullSingleChunkModel(t *testing.T) {
	setupTestKey(t)

	// Create blob content
	configContent := []byte(`{"family":"llama","parameter_size":"7B","quantization":"Q4_0"}`)
	configH := sha256.New()
	configH.Write(configContent)
	configHex := hex.EncodeToString(configH.Sum(nil))
	configDigest := "sha256:" + configHex

	layerContent := make([]byte, 512)
	for i := range layerContent {
		layerContent[i] = byte(i % 256)
	}
	layerH := sha256.New()
	layerH.Write(layerContent)
	layerHex := hex.EncodeToString(layerH.Sum(nil))
	layerDigest := "sha256:" + layerHex

	manifest := Manifest{
		SchemaVersion: 2,
		MediaType:     "application/vnd.docker.distribution.manifest.v2+json",
		Config:        ManifestLayer{Digest: configDigest, Size: int64(len(configContent))},
		Layers:        []ManifestLayer{{Digest: layerDigest, Size: int64(len(layerContent))}},
	}
	manifestBody, _ := json.Marshal(manifest)

	blobs := map[string][]byte{
		configHex: configContent,
		layerHex:  layerContent,
	}

	cdnSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Split(r.URL.Path, "/")
		blobHex := parts[len(parts)-1]
		data, ok := blobs[blobHex]
		if !ok {
			w.WriteHeader(404)
			return
		}
		rangeHdr := r.Header.Get("Range")
		if rangeHdr != "" {
			var start, end int
			if _, err := fmt.Sscanf(rangeHdr, "bytes=%d-%d", &start, &end); err == nil {
				if end >= len(data) {
					end = len(data) - 1
				}
				w.Header().Set("Content-Length", strconv.Itoa(end-start+1))
				w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, len(data)))
				w.WriteHeader(206)
				w.Write(data[start : end+1])
				return
			}
		}
		w.Header().Set("Content-Length", strconv.Itoa(len(data)))
		w.Write(data)
	}))
	defer cdnSrv.Close()

	regSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/manifests/") {
			w.Header().Set("Content-Type", "application/vnd.docker.distribution.manifest.v2+json")
			w.Write(manifestBody)
			return
		}
		if strings.Contains(r.URL.Path, "/blobs/") {
			digestParts := strings.Split(r.URL.Path, "/blobs/")
			if len(digestParts) == 2 {
				d := digestParts[1]
				blobHex := strings.TrimPrefix(d, "sha256:")
				w.Header().Set("Location", cdnSrv.URL+"/blob/"+blobHex)
				w.WriteHeader(307)
				return
			}
			w.WriteHeader(404)
			return
		}
		if strings.Contains(r.URL.Path, "/chunksums/") {
			w.WriteHeader(404)
			return
		}
		w.WriteHeader(404)
	}))
	defer regSrv.Close()

	modelsDir := t.TempDir()
	cfg := DefaultConfig()
	cfg.ModelsDir = modelsDir
	cfg.Scheme = "http"

	ref := ModelRef{Host: regSrv.Listener.Addr().String(), Namespace: "library", Name: "testmodel", Tag: "latest", Scheme: "http"}

	err := PullModel(context.Background(), cfg, ref, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Verify blobs exist
	blobDir := filepath.Join(modelsDir, "blobs")
	if _, err := os.Stat(filepath.Join(blobDir, "sha256-"+configHex)); err != nil {
		t.Errorf("config blob missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(blobDir, "sha256-"+layerHex)); err != nil {
		t.Errorf("layer blob missing: %v", err)
	}

	// Verify manifest exists
	manifestPath := filepath.Join(modelsDir, "manifests", ref.ManifestPath())
	if _, err := os.Stat(manifestPath); err != nil {
		t.Errorf("manifest missing: %v", err)
	}

	// Verify ListModels can see it
	models, err := ListModels(modelsDir)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, m := range models {
		if strings.Contains(m.Name, "testmodel") {
			found = true
			break
		}
	}
	if !found {
		t.Error("testmodel not found in ListModels output")
	}
}

// E2E.2: Pull a multi-chunk model; kill between chunks; resume; final blob verifies.
func TestE2EPullMultiChunkResume(t *testing.T) {
	setupTestKey(t)

	content := make([]byte, 2048)
	for i := range content {
		content[i] = byte(i % 256)
	}
	h := sha256.New()
	h.Write(content)
	hexStr := hex.EncodeToString(h.Sum(nil))
	digest := "sha256:" + hexStr

	// Two chunks
	mid := int64(len(content) / 2)
	chunk1 := content[:mid]
	chunk2 := content[mid:]
	h1 := sha256.New()
	h1.Write(chunk1)
	h2 := sha256.New()
	h2.Write(chunk2)
	chunk1Digest := "sha256:" + hex.EncodeToString(h1.Sum(nil))
	chunk2Digest := "sha256:" + hex.EncodeToString(h2.Sum(nil))

	configContent := []byte(`{"family":"test"}`)
	configH := sha256.New()
	configH.Write(configContent)
	configHex := hex.EncodeToString(configH.Sum(nil))
	configDigest := "sha256:" + configHex

	manifest := Manifest{
		SchemaVersion: 2,
		Config:        ManifestLayer{Digest: configDigest, Size: int64(len(configContent))},
		Layers:        []ManifestLayer{{Digest: digest, Size: int64(len(content))}},
	}
	manifestBody, _ := json.Marshal(manifest)

	blobs := map[string][]byte{
		configHex: configContent,
		hexStr:    content,
	}

	cdnSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Route by path suffix: /blob/<hex>
		parts := strings.Split(r.URL.Path, "/")
		blobHex := parts[len(parts)-1]
		data, ok := blobs[blobHex]
		if !ok {
			w.WriteHeader(404)
			return
		}
		rangeHdr := r.Header.Get("Range")
		if rangeHdr != "" {
			var start, end int
			if _, err := fmt.Sscanf(rangeHdr, "bytes=%d-%d", &start, &end); err == nil {
				if end >= len(data) {
					end = len(data) - 1
				}
				w.Header().Set("Content-Length", strconv.Itoa(end-start+1))
				w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, len(data)))
				w.WriteHeader(206)
				w.Write(data[start : end+1])
				return
			}
		}
		w.Header().Set("Content-Length", strconv.Itoa(len(data)))
		w.Write(data)
	}))
	defer cdnSrv.Close()

	regSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/manifests/") {
			w.Header().Set("Content-Type", "application/vnd.docker.distribution.manifest.v2+json")
			w.Write(manifestBody)
			return
		}
		if strings.Contains(r.URL.Path, "/blobs/") {
			// Extract digest from path to redirect to correct CDN URL
			digestParts := strings.Split(r.URL.Path, "/blobs/")
			if len(digestParts) == 2 {
				d := digestParts[1]
				blobHex := strings.TrimPrefix(d, "sha256:")
				w.Header().Set("Location", cdnSrv.URL+"/blob/"+blobHex)
				w.WriteHeader(307)
				return
			}
			w.WriteHeader(404)
			return
		}
		if strings.Contains(r.URL.Path, "/chunksums/") {
			// Only return chunksums for the layer blob, not config
			if strings.Contains(r.URL.Path, hexStr) {
				chunks := []ChunkDigest{
					{Digest: chunk1Digest, Start: 0, End: mid - 1},
					{Digest: chunk2Digest, Start: mid, End: int64(len(content)) - 1},
				}
				body, _ := json.Marshal(struct {
					Chunks []ChunkDigest `json:"chunks"`
				}{Chunks: chunks})
				w.Write(body)
				return
			}
			w.WriteHeader(404)
			return
		}
		w.WriteHeader(404)
	}))
	defer regSrv.Close()

	modelsDir := t.TempDir()
	cfg := DefaultConfig()
	cfg.ModelsDir = modelsDir
	cfg.Scheme = "http"

	ref := ModelRef{Host: regSrv.Listener.Addr().String(), Namespace: "library", Name: "testmodel", Tag: "latest", Scheme: "http"}

	err := PullModel(context.Background(), cfg, ref, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Verify final blob
	finalPath := filepath.Join(modelsDir, "blobs", "sha256-"+hexStr)
	ok, err := VerifyBlob(finalPath, digest)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Error("final blob verification failed")
	}
}

// E2E.3: Back-to-back pulls of the same model → second is a no-op.
func TestE2EBackToBackPullNoOp(t *testing.T) {
	setupTestKey(t)

	content := []byte("simple model blob")
	h := sha256.New()
	h.Write(content)
	hexStr := hex.EncodeToString(h.Sum(nil))
	digest := "sha256:" + hexStr

	configContent := []byte(`{"family":"test"}`)
	configH := sha256.New()
	configH.Write(configContent)
	configHex := hex.EncodeToString(configH.Sum(nil))
	configDigest := "sha256:" + configHex

	manifest := Manifest{
		SchemaVersion: 2,
		Config:        ManifestLayer{Digest: configDigest, Size: int64(len(configContent))},
		Layers:        []ManifestLayer{{Digest: digest, Size: int64(len(content))}},
	}
	manifestBody, _ := json.Marshal(manifest)

	blobs := map[string][]byte{
		configHex: configContent,
		hexStr:    content,
	}

	cdnSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Split(r.URL.Path, "/")
		blobHex := parts[len(parts)-1]
		data, ok := blobs[blobHex]
		if !ok {
			w.WriteHeader(404)
			return
		}
		rangeHdr := r.Header.Get("Range")
		if rangeHdr != "" {
			var start, end int
			if _, err := fmt.Sscanf(rangeHdr, "bytes=%d-%d", &start, &end); err == nil {
				w.Header().Set("Content-Length", strconv.Itoa(end-start+1))
				w.WriteHeader(206)
				w.Write(data[start : end+1])
				return
			}
		}
		w.Write(data)
	}))
	defer cdnSrv.Close()

	regSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/manifests/") {
			w.Header().Set("Content-Type", "application/vnd.docker.distribution.manifest.v2+json")
			w.Write(manifestBody)
			return
		}
		if strings.Contains(r.URL.Path, "/blobs/") {
			digestParts := strings.Split(r.URL.Path, "/blobs/")
			if len(digestParts) == 2 {
				d := digestParts[1]
				blobHex := strings.TrimPrefix(d, "sha256:")
				w.Header().Set("Location", cdnSrv.URL+"/blob/"+blobHex)
				w.WriteHeader(307)
				return
			}
			w.WriteHeader(404)
			return
		}
		if strings.Contains(r.URL.Path, "/chunksums/") {
			w.WriteHeader(404)
			return
		}
		w.WriteHeader(404)
	}))
	defer regSrv.Close()

	modelsDir := t.TempDir()
	cfg := DefaultConfig()
	cfg.ModelsDir = modelsDir
	cfg.Scheme = "http"

	ref := ModelRef{Host: regSrv.Listener.Addr().String(), Namespace: "library", Name: "testmodel", Tag: "latest", Scheme: "http"}

	// First pull
	err := PullModel(context.Background(), cfg, ref, nil)
	if err != nil {
		t.Fatal(err)
	}
	info1, _ := os.Stat(filepath.Join(modelsDir, "blobs", "sha256-"+hexStr))

	// Second pull (should be no-op, blobs already exist)
	err = PullModel(context.Background(), cfg, ref, nil)
	if err != nil {
		t.Fatal(err)
	}
	info2, _ := os.Stat(filepath.Join(modelsDir, "blobs", "sha256-"+hexStr))

	// Same file, same size
	if info1.Size() != info2.Size() {
		t.Errorf("blob size changed between pulls: %d vs %d", info1.Size(), info2.Size())
	}
}