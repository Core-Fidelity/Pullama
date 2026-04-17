package main

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"fmt"
	"os"

	"golang.org/x/crypto/ssh"
)

// MakeAuthToken produces an Ed25519 bearer token from an OpenSSH private key file.
// Format: base64(url):base64(pubkey):base64(signature)
func MakeAuthToken(ctx context.Context, keyPath, url string) (string, error) {
	keyData, err := os.ReadFile(keyPath)
	if err != nil {
		return "", fmt.Errorf("%w: cannot read key %s: %v", ErrAuthFailed, keyPath, err)
	}

	signer, err := ssh.ParsePrivateKey(keyData)
	if err != nil {
		return "", fmt.Errorf("%w: cannot parse key %s: %v", ErrAuthFailed, keyPath, err)
	}

	// Extract the Ed25519 public key bytes
	pubKey, ok := signer.PublicKey().(ssh.CryptoPublicKey)
	if !ok {
		return "", fmt.Errorf("%w: key is not Ed25519", ErrAuthFailed)
	}
	cryptoPub := pubKey.CryptoPublicKey()
	ed25519Pub, ok := cryptoPub.(ed25519.PublicKey)
	if !ok {
		return "", fmt.Errorf("%w: key is not Ed25519", ErrAuthFailed)
	}

	// Sign the URL
	sig, err := signer.Sign(nil, []byte(url))
	if err != nil {
		return "", fmt.Errorf("%w: signing failed: %v", ErrAuthFailed, err)
	}

	token := base64.RawURLEncoding.EncodeToString([]byte(url)) + ":" +
		base64.RawURLEncoding.EncodeToString(ed25519Pub) + ":" +
		base64.RawURLEncoding.EncodeToString(sig.Blob)

	return token, nil
}