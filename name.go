package main

import (
	"regexp"
	"strings"
)

type ModelRef struct {
	Host      string
	Namespace string
	Name      string
	Tag       string
	Scheme    string
}

var modelRe = regexp.MustCompile(`^[a-z0-9._/:-]+$`)

func ParseModelRef(s string) (ModelRef, error) {
	s = strings.TrimSpace(s)
	s = strings.ToLower(s)
	if s == "" || !modelRe.MatchString(s) {
		return ModelRef{}, ErrInvalidModelRef
	}
	// Reject multi-colon tag sections
	if colonCount := strings.Count(s, ":"); colonCount > 1 {
		return ModelRef{}, ErrInvalidModelRef
	}
	ref := ModelRef{
		Host:      "registry.ollama.ai",
		Namespace: "library",
		Tag:       "latest",
		Scheme:    "https",
	}
	// Split host
	if strings.Contains(s, "/") {
		parts := strings.SplitN(s, "/", 3)
		switch len(parts) {
		case 2:
			ref.Namespace = parts[0]
			s = parts[1]
		case 3:
			ref.Host = parts[0]
			ref.Namespace = parts[1]
			s = parts[2]
		}
	}
	// Split tag
	if idx := strings.LastIndex(s, ":"); idx >= 0 {
		ref.Tag = s[idx+1:]
		s = s[:idx]
	}
	if s == "" {
		return ModelRef{}, ErrInvalidModelRef
	}
	ref.Name = s
	return ref, nil
}

func (r ModelRef) String() string {
	return r.Scheme + "://" + r.Host + "/" + r.Namespace + "/" + r.Name + ":" + r.Tag
}

func (r ModelRef) ManifestPath() string {
	return r.Host + "/" + r.Namespace + "/" + r.Name + "/" + r.Tag
}

func (r ModelRef) BlobPath(digest string) string {
	// sha256:<hex> → sha256-<hex> (INV-13)
	return strings.Replace(digest, ":", "-", 1)
}