package main

import (
	"database/sql"
	"fmt"
	_ "modernc.org/sqlite"
	"os"
	"path"
	"path/filepath"
	"strings"
)

type RuleStore struct{ db *sql.DB }

func openRules(file string) (*RuleStore, error) {
	if err := os.MkdirAll(filepath.Dir(file), 0700); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", file)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	fail := func(err error) (*RuleStore, error) { db.Close(); return nil, err }
	if _, err = db.Exec(`CREATE TABLE IF NOT EXISTS excludes (pattern TEXT PRIMARY KEY); CREATE TABLE IF NOT EXISTS settings (key TEXT PRIMARY KEY); CREATE TABLE IF NOT EXISTS app_settings (key TEXT PRIMARY KEY, value INTEGER NOT NULL)`); err != nil {
		return fail(err)
	}
	s := &RuleStore{db: db}
	var initialized int
	if err = db.QueryRow(`SELECT COUNT(*) FROM settings WHERE key='excludes_initialized'`).Scan(&initialized); err != nil {
		return fail(err)
	}
	if initialized == 0 {
		if err = s.Save([]string{".git", "node_modules", ".env", ".env.*", "id_rsa", "id_ed25519"}); err != nil {
			return fail(err)
		}
	}
	tx, err := db.Begin()
	if err != nil {
		return fail(err)
	}
	result, err := tx.Exec(`INSERT OR IGNORE INTO app_settings(key,value) VALUES ('unlimited_default_v1',1)`)
	if err != nil {
		tx.Rollback()
		return fail(err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		tx.Rollback()
		return fail(err)
	}
	if count > 0 {
		if _, err = tx.Exec(`INSERT INTO app_settings(key,value) VALUES ('max_report_mib',0) ON CONFLICT(key) DO UPDATE SET value=0`); err != nil {
			tx.Rollback()
			return fail(err)
		}
	}
	if err = tx.Commit(); err != nil {
		return fail(err)
	}
	return s, nil
}
func (s *RuleStore) List() ([]string, error) {
	rows, err := s.db.Query(`SELECT pattern FROM excludes ORDER BY pattern`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	rules := []string{}
	for rows.Next() {
		var rule string
		if err = rows.Scan(&rule); err != nil {
			return nil, err
		}
		rules = append(rules, rule)
	}
	return rules, rows.Err()
}
func (s *RuleStore) Save(rules []string) error {
	if len(rules) > 500 {
		return fmt.Errorf("Use at most 500 exclusion rules")
	}
	normalized := map[string]bool{}
	for _, r := range rules {
		r = strings.TrimSpace(r)
		if r == "" {
			continue
		}
		if len(r) > 256 || strings.ContainsAny(r, "/\\") {
			return fmt.Errorf("Use a file or folder name pattern, without paths: %s", r)
		}
		if _, err := path.Match(r, ""); err != nil {
			return fmt.Errorf("Invalid pattern %q: %w", r, err)
		}
		normalized[r] = true
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.Exec(`DELETE FROM excludes`); err != nil {
		return err
	}
	for r := range normalized {
		if _, err = tx.Exec(`INSERT INTO excludes(pattern) VALUES (?)`, r); err != nil {
			return err
		}
	}
	if _, err = tx.Exec(`INSERT OR IGNORE INTO settings(key) VALUES ('excludes_initialized')`); err != nil {
		return err
	}
	return tx.Commit()
}
func matchesName(name string, rules []string) bool {
	for _, r := range rules {
		if ok, _ := path.Match(r, name); ok {
			return true
		}
	}
	return false
}
