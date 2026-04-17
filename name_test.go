package main

import (
	"testing"
)

func TestParseModelRef(t *testing.T) {
	ref, err := ParseModelRef("llama3.2")
	if err != nil {
		t.Fatal(err)
	}
	if ref.Host != "registry.ollama.ai" || ref.Namespace != "library" || ref.Name != "llama3.2" || ref.Tag != "latest" || ref.Scheme != "https" {
		t.Errorf("got %+v", ref)
	}

	ref, err = ParseModelRef("mistral:7b")
	if err != nil {
		t.Fatal(err)
	}
	if ref.Tag != "7b" || ref.Name != "mistral" {
		t.Errorf("got %+v", ref)
	}

	ref, err = ParseModelRef("user/my-model")
	if err != nil {
		t.Fatal(err)
	}
	if ref.Namespace != "user" || ref.Name != "my-model" {
		t.Errorf("got %+v", ref)
	}

	_, err = ParseModelRef("")
	if err != ErrInvalidModelRef {
		t.Errorf("empty input: got %v, want ErrInvalidModelRef", err)
	}

	_, err = ParseModelRef("host/ns/model:tag:extra")
	if err != ErrInvalidModelRef {
		t.Errorf("multi-colon: got %v, want ErrInvalidModelRef", err)
	}
}

func TestBlobPath(t *testing.T) {
	ref, _ := ParseModelRef("test")
	got := ref.BlobPath("sha256:abc")
	if got != "sha256-abc" {
		t.Errorf("BlobPath = %q, want %q", got, "sha256-abc")
	}
}

func TestModelRefRoundTrip(t *testing.T) {
	ref, err := ParseModelRef("llama3.2")
	if err != nil {
		t.Fatal(err)
	}
	s := ref.String()
	if s != "https://registry.ollama.ai/library/llama3.2:latest" {
		t.Errorf("String() = %q", s)
	}
}