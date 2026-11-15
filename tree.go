package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"
)

const maxFile = 2 * 1024 * 1024
const maxNodes = 50000

type Node struct {
	Root     string  `json:"root,omitempty"`
	Name     string  `json:"name"`
	Path     string  `json:"path"`
	Dir      bool    `json:"dir"`
	Children []*Node `json:"children"`
	Warning  string  `json:"warning,omitempty"`
}

func scan(root string) (*Node, error) { return scanSelected(root, nil, nil) }

func scanSelected(root string, rules, excluded []string) (*Node, error) {
	n, err := readLevel(root, "", rules)
	if err != nil {
		return nil, err
	}
	count := 0
	var expand func(*Node, int) error
	expand = func(n *Node, depth int) error {
		if depth > 128 {
			return fmt.Errorf("Folder exceeds 128 levels")
		}
		children := n.Children[:0]
		for _, c := range n.Children {
			if blockedPath(c.Path, excluded) {
				continue
			}
			count++
			if count > maxNodes {
				return fmt.Errorf("Report exceeds 50,000 entries; add exclusion rules")
			}
			if c.Dir && c.Warning == "" {
				level, err := readLevel(root, c.Path, rules)
				if err != nil {
					c.Warning = err.Error()
				} else {
					c.Children = level.Children
				}
				if err := expand(c, depth+1); err != nil {
					return err
				}
			}
			children = append(children, c)
		}
		n.Children = children
		return nil
	}
	if blockedPath("", excluded) {
		n.Children = nil
		return n, nil
	}
	return n, expand(n, 0)
}

func blockedPath(rel string, excluded []string) bool {
	for _, p := range excluded {
		if p == "" || rel == p || strings.HasPrefix(rel, p+"/") {
			return true
		}
	}
	return false
}

func checkedPath(root, rel string) (string, error) {
	if rel != "" && (!filepath.IsLocal(filepath.FromSlash(rel)) || strings.Contains(rel, "\\")) {
		return "", fmt.Errorf("Invalid relative path")
	}
	current := root
	parts := []string{""}
	if rel != "" {
		parts = append(parts, strings.Split(rel, "/")...)
	}
	for _, part := range parts {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if err != nil {
			return "", err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("Symbolic link skipped")
		}
	}
	return current, nil
}

// Only this directory's metadata is loaded; no file contents or descendants are read.
func readLevel(root, rel string, rules []string) (*Node, error) {
	full, err := checkedPath(root, rel)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(full)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("Choose a folder")
	}
	n := &Node{Name: info.Name(), Path: rel, Dir: true, Children: []*Node{}}
	if rel == "" {
		n.Root = root
	}
	entries, err := os.ReadDir(full)
	if err != nil {
		return nil, err
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].IsDir() != entries[j].IsDir() {
			return entries[i].IsDir()
		}
		return entries[i].Name() < entries[j].Name()
	})
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".molotov-history.sqlite") || matchesName(e.Name(), rules) {
			continue
		}
		c := &Node{Name: e.Name(), Path: filepath.ToSlash(filepath.Join(rel, e.Name())), Dir: e.IsDir()}
		if e.Type()&os.ModeSymlink != 0 {
			c.Warning = "Symbolic link skipped"
			c.Dir = false
		}
		n.Children = append(n.Children, c)
	}
	return n, nil
}

// Recheck every component so a replaced directory cannot redirect reads through a symlink.
func safeRead(root, rel string) ([]byte, error) {
	full, err := checkedPath(root, rel)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(full)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !st.Mode().IsRegular() {
		return nil, fmt.Errorf("not a regular file")
	}
	data, err := io.ReadAll(f)
	if err != nil {
		return nil, err
	}
	if !utf8.Valid(data) || strings.ContainsRune(string(data), 0) {
		return nil, fmt.Errorf("binary or non-UTF-8 file")
	}
	return data, nil
}

func report(root string, tree *Node, excluded []string) (string, error) {
	return reportLimited(root, tree, excluded, 0)
}
func reportLimited(root string, tree *Node, excluded []string, limit int) (string, error) {
	blocked := map[string]bool{}
	for _, p := range excluded {
		blocked[p] = true
	}
	var b strings.Builder
	add := func(s string) error {
		if limit > 0 && b.Len()+len(s) > limit {
			return fmt.Errorf("Report needs at least %.2f MiB; active report budget is %.2f MiB. Increase Settings or exclude more files", float64(b.Len()+len(s))/(1024*1024), float64(limit)/(1024*1024))
		}
		b.WriteString(s)
		return nil
	}
	b.WriteString("# " + tree.Name + "\n\n## Directory tree\n\n" + tree.Name + "/\n")
	manifest := Manifest{Root: tree.Name}
	if err := validRelative(tree.Name); err != nil {
		return "", err
	}
	var files []*Node
	var walk func(*Node, string) error
	walk = func(n *Node, prefix string) error {
		var children []*Node
		for _, c := range n.Children {
			if !blocked[c.Path] {
				children = append(children, c)
			}
		}
		for i, c := range children {
			if err := validRelative(c.Path); err != nil {
				return err
			}
			if c.Dir {
				manifest.Dirs = append(manifest.Dirs, c.Path)
			} else {
				manifest.Files = append(manifest.Files, c.Path)
			}
			branch, next := "├── ", "│   "
			if i == len(children)-1 {
				branch, next = "└── ", "    "
			}
			name := c.Name
			if c.Dir {
				name += "/"
			}
			if c.Warning != "" {
				name += " [" + c.Warning + "]"
			}
			if err := add(prefix + branch + name + "\n"); err != nil {
				return err
			}
			if c.Dir {
				if err := walk(c, prefix+next); err != nil {
					return err
				}
			} else {
				files = append(files, c)
			}
		}
		return nil
	}
	if !blocked[""] {
		if err := walk(tree, ""); err != nil {
			return "", err
		}
	}
	if tree.Warning != "" {
		b.WriteString("\n[" + tree.Warning + "]\n")
	}
	b.WriteString("\n## File contents\n")
	for _, f := range files {
		data, err := safeRead(root, f.Path)
		body := string(data)
		if err != nil {
			body = "[Skipped: " + err.Error() + "]"
			manifest.Skipped = append(manifest.Skipped, f.Path)
		}
		longest, run := 2, 0
		for _, ch := range body {
			if ch == '`' {
				run++
				if run > longest {
					longest = run
				}
			} else {
				run = 0
			}
		}
		fence := strings.Repeat("`", longest+1)
		if err := add("\n### " + f.Path + "\n\n" + fence + "\n" + body + "\n" + fence + "\n"); err != nil {
			return "", err
		}
	}
	metadata, err := json.Marshal(manifest)
	if err != nil {
		return "", err
	}
	output := strings.Replace(b.String(), "\n", "\n"+manifestPrefix+base64.StdEncoding.EncodeToString(metadata)+" -->\n", 1)
	if limit > 0 && len(output) > limit {
		return "", fmt.Errorf("Report including metadata needs %.2f MiB; active report budget is %.2f MiB. Increase Settings or exclude more files", float64(len(output))/(1024*1024), float64(limit)/(1024*1024))
	}
	return output, nil
}
