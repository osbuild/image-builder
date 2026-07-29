package main

import (
	"io"
	"os"
	"path/filepath"
)

func fileWriter(outputDir, filename string, content io.Reader) error {
	p := filepath.Join(outputDir, filename)
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		return err
	}
	f, err := os.Create(p)
	if err != nil {
		return err
	}
	defer f.Close()

	if _, err := io.Copy(f, content); err != nil {
		return err
	}

	return f.Sync()
}
