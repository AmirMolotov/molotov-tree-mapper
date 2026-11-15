package main

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReportRestoreRoundTrip(t *testing.T) {
	root := filepath.Join(t.TempDir(), "project")
	if err := os.MkdirAll(filepath.Join(root, "src", "empty"), 0700); err != nil {
		t.Fatal(err)
	}
	fixtures := map[string]string{"empty.txt": "", "src/no-newline.txt": "hello", "src/crlf.txt": "a\r\nb\r\n", "src/fences.md": "# heading\n```\n### fake.txt\n````\n\n"}
	for name, content := range fixtures {
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(name)), []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	os.WriteFile(filepath.Join(root, "binary"), []byte{0, 1}, 0600)
	tree, err := scan(root)
	if err != nil {
		t.Fatal(err)
	}
	text, err := report(root, tree, nil)
	if err != nil {
		t.Fatal(err)
	}
	key, err := newKey()
	if err != nil {
		t.Fatal(err)
	}
	cipher, err := encodeOutput(text, key)
	if err != nil {
		t.Fatal(err)
	}
	plain, err := decodeInput(cipher, key)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := parseReport(plain)
	if err != nil {
		t.Fatal(err)
	}
	parent := t.TempDir()
	dest, err := restoreFiles(parent, plan)
	if err != nil {
		t.Fatal(err)
	}
	for name, want := range fixtures {
		got, err := os.ReadFile(filepath.Join(dest, filepath.FromSlash(name)))
		if err != nil || string(got) != want {
			t.Errorf("%s: %q vs %q, %v", name, got, want, err)
		}
	}
	if info, err := os.Stat(filepath.Join(dest, "src", "empty")); err != nil || !info.IsDir() {
		t.Fatal("empty directory missing", err)
	}
	if _, err := os.Stat(filepath.Join(dest, "binary")); !os.IsNotExist(err) {
		t.Fatal("skipped binary restored")
	}
	if _, err := restoreFiles(parent, plan); err == nil {
		t.Fatal("existing target accepted")
	}
	edited := strings.Replace(text, "hello", "edited", 1)
	plan, err = parseReport(edited)
	if err != nil || plan.Contents["src/no-newline.txt"] != "edited" {
		t.Fatal("edited content failed", err)
	}
	// Old exports have no metadata line but should still restore unambiguous reports.
	start := strings.Index(text, "\n"+manifestPrefix)
	end := strings.Index(text[start+1:], "\n") + start + 1
	legacy := text[:start] + text[end:]
	plan, err = parseReport(legacy)
	if err != nil {
		t.Fatal("legacy:", err)
	}
	if plan.Contents["src/crlf.txt"] != fixtures["src/crlf.txt"] {
		t.Fatal("legacy bytes changed")
	}
}

func TestFileTypeFiltersBeforeReading(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "photo.PNG"), []byte("image data"), 0600)
	os.WriteFile(filepath.Join(root, "code.go"), []byte("package main"), 0600)
	tree, err := scan(root)
	if err != nil {
		t.Fatal(err)
	}
	filterTypes(tree, []string{".png"})
	text, err := report(root, tree, nil)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(text, "photo.PNG") || strings.Contains(text, "image data") || !strings.Contains(text, "package main") {
		t.Fatal("file types were not excluded")
	}
}

func TestEncryptionRejectsWrongKeyAndTampering(t *testing.T) {
	key, _ := newKey()
	other, _ := newKey()
	first, err := encodeOutput("secret", key)
	if err != nil {
		t.Fatal(err)
	}
	second, _ := encodeOutput("secret", key)
	if first == second {
		t.Fatal("nonce reused")
	}
	if strings.Contains(first, "secret") {
		t.Fatal("plaintext leaked")
	}
	for _, k := range []string{other, "", "password"} {
		if _, err := decodeInput(first, k); err == nil {
			t.Fatal("bad key accepted")
		}
	}
	bytes, _ := base64.StdEncoding.DecodeString(strings.TrimPrefix(first, encryptedHeader))
	bytes[len(bytes)-1] ^= 1
	if _, err := decodeInput(encryptedHeader+base64.StdEncoding.EncodeToString(bytes), key); err == nil {
		t.Fatal("modified report accepted")
	}
	if _, err := decodeInput(encryptedHeader+"bad", key); err == nil {
		t.Fatal("invalid input accepted")
	}
	if plain, err := decodeInput("plain", ""); err != nil || plain != "plain" {
		t.Fatal("optional encryption failed")
	}
}

func crafted(m Manifest, body string) string {
	data, _ := json.Marshal(m)
	return "# project\n" + manifestPrefix + base64.StdEncoding.EncodeToString(data) + " -->\n\n## Directory tree\n\nproject/\n\n## File contents\n" + body
}
func TestRestoreRejectsUnsafeAndMalformedReports(t *testing.T) {
	for _, name := range []string{"../escape", "/abs", "C:/escape", "a/../../escape", "a\\b", "CON", "NUL.txt", "a.", "a ", "a\nheading"} {
		_, err := parseReport(crafted(Manifest{Root: "project", Dirs: []string{name}}, ""))
		if err == nil {
			t.Errorf("unsafe path accepted: %q", name)
		}
	}
	cases := []string{
		crafted(Manifest{Root: "../project"}, ""),
		crafted(Manifest{Root: "project", Dirs: []string{"a", "A"}}, ""),
		crafted(Manifest{Root: "project", Dirs: []string{"a/b"}}, ""),
		crafted(Manifest{Root: "project", Dirs: []string{"a"}, Files: []string{"a"}}, ""),
		crafted(Manifest{Root: "project", Files: []string{"file"}}, ""),
		crafted(Manifest{Root: "project", Files: []string{"file"}}, "\n### file\n\n```\nunclosed"),
		crafted(Manifest{Root: "project"}, "\n### extra\n\n```\nhello\n```\n"),
		"not a report",
	}
	for i, text := range cases {
		if _, err := parseReport(text); err == nil {
			t.Errorf("malformed case %d accepted", i)
		}
	}
}
