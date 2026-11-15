package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRulesPersistAndValidate(t *testing.T) {
	file := filepath.Join(t.TempDir(), "settings.db")
	store, err := openRules(file)
	if err != nil {
		t.Fatal(err)
	}
	if err = store.Save([]string{"node_modules", "*.log", ".env.*", "file?.txt"}); err != nil {
		t.Fatal(err)
	}
	store.db.Close()
	store, err = openRules(file)
	if err != nil {
		t.Fatal(err)
	}
	defer store.db.Close()
	rules, err := store.List()
	if err != nil || len(rules) != 4 {
		t.Fatal(rules, err)
	}
	for _, name := range []string{"node_modules", "debug.log", ".env.local", "file1.txt"} {
		if !matchesName(name, rules) {
			t.Errorf("not matched: %s", name)
		}
	}
	if matchesName("main.go", rules) {
		t.Fatal("unrelated file excluded")
	}
	if err = store.Save([]string{"["}); err == nil {
		t.Fatal("invalid pattern accepted")
	}
	if err = store.Save([]string{"../secret"}); err == nil {
		t.Fatal("path accepted")
	}
	rules, _ = store.List()
	if len(rules) != 4 {
		t.Fatal("invalid update changed saved rules")
	}
	if err = store.Save(nil); err != nil {
		t.Fatal(err)
	}
	store.db.Close()
	store, err = openRules(file)
	if err != nil {
		t.Fatal(err)
	}
	defer store.db.Close()
	rules, err = store.List()
	if err != nil || len(rules) != 0 {
		t.Fatal("empty preferences were reset", rules, err)
	}
}

func TestLazyTreeAndFullGeneration(t *testing.T) {
	root := t.TempDir()
	for _, dir := range []string{"src/deep", "node_modules/huge", "src/node_modules/nested"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0700); err != nil {
			t.Fatal(err)
		}
	}
	os.WriteFile(filepath.Join(root, "src/deep/code.txt"), []byte("deep content"), 0600)
	os.WriteFile(filepath.Join(root, "src/debug.log"), []byte("excluded log"), 0600)
	rules := []string{"node_modules", "*.log"}
	level, err := readLevel(root, "", rules)
	if err != nil {
		t.Fatal(err)
	}
	if len(level.Children) != 1 || level.Children[0].Children != nil {
		t.Fatal("eager descendants or rules not applied")
	}
	full, err := scanSelected(root, rules, nil)
	if err != nil {
		t.Fatal(err)
	}
	text, err := report(root, full, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(text, "deep content") || strings.Contains(text, "node_modules") || strings.Contains(text, "debug.log") {
		t.Fatal("full walk failed")
	}
	full, err = scanSelected(root, rules, []string{"src"})
	if err != nil || len(full.Children) != 0 {
		t.Fatal("manual exclusions not pruned", err)
	}
	if _, err = readLevel(root, "../", nil); err == nil {
		t.Fatal("path escape accepted")
	}
	// Large source files are not read while browsing and no longer skipped at 2 MiB.
	body := strings.Repeat("x", maxFile+1)
	os.WriteFile(filepath.Join(root, "big.txt"), []byte(body), 0600)
	data, err := safeRead(root, "big.txt")
	if err != nil || len(data) != len(body) {
		t.Fatal("large text incorrectly skipped", err)
	}
}
