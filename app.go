package main

import (
	"context"
	"fmt"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

type App struct {
	progressMu   sync.Mutex
	progress     ReportProgress
	cancelReport context.CancelFunc
	ctx          context.Context
	mu           sync.Mutex
	root         string
	tree         *Node
	store        *RuleStore
	storeErr     error
	historyRoot  string
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
	return t, nil
}

type ReportProgress struct {
	Phase       string `json:"phase"`
	Processed   int    `json:"processed"`
	Total       int    `json:"total"`
	Included    int    `json:"included"`
	Skipped     int    `json:"skipped"`
	SourceBytes int    `json:"sourceBytes"`
	ReadBytes   int    `json:"readBytes"`
	OutputBytes int    `json:"outputBytes"`
}

func (a *App) GetReportProgress() ReportProgress {
	a.progressMu.Lock()
	defer a.progressMu.Unlock()
	return a.progress
}
func (a *App) CancelReport() {
	a.progressMu.Lock()
	defer a.progressMu.Unlock()
	if a.cancelReport != nil {
		a.cancelReport()
	}
}
func (a *App) setReportProgress(p ReportProgress) {
	a.progressMu.Lock()
	defer a.progressMu.Unlock()
	a.progress = p
}
func (a *App) Generate(excluded, excludedTypes []string, includePrompt bool) (string, error) {
	a.progressMu.Lock()
	if a.cancelReport != nil {
		a.progressMu.Unlock()
		return "", fmt.Errorf("Report already running")
	}
	ctx, cancel := context.WithCancel(context.Background())
	a.cancelReport = cancel
	a.progress = ReportProgress{Phase: "Scanning"}
	a.progressMu.Unlock()
	defer func() { cancel(); a.progressMu.Lock(); a.cancelReport = nil; a.progressMu.Unlock() }()

	a.mu.Lock()
	defer a.mu.Unlock()
	if a.tree == nil {
		return "", fmt.Errorf("Choose a folder first")
	}
	rules, err := a.rules()
	if err != nil {
		return "", err
	}
	tree, err := scanSelectedContext(ctx, a.root, rules, excluded)
	if err != nil {
		return "", err
	}
	filterTypes(tree, excludedTypes)
	settings, err := a.store.Settings()
	if err != nil {
		return "", err
	}
	limit := settings.MaxReportMiB * 1024 * 1024
	suffix := ""
	if includePrompt {
		suffix = llmPromptStart + llmInstructions + llmPromptEnd
	}
	r, err := reportWithProgress(ctx, a.root, tree, excluded, reportBudget(limit, len(suffix)), a.setReportProgress)
	if err != nil {
		return "", err
	}
	p := a.GetReportProgress()
	p.OutputBytes = len(r) + len(suffix)
	a.setReportProgress(p)
	return r + suffix, nil
}
func (a *App) NewKey() (string, error) { return newKey() }
func (a *App) CopyOutput(text, key string) error {
	if err := a.checkOutputSize(text); err != nil {
		return err
	}
	output, err := encodeOutput(text, key)
	if err != nil {
		return err
	}
	if err := a.checkOutputSize(output); err != nil {
		return err
	}
	return runtime.ClipboardSetText(a.ctx, output)
}
func (a *App) SaveOutput(text, key string) (string, error) {
	if err := a.checkOutputSize(text); err != nil {
		return "", err
	}
	output, err := encodeOutput(text, key)
	if err != nil {
		return "", err
	}
	if err := a.checkOutputSize(output); err != nil {
		return "", err
	}
	name := "tree-report.md"
	if key != "" {
		name = "tree-report.molotov"
	}
	return a.saveText(output, name)
}
func (a *App) SaveKey(key string) (string, error) {
	if _, err := keyCipher(key); err != nil {
		return "", err
	}
	return a.saveText(key, "molotov.key")
}
func (a *App) saveText(text, name string) (string, error) {
	p, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{Title: "Save " + name, DefaultFilename: name})
	if err != nil || p == "" {
		return "", err
	}
	f, err := os.CreateTemp(filepath.Dir(p), ".molotov-*")
	if err != nil {
		return "", err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if _, err = f.WriteString(text); err != nil {
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
func (a *App) OpenText() (string, error) {
	p, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{Title: "Open a report or key"})
	if err != nil || p == "" {
		return "", err
	}
	f, err := os.Open(p)
	if err != nil {
		return "", err
	}
	defer f.Close()
	data, err := io.ReadAll(f)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
func (a *App) InspectReport(text, key string) (*RestorePlan, error) {
	plain, err := decodeInput(text, key)
	if err != nil {
		return nil, err
	}
	return parseReport(plain)
}
func (a *App) DecryptReport(text, key string) (string, error) { return decodeInput(text, key) }
func (a *App) RestoreReport(text, key string) (string, error) {
	p, err := a.InspectReport(text, key)
	if err != nil {
		return "", err
	}
	parent, err := runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{Title: "Choose parent folder for " + p.Root + " (must not already exist)"})
	if err != nil || parent == "" {
		return "", err
	}
	dest, err := restoreFiles(parent, p)
	if err == nil {
		a.mu.Lock()
		a.historyRoot = dest
		a.mu.Unlock()
	}
	return dest, err
}

type FileType struct {
	Extension string `json:"extension"`
	Count     int    `json:"count"`
}

func (a *App) FileTypes() ([]FileType, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.root == "" {
		return nil, fmt.Errorf("Choose a folder first")
	}
	rules, err := a.rules()
	if err != nil {
		return nil, err
	}
	tree, err := scanSelected(a.root, rules, nil)
	if err != nil {
		return nil, err
	}
	counts := map[string]int{}
	var visit func(*Node)
	visit = func(n *Node) {
		if !n.Dir {
			counts[strings.ToLower(filepath.Ext(n.Name))]++
		}
		for _, c := range n.Children {
			visit(c)
		}
	}
	visit(tree)
	result := []FileType{}
	for ext, count := range counts {
		result = append(result, FileType{ext, count})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Extension < result[j].Extension })
	return result, nil
}
func filterTypes(tree *Node, excluded []string) {
	blocked := map[string]bool{}
	for _, ext := range excluded {
		blocked[strings.ToLower(ext)] = true
	}
	var walk func(*Node)
	walk = func(n *Node) {
		children := n.Children[:0]
		for _, c := range n.Children {
			if c.Dir {
				walk(c)
			} else if blocked[strings.ToLower(filepath.Ext(c.Name))] {
				continue
			}
			children = append(children, c)
		}
		n.Children = children
	}
	walk(tree)
}

func (a *App) checkOutputSize(text string) error {
	settings, err := a.GetSettings()
	if err != nil {
		return err
	}
	if settings.MaxReportMiB > 0 && len(text) > settings.MaxReportMiB*1024*1024 {
		return fmt.Errorf("Output is %.2f MiB; active limit is %d MiB. Increase Settings or reduce the report. Encryption adds encoding overhead", float64(len(text))/(1024*1024), settings.MaxReportMiB)
	}
	return nil
}

// Wails uses fixed Yes/No buttons for Windows question dialogs.
func confirmHistoryChange(ctx context.Context, title, message string, show func(context.Context, runtime.MessageDialogOptions) (string, error)) (bool, error) {
	answer, err := show(ctx, runtime.MessageDialogOptions{
		Type: runtime.QuestionDialog, Title: title, Message: message,
		Buttons: []string{"Yes", "No"}, DefaultButton: "No", CancelButton: "No",
	})
	return err == nil && answer == "Yes", err
}
func (a *App) UpdateRestore(text, key string) (History, error) {
	p, err := a.InspectReport(text, key)
	if err != nil {
		return History{}, err
	}
	root, err := runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{Title: "Choose existing project folder named " + p.Root})
	if err != nil || root == "" {
		return History{}, err
	}
	if filepath.Base(root) != p.Root {
		return History{}, fmt.Errorf("Selected folder must be named %s", p.Root)
	}
	confirmed, err := confirmHistoryChange(a.ctx, "Apply versioned update", fmt.Sprintf("Update %d files in %s? Current managed content will be saved in local history. Unmentioned files remain unchanged.", len(p.Contents), root), runtime.MessageDialog)
	if err != nil || !confirmed {
		return History{}, err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	changes := snapshot{}
	for name, body := range p.Contents {
		value := body
		changes[name] = &value
	}
	for _, dir := range p.Dirs {
		present := ""
		changes[dir+"/"] = &present
	}
	if _, err = versionedChange(root, "Restore update", changes, p.Files); err != nil {
		return History{}, err
	}
	a.historyRoot = root
	return loadHistory(root)
}
func (a *App) ChooseHistory() (History, error) {
	root, err := runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{Title: "Choose project for version history"})
	if err != nil || root == "" {
		return History{}, err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	h, err := loadHistory(root)
	if err == nil {
		a.historyRoot = root
	}
	return h, err
}
func (a *App) GetHistory() (History, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.historyRoot == "" {
		return History{Versions: []Version{}}, nil
	}
	return loadHistory(a.historyRoot)
}
func (a *App) RevertHistory(id int64, file string) (History, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.historyRoot == "" {
		return History{}, fmt.Errorf("Choose a history project first")
	}
	message := "Revert all managed files to this version? Later added managed files may be removed; Untracked content is preserved; tracked folders are removed only when empty. Current managed content is saved as another version."
	if file != "" {
		message = "Revert " + file + " to the selected version? Its current content is saved first. The file may be removed if it did not exist in that version."
	}
	confirmed, err := confirmHistoryChange(a.ctx, "Revert version", message, runtime.MessageDialog)
	if err != nil {
		return History{}, err
	}
	if !confirmed {
		return loadHistory(a.historyRoot)
	}
	if _, err = revertVersion(a.historyRoot, id, file); err != nil {
		return History{}, err
	}
	return loadHistory(a.historyRoot)
}
func (a *App) RecoverHistory() (History, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.historyRoot == "" {
		return History{}, fmt.Errorf("Choose a history project first")
	}
	if err := recoverHistory(a.historyRoot); err != nil {
		return History{}, err
	}
	return loadHistory(a.historyRoot)
}

func reportBudget(limit, suffix int) int {
	if limit == 0 {
		return 0
	}
	return limit - suffix
}
