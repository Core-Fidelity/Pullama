package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

type Manifest struct {
	SchemaVersion int             `json:"schemaVersion"`
	MediaType     string          `json:"mediaType"`
	Config        ManifestLayer   `json:"config"`
	Layers        []ManifestLayer `json:"layers"`
}

type ManifestLayer struct {
	MediaType string `json:"mediaType"`
	Digest    string `json:"digest"`
	Size      int64  `json:"size"`
}

func noRedirect() func(*http.Request, []*http.Request) error {
	return func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
}

func makeClient(cfg *Config) *http.Client {
	return &http.Client{Timeout: cfg.Timeout, CheckRedirect: noRedirect()}
}

func FetchManifest(ctx context.Context, cfg *Config, ref ModelRef) (*Manifest, error) {
	url := fmt.Sprintf("%s://%s/v2/%s/%s/manifests/%s", ref.Scheme, ref.Host, ref.Namespace, ref.Name, ref.Tag)
	return fetchManifestRetry(ctx, cfg, ref, url, 0)
}

func fetchManifestRetry(ctx context.Context, cfg *Config, ref ModelRef, url string, refreshes int) (*Manifest, error) {
	token, err := MakeAuthToken(ctx, defaultKeyPath(), url)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.docker.distribution.manifest.v2+json")

	resp, err := makeClient(cfg).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case 200:
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}
		var m Manifest
		if err := json.Unmarshal(body, &m); err != nil {
			return nil, ErrManifestCorrupt
		}
		return &m, nil
	case 401:
		if refreshes < 1 {
			return fetchManifestRetry(ctx, cfg, ref, url, refreshes+1)
		}
		return nil, ErrAuthFailed
	case 404:
		return nil, ErrNotFound
	default:
		return nil, fmt.Errorf("manifest fetch: HTTP %d", resp.StatusCode)
	}
}

func ResolveBlobURL(ctx context.Context, cfg *Config, ref ModelRef, digest string) (string, error) {
	url := fmt.Sprintf("%s://%s/v2/%s/%s/blobs/%s", ref.Scheme, ref.Host, ref.Namespace, ref.Name, digest)
	token, err := MakeAuthToken(ctx, defaultKeyPath(), url)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := makeClient(cfg).Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case 307:
		if loc := resp.Header.Get("Location"); loc != "" {
			return loc, nil
		}
		return "", fmt.Errorf("307 with no Location header")
	case 404:
		return "", ErrNotFound
	default:
		return "", fmt.Errorf("resolve blob URL: HTTP %d", resp.StatusCode)
	}
}

func FetchChunksums(ctx context.Context, cfg *Config, ref ModelRef, digest string) ([]ChunkDigest, error) {
	url := fmt.Sprintf("%s://%s/v2/%s/%s/chunksums/%s", ref.Scheme, ref.Host, ref.Namespace, ref.Name, digest)
	token, err := MakeAuthToken(ctx, defaultKeyPath(), url)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := makeClient(cfg).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case 404:
		return nil, nil
	case 200:
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}
		return parseChunksums(body)
	default:
		return nil, fmt.Errorf("chunksums fetch: HTTP %d", resp.StatusCode)
	}
}

func defaultKeyPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".ollama", "id_ed25519")
}

// parseChunksums parses the plain-text chunksums format:
// each line: "<digest> <start>-<end>"
func parseChunksums(data []byte) ([]ChunkDigest, error) {
	var chunks []ChunkDigest
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, " ", 2)
		if len(parts) != 2 {
			continue
		}
		digest := parts[0]
		rangeStr := parts[1]
		dashIdx := strings.Index(rangeStr, "-")
		if dashIdx < 0 {
			continue
		}
		start, err := strconv.ParseInt(rangeStr[:dashIdx], 10, 64)
		if err != nil {
			continue
		}
		end, err := strconv.ParseInt(rangeStr[dashIdx+1:], 10, 64)
		if err != nil {
			continue
		}
		chunks = append(chunks, ChunkDigest{Digest: digest, Start: start, End: end})
	}
	sort.Slice(chunks, func(i, j int) bool { return chunks[i].Start < chunks[j].Start })
	return chunks, nil
}