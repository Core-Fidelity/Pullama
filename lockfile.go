package main

import (
	"os"
)

type Lock struct {
	fd   int
	path string
}

func (l *Lock) Release() error {
	err := releaseLock(l.fd)
	os.Remove(l.path) // best-effort; ignore NotExist
	return err
}