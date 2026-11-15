package main

import (
	"encoding/json"
	"fmt"
	"io"
	"path"
	"sort"
	"strings"
	"unicode/utf8"
)

func parseLLM(text string) (*RestorePlan, error) {
	if len(text) > maxReport || !utf8.ValidString(text) {
		return nil, fmt.Errorf("LLM response exceeds the supported size or is not UTF-8")
	}
	if strings.HasPrefix(text, "```json\n") && strings.HasSuffix(text, "\n```") {
		text = strings.TrimSuffix(strings.TrimPrefix(text, "```json\n"), "\n```")
	}
	var input struct {
		Format      string   `json:"format"`
		Root        string   `json:"root"`
		Directories []string `json:"directories"`
		Files       []struct {
			Path    string  `json:"path"`
			Content *string `json:"content"`
		} `json:"files"`
	}
	decoder := json.NewDecoder(strings.NewReader(text))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		return nil, fmt.Errorf("Invalid LLM JSON: %w", err)
	}
	if input.Format != "molotov-llm-v1" {
		return nil, fmt.Errorf("Use format molotov-llm-v1")
	}
	var extra interface{}
	if err := decoder.Decode(&extra); err != io.EOF {
		return nil, fmt.Errorf("Unexpected text after LLM JSON")
	}
	p := &RestorePlan{Manifest: Manifest{Root: input.Root}, Contents: map[string]string{}}
	dirs := map[string]bool{}
	addParents := func(name string) {
		for d := path.Dir(name); d != "."; d = path.Dir(d) {
			dirs[d] = true
		}
	}
	for _, d := range input.Directories {
		if err := validRelative(d); err != nil {
			return nil, err
		}
		dirs[d] = true
		addParents(d)
	}
	for _, f := range input.Files {
		if err := validRelative(f.Path); err != nil {
			return nil, err
		}
		if f.Content == nil || strings.ContainsRune(*f.Content, 0) {
			return nil, fmt.Errorf("Complete text content is required for %s", f.Path)
		}
		if _, ok := p.Contents[f.Path]; ok {
			return nil, fmt.Errorf("Duplicate file %s", f.Path)
		}
		p.Files = append(p.Files, f.Path)
		p.Contents[f.Path] = *f.Content
		addParents(f.Path)
	}
	for d := range dirs {
		p.Dirs = append(p.Dirs, d)
	}
	sort.Strings(p.Dirs)
	if err := validatePlan(p); err != nil {
		return nil, err
	}
	return p, nil
}
