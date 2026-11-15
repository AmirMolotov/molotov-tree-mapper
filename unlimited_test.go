package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUnlimitedGenerationAfterExplicitCap(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "large.txt"), []byte(strings.Repeat("x", 2*1024*1024)), 0600)
	store, err := openRules(filepath.Join(t.TempDir(), "settings.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.db.Close()
	a := &App{store: store}
	defaults, err := a.GetSettings()
	if err != nil || defaults.MaxReportMiB != 0 {
		t.Fatal("unlimited not default", defaults, err)
	}
	if _, err = a.LoadFolder(root); err != nil {
		t.Fatal(err)
	}
	a.SaveSettings(Settings{1})
	if _, err = a.Generate(nil, nil, false); err == nil {
		t.Fatal("explicit cap ignored")
	}
	if err = a.SaveSettings(Settings{0}); err != nil {
		t.Fatal(err)
	}
	text, err := a.Generate(nil, nil, true)
	if err != nil {
		t.Fatal("unlimited generation rejected", err)
	}
	if len(text) <= 2*1024*1024 {
		t.Fatal("content truncated")
	}
	if err = a.checkOutputSize(text); err != nil {
		t.Fatal(err)
	}
	key, _ := newKey()
	encrypted, err := encodeOutput(text, key)
	if err != nil {
		t.Fatal(err)
	}
	if err = a.checkOutputSize(encrypted); err != nil {
		t.Fatal(err)
	}
	plain, err := decodeInput(encrypted, key)
	if err != nil || plain != text {
		t.Fatal("encrypted round trip failed", err)
	}
	if _, err = parseReport(plain); err != nil {
		t.Fatal(err)
	}
}
func TestLegacyCapMigratesOnce(t *testing.T) {
	file := filepath.Join(t.TempDir(), "settings.db")
	store, err := openRules(file)
	if err != nil {
		t.Fatal(err)
	}
	store.SaveSettings(Settings{64})
	store.db.Exec(`DELETE FROM app_settings WHERE key='unlimited_default_v1'`)
	store.db.Close()
	store, err = openRules(file)
	if err != nil {
		t.Fatal(err)
	}
	s, err := store.Settings()
	if err != nil || s.MaxReportMiB != 0 {
		t.Fatal("legacy cap was not disabled", s, err)
	}
	store.SaveSettings(Settings{256})
	store.db.Close()
	store, err = openRules(file)
	if err != nil {
		t.Fatal(err)
	}
	defer store.db.Close()
	s, err = store.Settings()
	if err != nil || s.MaxReportMiB != 256 {
		t.Fatal("opt-in cap was reset or restricted", s, err)
	}
}
