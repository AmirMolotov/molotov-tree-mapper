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
	if err = store.SaveSettings(Settings{0}); err == nil {
		t.Fatal("invalid size accepted")
	}
	text := "```json\n" + `{"format":"molotov-llm-v1","root":"project","files":[{"path":"src/main.go","content":"package main\n"}]}` + "\n```"
	plan, err := parseReport(text)
	if err != nil || plan.Contents["src/main.go"] != "package main\n" || len(plan.Dirs) != 1 {
		t.Fatal(plan, err)
	}
	for _, bad := range []string{`{"format":"molotov-llm-v1","root":"project","files":[{"path":"../bad","content":"bad"}]}`, `{"format":"molotov-llm-v1","root":"project","files":[{"path":"main.go"}]}`, text + "prose"} {
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
}
