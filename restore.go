package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"
)

type Manifest struct {
	Root    string   `json:"root"`
	Dirs    []string `json:"dirs"`
	Files   []string `json:"files"`
	Skipped []string `json:"skipped"`
}
type RestorePlan struct {
	Manifest
	Contents map[string]string `json:"-"`
}

const manifestPrefix = "<!-- molotov-v2 "

func validRelative(p string) error {
	if p == "" || len(p) > 4096 || strings.ContainsAny(p, "\\:\x00") || path.IsAbs(p) || path.Clean(p) != p {
		return fmt.Errorf("Unsafe path %q", p)
	}
	for _, part := range strings.Split(p, "/") {
		if part == "." || part == ".." || len(part) > 255 || strings.TrimRight(part, ". ") != part {
			return fmt.Errorf("Unsafe filename %q", part)
		}
		for _, r := range part {
			if unicode.IsControl(r) || strings.ContainsRune(`<>"|?*`, r) {
				return fmt.Errorf("Unsupported filename %q", part)
			}
		}
		stem := strings.ToUpper(strings.Split(part, ".")[0])
		if stem == "CON" || stem == "PRN" || stem == "AUX" || stem == "NUL" || len(stem) == 4 && (strings.HasPrefix(stem, "COM") || strings.HasPrefix(stem, "LPT")) && stem[3] >= '0' && stem[3] <= '9' {
			return fmt.Errorf("Reserved filename %q", part)
		}
	}
	return nil
}

func parseReport(text string) (*RestorePlan, error) {
	text = stripLLMPrompt(text)
	trimmed := strings.TrimSpace(text)
	if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "```json") {
		return parseLLM(trimmed)
	}

	if len(text) > maxReport || !utf8.ValidString(text) || strings.ContainsRune(text, 0) {
		return nil, fmt.Errorf("Report must be UTF-8 text under 128 MiB")
	}
	p := &RestorePlan{Contents: map[string]string{}}
	treeStart := strings.Index(text, "\n## Directory tree\n\n")
	bodyStart := strings.Index(text, "\n## File contents\n")
	if !strings.HasPrefix(text, "# ") || treeStart < 0 || bodyStart <= treeStart {
		return nil, fmt.Errorf("Not a Molotov report: directory tree and file contents sections required")
	}
	header := text[:treeStart]
	if i := strings.Index(header, "\n"+manifestPrefix); i >= 0 {
		line := strings.TrimSpace(header[i+1:])
		if !strings.HasSuffix(line, " -->") {
			return nil, fmt.Errorf("Invalid manifest")
		}
		data, err := base64.StdEncoding.DecodeString(strings.TrimSuffix(strings.TrimPrefix(line, manifestPrefix), " -->"))
		if err != nil {
			return nil, fmt.Errorf("Invalid manifest")
		}
		if err = json.Unmarshal(data, &p.Manifest); err != nil {
			return nil, fmt.Errorf("Invalid manifest: %w", err)
		}
	} else {
		if err := parseLegacyTree(text[treeStart+len("\n## Directory tree\n\n"):bodyStart], &p.Manifest); err != nil {
			return nil, err
		}
	}
	if err := validatePlan(p); err != nil {
		return nil, err
	}
	kinds := map[string]bool{}
	for _, name := range p.Files {
		kinds[name] = false
	}
	for _, name := range p.Dirs {
		kinds[name] = true
	}
	skipped := map[string]bool{}
	for _, name := range p.Skipped {
		skipped[name] = true
	}
	body := text[bodyStart+len("\n## File contents\n"):]
	parsed := map[string]bool{}
	for strings.TrimSpace(body) != "" {
		body = strings.TrimLeft(body, "\n")
		if !strings.HasPrefix(body, "### ") {
			return nil, fmt.Errorf("Expected file heading")
		}
		end := strings.IndexByte(body, '\n')
		if end < 0 {
			return nil, fmt.Errorf("Incomplete file heading")
		}
		name := strings.TrimPrefix(body[:end], "### ")
		body = body[end+1:]
		if dir, ok := kinds[name]; !ok || dir || parsed[name] {
			return nil, fmt.Errorf("Unexpected or duplicate file %q", name)
		}
		parsed[name] = true
		if !strings.HasPrefix(body, "\n") {
			return nil, fmt.Errorf("Missing blank line after %s", name)
		}
		body = body[1:]
		end = strings.IndexByte(body, '\n')
		if end < 0 {
			return nil, fmt.Errorf("Missing file fence")
		}
		fence := body[:end]
		if len(fence) < 3 || strings.Trim(fence, "`") != "" {
			return nil, fmt.Errorf("Invalid file fence")
		}
		body = body[end+1:]
		closeAt := strings.Index(body, "\n"+fence+"\n")
		if closeAt < 0 {
			return nil, fmt.Errorf("Unclosed file fence: %s", name)
		}
		content := body[:closeAt]
		body = body[closeAt+len(fence)+2:]
		if !skipped[name] && strings.HasPrefix(content, "[Skipped: ") && strings.HasSuffix(content, "]") && !strings.Contains(header, manifestPrefix) {
			p.Skipped = append(p.Skipped, name)
			skipped[name] = true
		}
		if !skipped[name] {
			p.Contents[name] = content
		}
	}
	for _, name := range p.Files {
		if !parsed[name] {
			return nil, fmt.Errorf("Missing contents for %s", name)
		}
	}
	return p, nil
}

// Older reports have no manifest; their tree provides empty directories and file identities.
func parseLegacyTree(text string, m *Manifest) error {
	lines := strings.Split(strings.TrimSuffix(text, "\n"), "\n")
	if len(lines) == 0 || !strings.HasSuffix(lines[0], "/") {
		return fmt.Errorf("Invalid directory tree")
	}
	m.Root = strings.TrimSuffix(lines[0], "/")
	parents := []string{""}
	for _, line := range lines[1:] {
		if line == "" {
			continue
		}
		depth := 0
		for strings.HasPrefix(line, "│   ") || strings.HasPrefix(line, "    ") {
			if strings.HasPrefix(line, "│   ") {
				line = strings.TrimPrefix(line, "│   ")
			} else {
				line = line[4:]
			}
			depth++
		}
		if depth >= len(parents) || (!strings.HasPrefix(line, "├── ") && !strings.HasPrefix(line, "└── ")) {
			return fmt.Errorf("Invalid tree indentation")
		}
		name := strings.TrimPrefix(strings.TrimPrefix(line, "├── "), "└── ")
		if i := strings.Index(name, " ["); i >= 0 {
			name = name[:i]
		}
		dir := strings.HasSuffix(name, "/")
		name = strings.TrimSuffix(name, "/")
		if strings.Contains(name, "/") {
			return fmt.Errorf("Invalid tree name")
		}
		rel := path.Join(parents[depth], name)
		if dir {
			m.Dirs = append(m.Dirs, rel)
			parents = append(parents[:depth+1], rel)
		} else {
			m.Files = append(m.Files, rel)
			parents = parents[:depth+1]
		}
	}
	return nil
}

func restoreFiles(parent string, p *RestorePlan) (string, error) {
	if err := validatePlan(p); err != nil {
		return "", err
	}
	parent, err := filepath.Abs(parent)
	if err != nil {
		return "", err
	}
	info, err := os.Lstat(parent)
	if err != nil {
		return "", err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return "", fmt.Errorf("Choose a real destination directory")
	}
	target := filepath.Join(parent, p.Root)
	if _, err = os.Lstat(target); !os.IsNotExist(err) {
		return "", fmt.Errorf("Destination already exists or is inaccessible; choose another parent folder")
	}
	stage, err := os.MkdirTemp(parent, ".molotov-restore-")
	if err != nil {
		return "", err
	}
	// stage is a newly created directory directly inside the explicitly chosen parent.
	defer os.RemoveAll(stage)
	for _, dir := range p.Dirs {
		if err = os.MkdirAll(filepath.Join(stage, filepath.FromSlash(dir)), 0700); err != nil {
			return "", err
		}
	}
	changes := snapshot{}
	for name, body := range p.Contents {
		value := body
		changes[name] = &value
	}
	if _, err = versionedChange(stage, "Initial restore", changes); err != nil {
		return "", err
	}
	if _, err = os.Lstat(target); !os.IsNotExist(err) {
		return "", fmt.Errorf("Destination appeared during restore; nothing overwritten")
	}
	if err = os.Rename(stage, target); err != nil {
		return "", err
	}
	return target, nil
}

func validatePlan(p *RestorePlan) error {
	if strings.Contains(p.Root, "/") {
		return fmt.Errorf("Root must be a single folder name")
	}
	if err := validRelative(p.Root); err != nil {
		return err
	}
	if len(p.Dirs)+len(p.Files) > maxNodes {
		return fmt.Errorf("Too many entries")
	}
	kinds := map[string]bool{}
	seen := map[string]bool{}
	for _, group := range []struct {
		paths []string
		dir   bool
	}{{p.Dirs, true}, {p.Files, false}} {
		for _, name := range group.paths {
			if err := validManagedPath(name); err != nil {
				return err
			}
			fold := strings.ToLower(name)
			if seen[fold] {
				return fmt.Errorf("Duplicate or case-conflicting path %q", name)
			}
			seen[fold] = true
			kinds[name] = group.dir
		}
	}
	for name := range kinds {
		for parent := path.Dir(name); parent != "."; parent = path.Dir(parent) {
			if dir, ok := kinds[parent]; !ok || !dir {
				return fmt.Errorf("Missing directory or file/directory collision: %s", parent)
			}
		}
	}
	skipped := map[string]bool{}
	for _, name := range p.Skipped {
		if dir, ok := kinds[name]; !ok || dir || skipped[name] {
			return fmt.Errorf("Invalid skipped-file entry")
		}
		skipped[name] = true
	}

	return nil
}
