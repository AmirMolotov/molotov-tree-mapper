package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const historyName = ".molotov-history.sqlite"

type snapshot map[string]*string
type FileChange struct {
	Path   string `json:"path"`
	Status string `json:"status"`
}
type Version struct {
	Changes []FileChange `json:"changes"`
	ID      int64        `json:"id"`
	Label   string       `json:"label"`
	Created string       `json:"created"`
	Files   []string     `json:"files"`
}
type History struct {
	Root     string    `json:"root"`
	Versions []Version `json:"versions"`
	Pending  bool      `json:"pending"`
}

func openHistory(root string) (*sql.DB, error) {
	if _, err := checkedPath(root, ""); err != nil {
		return nil, err
	}
	file := filepath.Join(root, historyName)
	if st, err := os.Lstat(file); err == nil && (!st.Mode().IsRegular() || st.Mode()&os.ModeSymlink != 0) {
		return nil, fmt.Errorf("Unsafe history file")
	}
	db, err := sql.Open("sqlite", file)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if _, err = db.Exec(`PRAGMA busy_timeout=5000; CREATE TABLE IF NOT EXISTS versions(id INTEGER PRIMARY KEY, label TEXT NOT NULL, created TEXT NOT NULL DEFAULT(strftime('%Y-%m-%dT%H:%M:%fZ','now')), snapshot TEXT NOT NULL); CREATE TABLE IF NOT EXISTS pending(id INTEGER PRIMARY KEY CHECK(id=1), before_id INTEGER, after_id INTEGER)`); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}
func readSnapshot(db *sql.DB, id int64) (snapshot, error) {
	var data string
	err := db.QueryRow(`SELECT snapshot FROM versions WHERE id=?`, id).Scan(&data)
	if err != nil {
		return nil, err
	}
	if len(data) > maxImport {
		return nil, fmt.Errorf("History snapshot exceeds supported size")
	}
	var s snapshot
	if err = json.Unmarshal([]byte(data), &s); err != nil {
		return nil, err
	}
	if len(s) > maxNodes {
		return nil, fmt.Errorf("Too many history paths")
	}
	for name := range s {
		if err = validManagedPath(name); err != nil {
			return nil, err
		}
	}
	return s, nil
}
func validManagedPath(name string) error {
	if err := validRelative(name); err != nil {
		return err
	}
	if strings.HasPrefix(strings.ToLower(strings.Split(name, "/")[0]), historyName) {
		return fmt.Errorf("History files cannot be imported or reverted")
	}
	return nil
}
func pendingIDs(db *sql.DB) (int64, int64, error) {
	var before, after int64
	err := db.QueryRow(`SELECT before_id,after_id FROM pending WHERE id=1`).Scan(&before, &after)
	if err == sql.ErrNoRows {
		return 0, 0, nil
	}
	return before, after, err
}
func currentFile(root, name string) (*string, error) {
	if err := validManagedPath(name); err != nil {
		return nil, err
	}
	full := filepath.Join(root, filepath.FromSlash(name))
	if _, err := os.Lstat(full); os.IsNotExist(err) {
		// Missing files still require checking any existing parent for links.
		if err = ensureParents(root, name, false); err != nil {
			return nil, err
		}
		return nil, nil
	}
	data, err := safeRead(root, name)
	if err != nil {
		return nil, fmt.Errorf("Cannot version %s: %w", name, err)
	}
	text := string(data)
	return &text, nil
}
func ensureParents(root, name string, create bool) error {
	parts := strings.Split(filepath.ToSlash(filepath.Dir(name)), "/")
	current := root
	for _, part := range parts {
		if part == "." {
			continue
		}
		current = filepath.Join(current, part)
		st, err := os.Lstat(current)
		if os.IsNotExist(err) {
			if !create {
				continue
			}
			if err = os.Mkdir(current, 0700); err != nil {
				return err
			}
			continue
		}
		if err != nil {
			return err
		}
		if !st.IsDir() || st.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("Unsafe parent directory: %s", current)
		}
	}
	return nil
}
func same(a, b *string) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}
func applySnapshot(root string, desired, expected snapshot, order ...[]string) error {
	names := make([]string, 0, len(desired))
	for name := range desired {
		names = append(names, name)
	}
	sort.Strings(names)
	if len(order) > 0 {
		prioritized := []string{}
		seen := map[string]bool{}
		for _, name := range order[0] {
			if _, ok := desired[name]; ok && !seen[name] {
				prioritized = append(prioritized, name)
				seen[name] = true
			}
		}
		for _, name := range names {
			if !seen[name] {
				prioritized = append(prioritized, name)
			}
		}
		names = prioritized
	}
	// Preflight all paths before the first write. Recovery accepts either side of a partial operation.
	for _, name := range names {
		current, err := currentFile(root, name)
		if err != nil {
			return err
		}
		if !same(current, expected[name]) && !same(current, desired[name]) {
			return fmt.Errorf("%s changed outside Molotov; preserve that edit before recovery", name)
		}
	}
	for _, name := range names {
		want := desired[name]
		current, err := currentFile(root, name)
		if err != nil {
			return err
		}
		if same(current, want) {
			continue
		}
		if !same(current, expected[name]) {
			return fmt.Errorf("%s changed during update", name)
		}
		full := filepath.Join(root, filepath.FromSlash(name))
		if want == nil {
			if err = os.Remove(full); err != nil {
				return err
			}
			continue
		}
		if err = ensureParents(root, name, true); err != nil {
			return err
		}
		mode := os.FileMode(0600)
		if st, err := os.Stat(full); err == nil {
			mode = st.Mode().Perm()
		}
		f, err := os.CreateTemp(filepath.Dir(full), ".molotov-write-")
		if err != nil {
			return err
		}
		tmp := f.Name()
		_, err = f.WriteString(*want)
		if err == nil {
			err = f.Chmod(mode)
		}
		closeErr := f.Close()
		if err == nil {
			err = closeErr
		}
		if err == nil {
			err = os.Rename(tmp, full)
		}
		os.Remove(tmp)
		if err != nil {
			return err
		}
	}
	return nil
}
func versionedChange(root, label string, changes snapshot, order ...[]string) (int64, error) {
	db, err := openHistory(root)
	if err != nil {
		return 0, err
	}
	defer db.Close()
	tx, err := db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	// Acquire the SQLite writer lock before checking the journal or filesystem.
	if _, err = tx.Exec(`UPDATE pending SET id=id`); err != nil {
		return 0, err
	}
	var pending int
	if err = tx.QueryRow(`SELECT count(*) FROM pending`).Scan(&pending); err != nil {
		return 0, err
	}
	if pending != 0 {
		return 0, fmt.Errorf("Interrupted change found. Recover it from History before continuing")
	}
	var previous string
	err = tx.QueryRow(`SELECT snapshot FROM versions ORDER BY id DESC LIMIT 1`).Scan(&previous)
	if err != nil && err != sql.ErrNoRows {
		return 0, err
	}
	managed := snapshot{}
	if len(previous) > maxImport {
		return 0, fmt.Errorf("History snapshot exceeds supported size")
	}
	if previous != "" {
		if err = json.Unmarshal([]byte(previous), &managed); err != nil {
			return 0, err
		}
	}
	if len(managed)+len(changes) > maxNodes {
		return 0, fmt.Errorf("Too many managed history files")
	}
	for name := range changes {
		managed[name] = nil
	}
	seenNames := map[string]bool{}
	for name := range managed {
		fold := strings.ToLower(name)
		if seenNames[fold] {
			return 0, fmt.Errorf("Case-conflicting managed file: %s", name)
		}
		seenNames[fold] = true
	}
	before, after := snapshot{}, snapshot{}
	total := 0
	for name := range managed {
		current, err := currentFile(root, name)
		if err != nil {
			return 0, err
		}
		before[name] = current
		after[name] = current
		if current != nil {
			total += len(*current)
		}
	}
	for name, value := range changes {
		after[name] = value
		if value != nil {
			total += len(*value)
		}
	}
	totalAfter := 0
	for _, value := range after {
		if value != nil {
			totalAfter += len(*value)
		}
	}
	if totalAfter > maxReport || total > 2*maxReport {
		return 0, fmt.Errorf("Managed history snapshot exceeds 128 MiB")
	}
	oldJSON, err := json.Marshal(before)
	if err != nil {
		return 0, err
	}
	newJSON, err := json.Marshal(after)
	if err != nil {
		return 0, err
	}
	if len(oldJSON) > maxImport || len(newJSON) > maxImport {
		return 0, fmt.Errorf("Encoded history snapshot exceeds supported size")
	}
	result, err := tx.Exec(`INSERT INTO versions(label,snapshot) VALUES (?,?)`, "Before "+label, string(oldJSON))
	if err != nil {
		return 0, err
	}
	beforeID, _ := result.LastInsertId()
	result, err = tx.Exec(`INSERT INTO versions(label,snapshot) VALUES (?,?)`, label, string(newJSON))
	if err != nil {
		return 0, err
	}
	afterID, _ := result.LastInsertId()
	if _, err = tx.Exec(`INSERT INTO pending(id,before_id,after_id) VALUES (1,?,?)`, beforeID, afterID); err != nil {
		return 0, err
	}
	if err = tx.Commit(); err != nil {
		return 0, err
	}
	if err = applySnapshot(root, after, before, order...); err != nil {
		if rollbackErr := applySnapshot(root, before, after); rollbackErr != nil {
			return 0, fmt.Errorf("Update failed (%v); recovery is required (%v)", err, rollbackErr)
		}
		if _, cleanupErr := db.Exec(`DELETE FROM versions WHERE id=?; DELETE FROM pending WHERE id=1`, afterID); cleanupErr != nil {
			return 0, fmt.Errorf("Update rolled back; journal cleanup requires recovery: %w", cleanupErr)
		}
		return 0, err
	}
	if _, err = db.Exec(`DELETE FROM pending WHERE id=1`); err != nil {
		return 0, fmt.Errorf("Files written; journal needs recovery before another change: %w", err)
	}
	return afterID, nil
}
func loadHistory(root string) (History, error) {
	h := History{Root: root, Versions: []Version{}}
	db, err := openHistory(root)
	if err != nil {
		return h, err
	}
	defer db.Close()
	before, _, err := pendingIDs(db)
	if err != nil {
		return h, err
	}
	h.Pending = before != 0
	rows, err := db.Query(`SELECT id,label,created FROM versions WHERE id NOT IN (SELECT after_id FROM pending) ORDER BY id DESC`)
	if err != nil {
		return h, err
	}
	for rows.Next() {
		var v Version
		if err = rows.Scan(&v.ID, &v.Label, &v.Created); err != nil {
			rows.Close()
			return h, err
		}
		h.Versions = append(h.Versions, v)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return h, err
	}
	previous := snapshot{}
	for i := len(h.Versions) - 1; i >= 0; i-- {
		s, err := readSnapshot(db, h.Versions[i].ID)
		if err != nil {
			return h, err
		}
		h.Versions[i].Changes = []FileChange{}
		names := map[string]bool{}
		for name := range previous {
			names[name] = true
		}
		for name := range s {
			names[name] = true
			h.Versions[i].Files = append(h.Versions[i].Files, name)
		}
		for name := range names {
			before, after := previous[name], s[name]
			if same(before, after) {
				continue
			}
			status := "Modified"
			if before == nil {
				status = "Added"
				if strings.HasPrefix(h.Versions[i].Label, "Before ") {
					status = "Existing"
				}
			} else if after == nil {
				status = "Removed"
			}
			h.Versions[i].Changes = append(h.Versions[i].Changes, FileChange{name, status})
		}
		sort.Strings(h.Versions[i].Files)
		sort.Slice(h.Versions[i].Changes, func(a, b int) bool { return h.Versions[i].Changes[a].Path < h.Versions[i].Changes[b].Path })
		previous = s
	}

	return h, nil
}
func revertVersion(root string, id int64, file string) (int64, error) {
	db, err := openHistory(root)
	if err != nil {
		return 0, err
	}
	desired, err := readSnapshot(db, id)
	db.Close()
	if err != nil {
		return 0, err
	}
	if file != "" {
		if err = validManagedPath(file); err != nil {
			return 0, err
		}
		value, ok := desired[file]
		if !ok {
			return 0, fmt.Errorf("File is not tracked in this version")
		}
		desired = snapshot{file: value}
	}
	if file == "" {
		db, err = openHistory(root)
		if err != nil {
			return 0, err
		}
		var latest int64
		err = db.QueryRow(`SELECT max(id) FROM versions`).Scan(&latest)
		if err != nil {
			db.Close()
			return 0, err
		}
		tracked, err := readSnapshot(db, latest)
		db.Close()
		if err != nil {
			return 0, err
		}
		for name := range tracked {
			if _, ok := desired[name]; !ok {
				desired[name] = nil
			}
		}
	}
	return versionedChange(root, fmt.Sprintf("Revert version %d %s", id, file), desired)
}
func recoverHistory(root string) error {
	db, err := openHistory(root)
	if err != nil {
		return err
	}
	defer db.Close()
	before, after, err := pendingIDs(db)
	if err != nil || before == 0 {
		return err
	}
	old, err := readSnapshot(db, before)
	if err != nil {
		return err
	}
	next, err := readSnapshot(db, after)
	if err != nil {
		return err
	}
	if err = applySnapshot(root, old, next); err != nil {
		return err
	}
	_, err = db.Exec(`DELETE FROM versions WHERE id=?; DELETE FROM pending WHERE id=1`, after)
	return err
}
