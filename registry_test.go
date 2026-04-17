package main

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func setupTestKey(t *testing.T) string {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		t.Fatal(err)
	}
	block := &pem.Block{Type: "PRIVATE KEY", Bytes: der}
	home := t.TempDir()
	keyDir := filepath.Join(home, ".ollama")
	os.MkdirAll(keyDir, 0o755)
	os.WriteFile(filepath.Join(keyDir, "id_ed25519"), pem.EncodeToMemory(block), 0o600)
	// Override HOME for defaultKeyPath
	t.Setenv("HOME", home)
	return home
}

func TestFetchManifest(t *testing.T) {
	setupTestKey(t)
	manifest := Manifest{
		SchemaVersion: 2,
		MediaType:     "application/vnd.docker.distribution.manifest.v2+json",
		Config:        ManifestLayer{Digest: "sha256:config", Size: 100},
		Layers:        []ManifestLayer{{Digest: "sha256:layer1", Size: 1000}},
	}
	body, _ := json.Marshal(manifest)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v2/library/test/manifests/latest" {
			w.Header().Set("Content-Type", "application/vnd.docker.distribution.manifest.v2+json")
			w.Write(body)
			return
		}
		w.WriteHeader(404)
	}))
	defer srv.Close()

	ref := ModelRef{Host: srv.Listener.Addr().String(), Namespace: "library", Name: "test", Tag: "latest", Scheme: "http"}
	cfg := DefaultConfig()
	cfg.Scheme = "http"

	m, err := FetchManifest(context.Background(), cfg, ref)
	if err != nil {
		t.Fatal(err)
	}
	if m.SchemaVersion != 2 {
		t.Errorf("schema = %d, want 2", m.SchemaVersion)
	}
}

func TestFetchManifest404(t *testing.T) {
	setupTestKey(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
	}))
	defer srv.Close()

	ref := ModelRef{Host: srv.Listener.Addr().String(), Namespace: "library", Name: "test", Tag: "latest", Scheme: "http"}
	cfg := DefaultConfig()
	cfg.Scheme = "http"

	_, err := FetchManifest(context.Background(), cfg, ref)
	if err != ErrNotFound {
		t.Errorf("got %v, want ErrNotFound", err)
	}
}

func TestFetchManifestNoCache(t *testing.T) {
	setupTestKey(t)
	count := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count++
		manifest := Manifest{SchemaVersion: 2}
		body, _ := json.Marshal(manifest)
		w.Header().Set("Content-Type", "application/vnd.docker.distribution.manifest.v2+json")
		w.Write(body)
	}))
	defer srv.Close()

	ref := ModelRef{Host: srv.Listener.Addr().String(), Namespace: "library", Name: "test", Tag: "latest", Scheme: "http"}
	cfg := DefaultConfig()
	cfg.Scheme = "http"

	FetchManifest(context.Background(), cfg, ref)
	FetchManifest(context.Background(), cfg, ref)

	if count != 2 {
		t.Errorf("expected 2 requests, got %d (manifest should not be cached)", count)
	}
}

func TestResolveBlobURL307(t *testing.T) {
	setupTestKey(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v2/library/test/blobs/sha256:abc" {
			w.Header().Set("Location", "https://cdn.example.com/blob")
			w.WriteHeader(307)
			return
		}
		w.WriteHeader(404)
	}))
	defer srv.Close()

	ref := ModelRef{Host: srv.Listener.Addr().String(), Namespace: "library", Name: "test", Tag: "latest", Scheme: "http"}
	cfg := DefaultConfig()
	cfg.Scheme = "http"

	url, err := ResolveBlobURL(context.Background(), cfg, ref, "sha256:abc")
	if err != nil {
		t.Fatal(err)
	}
	if url != "https://cdn.example.com/blob" {
		t.Errorf("got %q, want CDN URL", url)
	}
}

func TestFetchChunksums404(t *testing.T) {
	setupTestKey(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
	}))
	defer srv.Close()

	ref := ModelRef{Host: srv.Listener.Addr().String(), Namespace: "library", Name: "test", Tag: "latest", Scheme: "http"}
	cfg := DefaultConfig()
	cfg.Scheme = "http"

	chunks, err := FetchChunksums(context.Background(), cfg, ref, "sha256:abc")
	if err != nil {
		t.Fatal(err)
	}
	if chunks != nil {
		t.Errorf("expected nil for 404 chunksums, got %v", chunks)
	}
}