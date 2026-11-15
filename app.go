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
	ctx      context.Context
	mu       sync.Mutex
	root     string
	tree     *Node
	latest   string
	store    *RuleStore
	storeErr error
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	dir, err := os.UserConfigDir()
	if err != nil {
		a.storeErr = err
		return
	}
	a.store, a.storeErr = openRules(filepath.Join(dir, "Molotov Tree Mapper", "settings.db"))
}
func (a *App) GetExcludes() ([]string, error) { a.mu.Lock(); defer a.mu.Unlock(); return a.rules() }
func (a *App) rules() ([]string, error) {
	if a.storeErr != nil {
		return nil, a.storeErr
	}
	if a.store == nil {
		return nil, fmt.Errorf("Settings unavailable")
	}
	return a.store.List()
}
func (a *App) SaveExcludes(rules []string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if _, err := a.rules(); err != nil {
		return err
	}
	if err := a.store.Save(rules); err != nil {
		return err
	}
	a.latest = ""
	return nil
}
func (a *App) LoadChildren(rel string) ([]*Node, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.root == "" {
		return nil, fmt.Errorf("Choose a folder first")
	}
	rules, err := a.rules()
	if err != nil {
		return nil, err
	}
	n, err := readLevel(a.root, rel, rules)
	if err != nil {
		return nil, err
	}
	return n.Children, nil
}

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
	rules, err := a.rules()
	if err != nil {
		return nil, err
	}
	t, err := readLevel(p, "", rules)
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
	rules, err := a.rules()
	if err != nil {
		return "", err
	}
	tree, err := scanSelected(a.root, rules, excluded)
	if err != nil {
		return "", err
	}
	r, err := report(a.root, tree, excluded)
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
