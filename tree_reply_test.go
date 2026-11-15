package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTreeReplyCreatesNewAndChangesExistingFiles(t *testing.T) {
	text := "# project\n\n## Directory tree\n\nproject/\n├── src/\n│   └── new.go\n└── old.go\n\n## File contents\n\n### src/new.go\n\n```go\npackage newfile\n```\n\n### old.go\n\n```go\npackage updated\n```"
	p, err := parseReport(text)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(p.Files, ",") != "src/new.go,old.go" {
		t.Fatal("tree order lost", p.Files)
	}
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "old.go"), []byte("package original"), 0600)
	changes := snapshot{}
	for name, body := range p.Contents {
		changes[name] = str(body)
	}
	id, err := versionedChange(root, "Tree reply", changes, p.Files)
	if err != nil {
		t.Fatal(err)
	}
	for name, want := range p.Contents {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(name)))
		if err != nil || string(data) != want {
			t.Fatal(name, err)
		}
	}
	h, err := loadHistory(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range h.Versions {
		if v.ID == id {
			statuses := map[string]string{}
			for _, c := range v.Changes {
				statuses[c.Path] = c.Status
			}
			if statuses["src/new.go"] != "Added" || statuses["old.go"] != "Modified" {
				t.Fatal(statuses)
			}
		}
	}
	// Sections must correspond exactly to tree paths and order; no stale JSON manifests are needed.
	reversed := strings.Replace(text, "### src/new.go", "### old.go", 1)
	if _, err = parseReport(reversed); err == nil {
		t.Fatal("out-of-order contents accepted")
	}
	if strings.Contains(llmInstructions, "return one JSON") || !strings.Contains(llmInstructions, "## Directory tree") {
		t.Fatal("prompt is not tree-first")
	}
}
