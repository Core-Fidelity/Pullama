package main

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func generateTestEd25519Key(t *testing.T) (string, ed25519.PrivateKey) {
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
	keyPath := filepath.Join(t.TempDir(), "id_ed25519")
	if err := os.WriteFile(keyPath, pem.EncodeToMemory(block), 0o600); err != nil {
		t.Fatal(err)
	}
	return keyPath, priv
}

func TestMakeAuthTokenFormat(t *testing.T) {
	keyPath, _ := generateTestEd25519Key(t)
	token, err := MakeAuthToken(context.Background(), keyPath, "https://registry.ollama.ai/v2/library/llama3.2/manifests/latest")
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(token, ":")
	if len(parts) != 3 {
		t.Errorf("token has %d parts, want 3: %q", len(parts), token)
	}
}

func TestMakeAuthTokenVerifies(t *testing.T) {
	keyPath, priv := generateTestEd25519Key(t)
	url := "https://registry.ollama.ai/v2/library/llama3.2/manifests/latest"

	token, err := MakeAuthToken(context.Background(), keyPath, url)
	if err != nil {
		t.Fatal(err)
	}

	parts := strings.Split(token, ":")
	if len(parts) != 3 {
		t.Fatal("token format wrong")
	}

	pubBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		t.Fatal(err)
	}
	sigBytes, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		t.Fatal(err)
	}

	pub := priv.Public().(ed25519.PublicKey)
	if string(pubBytes) != string(pub) {
		t.Error("pubkey in token does not match private key")
	}
	if !ed25519.Verify(pub, []byte(url), sigBytes) {
		t.Error("signature does not verify")
	}
}

func TestMakeAuthTokenMissingKey(t *testing.T) {
	_, err := MakeAuthToken(context.Background(), "/nonexistent/key", "url")
	if err == nil {
		t.Error("expected error for missing key")
	}
}