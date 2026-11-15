package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func str(s string) *string { return &s }
func TestVersionUpdatesAndFileRevert(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "existing.txt"), []byte("original"), 0600)
	first, err := versionedChange(root, "Import 1", snapshot{"existing.txt": str("one"), "src/new.txt": str("new")})
	if err != nil {
		t.Fatal(err)
	}
	_, err = versionedChange(root, "Import 2", snapshot{"existing.txt": str("two"), "later.txt": str("later")})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = revertVersion(root, first, "existing.txt"); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(root, "existing.txt"))
	if string(data) != "one" {
		t.Fatal("file revert failed")
	}
	data, _ = os.ReadFile(filepath.Join(root, "later.txt"))
	if string(data) != "later" {
		t.Fatal("file revert changed another file")
	}
	if _, err = revertVersion(root, first, ""); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(filepath.Join(root, "later.txt")); !os.IsNotExist(err) {
		t.Fatal("full revert did not remove later managed file")
	}
	os.WriteFile(filepath.Join(root, "unrelated.txt"), []byte("keep"), 0600)
	h, err := loadHistory(root)
	if err != nil || len(h.Versions) < 6 {
		t.Fatal(h, err)
	}
	for _, v := range h.Versions {
		if v.ID == first {
			statuses := map[string]string{}
			for _, c := range v.Changes {
				statuses[c.Path] = c.Status
			}
			if statuses["existing.txt"] != "Modified" || statuses["src/new.txt"] != "Added" {
				t.Fatal("missing new/modified classifications", statuses)
			}
		}
	}
	db, _ := openHistory(root)
	before, err := readSnapshot(db, 1)
	db.Close()
	if err != nil || *before["existing.txt"] != "original" {
		t.Fatal("baseline lost", err)
	}
	tree, err := scan(root)
	if err != nil {
		t.Fatal(err)
	}
	text, err := report(root, tree, nil)
	if err != nil || strings.Contains(text, historyName) {
		t.Fatal("history database leaked into export", err)
	}
}
func TestInterruptedHistoryRecovery(t *testing.T) {
	root := t.TempDir()
	id, err := versionedChange(root, "Initial", snapshot{"a.txt": str("old")})
	if err != nil {
		t.Fatal(err)
	}
	db, _ := openHistory(root)
	db.Exec(`INSERT INTO versions(label,snapshot) VALUES ('interrupted',?)`, `{"a.txt":"new"}`)
	db.Exec(`INSERT INTO pending(id,before_id,after_id) VALUES(1,?,?)`, id, id+1)
	db.Close()
	os.WriteFile(filepath.Join(root, "a.txt"), []byte("new"), 0600)
	if _, err = versionedChange(root, "blocked", snapshot{"a.txt": str("other")}); err == nil {
		t.Fatal("pending update not blocked")
	}
	if err = recoverHistory(root); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(root, "a.txt"))
	if string(data) != "old" {
		t.Fatal("recovery failed")
	}
	if _, err = versionedChange(root, "unsafe", snapshot{"../outside": str("bad")}); err == nil {
		t.Fatal("unsafe versioned path accepted")
	}
	if _, err = versionedChange(root, "unsafe", snapshot{historyName: str("bad")}); err == nil {
		t.Fatal("history overwrite accepted")
	}
}

func TestHistoryPreservesExternalEditsAndRejectsCaseCollisions(t *testing.T) {
	root := t.TempDir()
	id, err := versionedChange(root, "Initial", snapshot{"a.txt": str("old")})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = versionedChange(root, "case", snapshot{"A.txt": str("conflict")}); err == nil {
		t.Fatal("case-conflicting history path accepted")
	}
	db, _ := openHistory(root)
	db.Exec(`INSERT INTO versions(label,snapshot) VALUES ('interrupted',?)`, `{"a.txt":"new"}`)
	db.Exec(`INSERT INTO pending(id,before_id,after_id) VALUES(1,?,?)`, id, id+1)
	db.Close()
	os.WriteFile(filepath.Join(root, "a.txt"), []byte("external edit"), 0600)
	if err = recoverHistory(root); err == nil {
		t.Fatal("recovery overwrote external edit")
	}
	data, _ := os.ReadFile(filepath.Join(root, "a.txt"))
	if string(data) != "external edit" {
		t.Fatal("external edit lost")
	}
}
func TestSettingsAndLLMFormat(t *testing.T) {
	file := filepath.Join(t.TempDir(), "settings.db")
	store, err := openRules(file)
	if err != nil {
		t.Fatal(err)
	}
	if err = store.SaveSettings(Settings{1}); err != nil {
		t.Fatal(err)
	}
	store.db.Close()
	store, _ = openRules(file)
	defer store.db.Close()
	got, err := store.Settings()
	if err != nil || got.MaxReportMiB != 1 {
		t.Fatal(got, err)
	}
	if err = store.SaveSettings(Settings{-1}); err == nil {
		t.Fatal("invalid size accepted")
	}
	text := "# project\n\n## Directory tree\n\nproject/\n└── src/\n    └── main.go\n\n## File contents\n\n### src/main.go\n\n```go\npackage main\n```\n"
	plan, err := parseReport(text)
	if err != nil || plan.Contents["src/main.go"] != "package main" || len(plan.Dirs) != 1 {
		t.Fatal(plan, err)
	}
	for _, bad := range []string{strings.Replace(text, "main.go", "../bad", 1), strings.Replace(text, "package main\n```\n", "", 1), `{"format":"molotov-llm-v1"}`} {
		if _, err = parseReport(bad); err == nil {
			t.Fatal("invalid LLM reply accepted")
		}
	}
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "a.txt"), []byte(strings.Repeat("a", 1024*1024+1)), 0600)
	tree, _ := scan(root)
	if _, err = reportLimited(root, tree, nil, 1024*1024); err == nil {
		t.Fatal("report size limit ignored")
	}
	text, err = reportLimited(root, tree, nil, 3*1024*1024)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = parseReport(text + llmPromptStart + llmInstructions + llmPromptEnd); err != nil {
		t.Fatal("prompt prevented restore", err)
	}
}

func TestAppGenerationUsesSettingsAndPrompt(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "a.txt"), []byte("content"), 0600)
	store, err := openRules(filepath.Join(t.TempDir(), "settings.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.db.Close()
	a := &App{store: store}
	if _, err = a.LoadFolder(root); err != nil {
		t.Fatal(err)
	}
	text, err := a.Generate(nil, nil, true)
	if err != nil || !strings.Contains(text, llmInstructions) {
		t.Fatal("prompt not generated", err)
	}
	if _, err = parseReport(text); err != nil {
		t.Fatal(err)
	}
	if err = a.SaveSettings(Settings{1}); err != nil {
		t.Fatal(err)
	}
	if err = a.checkOutputSize(strings.Repeat("a", 1024*1024+1)); err == nil {
		t.Fatal("edited output limit ignored")
	}
	os.WriteFile(filepath.Join(root, "large.txt"), []byte(strings.Repeat("a", 2*1024*1024)), 0600)
	if _, err = a.LoadFolder(root); err != nil {
		t.Fatal("opening a large source must not read contents", err)
	}
	if _, err = a.Generate(nil, nil, false); err == nil {
		t.Fatal("small report limit should fail")
	}
	if err = a.SaveSettings(Settings{3}); err != nil {
		t.Fatal(err)
	}
	if _, err = a.Generate(nil, nil, false); err != nil {
		t.Fatal("new setting was not applied to the already-open folder", err)
	}
}

func TestRevertToEmptyFirstVersion(t *testing.T) {
	root := t.TempDir()
	if _, err := versionedChange(root, "First import", snapshot{"src/deep/new.txt": str("new"), "top.txt": str("top")}); err != nil {
		t.Fatal(err)
	}
	if _, err := versionedChange(root, "Second import", snapshot{"later.txt": str("later")}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "untracked.txt"), []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := revertVersion(root, 1, ""); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"src/deep/new.txt", "top.txt", "later.txt"} {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(name))); !os.IsNotExist(err) {
			t.Fatalf("file %s remains after reverting to empty version: %v", name, err)
		}
	}
	if data, err := os.ReadFile(filepath.Join(root, "untracked.txt")); err != nil || string(data) != "keep" {
		t.Fatalf("untracked file changed: %q, %v", data, err)
	}
	// History intentionally retains directories and its database.
	if _, err := revertVersion(root, 4, ""); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"src/deep/new.txt", "top.txt", "later.txt"} {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(name))); err != nil {
			t.Fatalf("cannot undo empty revert for %s: %v", name, err)
		}
	}
}

func TestHistoryDirectoryRoundTrip(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "existing"), 0700); err != nil {
		t.Fatal(err)
	}
	populated, err := versionedChange(root, "Restore", snapshot{"src/deep/a.txt": str("exact\r\ncontent"), "empty/": str(""), "existing/new.txt": str("new")})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = revertVersion(root, 1, ""); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"src", "empty", "existing/new.txt"} {
		if _, err = os.Stat(filepath.Join(root, filepath.FromSlash(name))); !os.IsNotExist(err) {
			t.Fatalf("created path %s remains: %v", name, err)
		}
	}
	if st, err := os.Stat(filepath.Join(root, "existing")); err != nil || !st.IsDir() {
		t.Fatal("preexisting directory lost", err)
	}
	if _, err = revertVersion(root, populated, ""); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(filepath.Join(root, "src", "deep", "a.txt")); err != nil || string(data) != "exact\r\ncontent" {
		t.Fatalf("populated version not restored: %q, %v", data, err)
	}
	if st, err := os.Stat(filepath.Join(root, "empty")); err != nil || !st.IsDir() {
		t.Fatal("empty restored directory missing", err)
	}
	if err := os.WriteFile(filepath.Join(root, "src", "keep.txt"), []byte("untracked"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = revertVersion(root, 1, ""); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(filepath.Join(root, "src", "keep.txt")); err != nil || string(data) != "untracked" {
		t.Fatal("untracked content lost", err)
	}
}

func TestHistoryFileRevertKeepsExistingParent(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "existing"), 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := versionedChange(root, "Import", snapshot{"existing/a.txt": str("a")}); err != nil {
		t.Fatal(err)
	}
	if _, err := revertVersion(root, 1, "existing/a.txt"); err != nil {
		t.Fatal(err)
	}
	if st, err := os.Stat(filepath.Join(root, "existing")); err != nil || !st.IsDir() {
		t.Fatal("file-only revert removed parent", err)
	}
}

func TestHistoryDirectoryRecoveryAndLegacyRestore(t *testing.T) {
	root := t.TempDir()
	id, err := versionedChange(root, "Import", snapshot{"nested/a.txt": str("a"), "empty/": str("")})
	if err != nil {
		t.Fatal(err)
	}
	db, err := openHistory(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`INSERT INTO versions(label,snapshot) VALUES ('interrupted',?)`, `{"nested/a.txt":null,"nested/":null,"empty/":null}`); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`INSERT INTO pending(id,before_id,after_id) VALUES(1,?,?)`, id, id+1); err != nil {
		t.Fatal(err)
	}
	db.Close()
	if err = applySnapshot(root, snapshot{"nested/a.txt": nil, "nested/": nil, "empty/": nil}, snapshot{"nested/a.txt": str("a"), "nested/": str(""), "empty/": str("")}); err != nil {
		t.Fatal(err)
	}
	if err = recoverHistory(root); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(filepath.Join(root, "nested", "a.txt")); err != nil || string(data) != "a" {
		t.Fatal("recovery failed", err)
	}
	if st, err := os.Stat(filepath.Join(root, "empty")); err != nil || !st.IsDir() {
		t.Fatal("empty directory recovery failed", err)
	}
	// Old databases contain only file entries. Restore their contents without migration.
	db, err = openHistory(root)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`INSERT INTO versions(id,label,snapshot) VALUES (100,'legacy populated',?), (101,'legacy empty',?)`, `{"old/deep/a.txt":"old"}`, `{"old/deep/a.txt":null}`)
	db.Close()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = revertVersion(root, 100, ""); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(filepath.Join(root, "old", "deep", "a.txt")); err != nil || string(data) != "old" {
		t.Fatal("legacy version not restored", err)
	}
	if _, err = revertVersion(root, 101, ""); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(filepath.Join(root, "old")); !os.IsNotExist(err) {
		t.Fatal("legacy restored directory remains", err)
	}
}
