package main

import (
	"context"
	"fmt"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"os"
	"path/filepath"
	"sync"
)

type App struct {
	ctx    context.Context
	mu     sync.Mutex
	root   string
	tree   *Node
	latest string
}

func (a *App) startup(ctx context.Context) { a.ctx = ctx }
func (a *App) ChooseFolder() (*Node, error) {
	p, err := runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{Title: "Choose a folder"})
	if err != nil || p == "" {
		return nil, err
	}
	return a.LoadFolder(p)
}
func (a *App) LoadFolder(path string) (*Node, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	p, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	t, err := scan(p)
	if err != nil {
		return nil, err
	}
	a.root = p
	a.tree = t
	a.latest = ""
	return t, nil
}
func (a *App) Generate(excluded []string) (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.tree == nil {
		return "", fmt.Errorf("Choose a folder first")
	}
	r, err := report(a.root, a.tree, excluded)
	if err != nil {
		return "", err
	}
	a.latest = r
	return r, nil
}
func (a *App) CopyReport() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.latest == "" {
		return fmt.Errorf("Generate a report first")
	}
	return runtime.ClipboardSetText(a.ctx, a.latest)
}
func (a *App) SaveReport() (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.latest == "" {
		return "", fmt.Errorf("Generate a report first")
	}
	p, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{Title: "Save report", DefaultFilename: "tree-report.md", Filters: []runtime.FileFilter{{DisplayName: "Markdown", Pattern: "*.md"}, {DisplayName: "Text", Pattern: "*.txt"}}})
	if err != nil || p == "" {
		return "", err
	}
	// Write beside the destination, then rename, so a failed write leaves an existing report intact.
	f, err := os.CreateTemp(filepath.Dir(p), ".molotov-*")
	if err != nil {
		return "", err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if _, err = f.WriteString(a.latest); err != nil {
		f.Close()
		return "", err
	}
	if err = f.Close(); err != nil {
		return "", err
	}
	if err = os.Rename(tmp, p); err != nil {
		return "", err
	}
	return p, nil
}
