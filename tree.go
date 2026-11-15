package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"
)

const maxFile = 2 * 1024 * 1024
const maxReport = 32 * 1024 * 1024
const maxNodes = 50000

type Node struct {
	Name     string  `json:"name"`
	Path     string  `json:"path"`
	Dir      bool    `json:"dir"`
	Children []*Node `json:"children"`
	Warning  string  `json:"warning,omitempty"`
}

func scan(root string) (*Node, error) {
	info, err := os.Lstat(root)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("Drop or choose a folder")
	}
	count := 0
	var walk func(string, string, int) (*Node, error)
	walk = func(full, rel string, depth int) (*Node, error) {
		count++
		if count > maxNodes || depth > 128 {
			return nil, fmt.Errorf("Folder exceeds the scan limit (50,000 entries or 128 levels); choose a smaller folder")
		}
		st, err := os.Lstat(full)
		if err != nil {
			return &Node{Name: filepath.Base(full), Path: rel, Warning: err.Error()}, nil
		}
		n := &Node{Name: st.Name(), Path: rel, Dir: st.IsDir()}
		if st.Mode()&os.ModeSymlink != 0 {
			n.Warning = "Symbolic link skipped"
			return n, nil
		}
		if !n.Dir {
			return n, nil
		}
		entries, err := os.ReadDir(full)
		if err != nil {
			n.Warning = "Cannot read directory: " + err.Error()
			return n, nil
		}
		sort.Slice(entries, func(i, j int) bool {
			if entries[i].IsDir() != entries[j].IsDir() {
				return entries[i].IsDir()
			}
			return entries[i].Name() < entries[j].Name()
		})
		for _, e := range entries {
			child, err := walk(filepath.Join(full, e.Name()), filepath.ToSlash(filepath.Join(rel, e.Name())), depth+1)
			if err != nil {
				return nil, err
			}
			n.Children = append(n.Children, child)
		}
		return n, nil
	}
	return walk(root, "", 0)
}

// Recheck every component so a replaced directory cannot redirect reads through a symlink.
func safeRead(root, rel string) ([]byte, error) {
	full := filepath.Join(root, filepath.FromSlash(rel))
	current := root
	for _, part := range strings.Split(filepath.FromSlash(rel), string(filepath.Separator)) {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if err != nil {
			return nil, err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("symbolic link skipped")
		}
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
	data, err := io.ReadAll(io.LimitReader(f, maxFile+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxFile {
		return nil, fmt.Errorf("larger than 2 MiB")
	}
	if !utf8.Valid(data) || strings.ContainsRune(string(data), 0) {
		return nil, fmt.Errorf("binary or non-UTF-8 file")
	}
	return data, nil
}

func report(root string, tree *Node, excluded []string) (string, error) {
	blocked := map[string]bool{}
	for _, p := range excluded {
		blocked[p] = true
	}
	var b strings.Builder
	add := func(s string) error {
		if b.Len()+len(s) > maxReport {
			return fmt.Errorf("Report exceeds 32 MiB; exclude folders and try again")
		}
		b.WriteString(s)
		return nil
	}
	b.WriteString("# " + tree.Name + "\n\n## Directory tree\n\n" + tree.Name + "/\n")
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
		}
		fence := "```"
		for strings.Contains(body, fence) {
			fence += "`"
		}
		if err := add("\n### " + f.Path + "\n\n" + fence + "\n" + body + "\n" + fence + "\n"); err != nil {
			return "", err
		}
	}
	return b.String(), nil
}
