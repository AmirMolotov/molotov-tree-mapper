package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReport(t *testing.T) {
	root := t.TempDir()
	os.Mkdir(filepath.Join(root, "src"), 0700)
	os.WriteFile(filepath.Join(root, "src", "main.go"), []byte("package main\n// ``` fence"), 0600)
	os.WriteFile(filepath.Join(root, "secret.txt"), []byte("do not share"), 0600)
	os.WriteFile(filepath.Join(root, "binary"), []byte{0, 1, 2}, 0600)
	tree, err := scan(root)
	if err != nil {
		t.Fatal(err)
	}
	text, err := report(root, tree, []string{"secret.txt"})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"├── src/", "main.go", "package main", "````", "binary or non-UTF-8"} {
		if !strings.Contains(text, want) {
			t.Errorf("missing %q", want)
		}
	}
	if strings.Contains(text, "do not share") || strings.Contains(text, "secret.txt") {
		t.Fatal("excluded file leaked")
	}
	text, err = report(root, tree, []string{"src"})
	if err != nil || strings.Contains(text, "package main") {
		t.Fatal("directory exclusion failed", err)
	}
	text, err = report(root, tree, []string{""})
	if err != nil || strings.Contains(text, "main.go") {
		t.Fatal("root exclusion failed", err)
	}
}

func TestLimitsAndChangedFiles(t *testing.T) {
	root := t.TempDir()
	p := filepath.Join(root, "large.txt")
	os.WriteFile(p, make([]byte, maxReport+1), 0600)
	if _, err := safeRead(root, "large.txt"); err == nil {
		t.Fatal("large file accepted")
	}
	tree, err := scan(root)
	if err != nil {
		t.Fatal(err)
	}
	os.Remove(p)
	text, err := report(root, tree, nil)
	if err != nil || !strings.Contains(text, "[Skipped:") {
		t.Fatal("missing file not labelled", err)
	}
	if _, err = scan(filepath.Join(root, "missing")); err == nil {
		t.Fatal("missing root accepted")
	}
}

func TestSymlinkNotFollowed(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	os.WriteFile(filepath.Join(outside, "secret"), []byte("outside"), 0600)
	if err := os.Symlink(outside, filepath.Join(root, "link")); err != nil {
		t.Skip("Symlink privilege unavailable")
	}
	tree, err := scan(root)
	if err != nil {
		t.Fatal(err)
	}
	text, err := report(root, tree, nil)
	if err != nil || strings.Contains(text, "### link/secret") {
		t.Fatal("symlink followed", err)
	}
}
