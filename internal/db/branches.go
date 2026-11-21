package db

import (
	"database/sql"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

var branchNameCleaner = regexp.MustCompile(`[^a-z0-9._/-]+`)

// NormalizeBranchName cleans and validates a branch name.
func NormalizeBranchName(raw string) (string, error) {
	name := strings.ToLower(strings.TrimSpace(raw))
	name = strings.ReplaceAll(name, " ", "-")
	name = branchNameCleaner.ReplaceAllString(name, "")
	name = strings.Trim(name, "/.-")
	if name == "" {
		return "", fmt.Errorf("branch name is required")
	}
	if utf8.RuneCountInString(name) > 40 {
		name = string([]rune(name)[:40])
		name = strings.Trim(name, "/.-")
	}
	if name == "" {
		return "", fmt.Errorf("branch name is required")
	}
	return name, nil
}

// EnsureFileBranchDefaults creates the main branch and file_state row if missing.
func (d *DB) EnsureFileBranchDefaults(fileID int64, createdAt time.Time) error {
	if createdAt.IsZero() {
		createdAt = time.Now()
	}
	_, err := d.sql.Exec(`
		INSERT OR IGNORE INTO branches(file_id, name, from_version, created_at)
		VALUES (?, 'main', 0, ?)`,
		fileID, createdAt.UTC().Format(time.RFC3339Nano))
	if err != nil {
		return fmt.Errorf("ensure main branch: %w", err)
	}
	_, err = d.sql.Exec(`
		INSERT OR IGNORE INTO file_state(file_id, current_branch)
		VALUES (?, 'main')`, fileID)
	if err != nil {
		return fmt.Errorf("ensure file state: %w", err)
	}
	return nil
}

// ListBranches returns branches for a file, main first then by creation.
func (d *DB) ListBranches(fileID int64) ([]Branch, error) {
	if err := d.EnsureFileBranchDefaults(fileID, time.Time{}); err != nil {
		return nil, err
	}
	rows, err := d.sql.Query(`
		SELECT id, file_id, name, from_version, created_at
		FROM branches
		WHERE file_id = ?
		ORDER BY CASE WHEN name = 'main' THEN 0 ELSE 1 END, id ASC`, fileID)
	if err != nil {
		return nil, fmt.Errorf("list branches: %w", err)
	}
	defer rows.Close()

	var out []Branch
	for rows.Next() {
		b, err := scanBranch(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// GetBranch returns a branch by name for a file.
func (d *DB) GetBranch(fileID int64, name string) (Branch, error) {
	row := d.sql.QueryRow(`
		SELECT id, file_id, name, from_version, created_at
		FROM branches
		WHERE file_id = ? AND name = ?`, fileID, name)
	b, err := scanBranch(row)
	if err == sql.ErrNoRows {
		return Branch{}, fmt.Errorf("unknown branch: %s", name)
	}
	if err != nil {
		return Branch{}, err
	}
	return b, nil
}

// CurrentBranch returns the checked-out branch name for a file.
func (d *DB) CurrentBranch(fileID int64) (string, error) {
	if err := d.EnsureFileBranchDefaults(fileID, time.Time{}); err != nil {
		return "", err
	}
	var name string
	err := d.sql.QueryRow(`SELECT current_branch FROM file_state WHERE file_id = ?`, fileID).Scan(&name)
	if err == sql.ErrNoRows {
		return "main", nil
	}
	if err != nil {
		return "", fmt.Errorf("current branch: %w", err)
	}
	if name == "" {
		return "main", nil
	}
	return name, nil
}

// SetCurrentBranch switches the checked-out branch for a file.
func (d *DB) SetCurrentBranch(fileID int64, name string) error {
	if _, err := d.GetBranch(fileID, name); err != nil {
		return err
	}
	res, err := d.sql.Exec(`
		INSERT INTO file_state(file_id, current_branch) VALUES (?, ?)
		ON CONFLICT(file_id) DO UPDATE SET current_branch = excluded.current_branch`,
		fileID, name)
	if err != nil {
		return fmt.Errorf("set current branch: %w", err)
	}
	_, err = res.RowsAffected()
	return err
}

// CreateBranch forks a named branch from a version number and checks it out.
func (d *DB) CreateBranch(fileID int64, rawName string, fromVersion int) (Branch, error) {
	name, err := NormalizeBranchName(rawName)
	if err != nil {
		return Branch{}, err
	}
	if name == "main" {
		return Branch{}, fmt.Errorf("main already exists")
	}
	if fromVersion < 0 {
		return Branch{}, fmt.Errorf("from version must be >= 0")
	}
	if fromVersion > 0 {
		if _, err := d.VersionByNumber(fileID, fromVersion); err != nil {
			return Branch{}, err
		}
	}

	createdAt := time.Now().UTC()
	_, err = d.sql.Exec(`
		INSERT INTO branches(file_id, name, from_version, created_at)
		VALUES (?, ?, ?, ?)`,
		fileID, name, fromVersion, createdAt.Format(time.RFC3339Nano))
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			return Branch{}, fmt.Errorf("branch %q already exists", name)
		}
		return Branch{}, fmt.Errorf("create branch: %w", err)
	}
	if err := d.SetCurrentBranch(fileID, name); err != nil {
		return Branch{}, err
	}
	return d.GetBranch(fileID, name)
}

// SetVersionTitle sets or clears the title for a 1-based version number.
func (d *DB) SetVersionTitle(fileID int64, versionNumber int, title string) error {
	v, err := d.VersionByNumber(fileID, versionNumber)
	if err != nil {
		return err
	}
	clean := strings.TrimSpace(title)
	if utf8.RuneCountInString(clean) > 80 {
		clean = string([]rune(clean)[:80])
		clean = strings.TrimSpace(clean)
	}
	var arg any
	if clean == "" {
		arg = nil
	} else {
		arg = clean
	}
	_, err = d.sql.Exec(`UPDATE versions SET title = ? WHERE id = ?`, arg, v.ID)
	if err != nil {
		return fmt.Errorf("set version title: %w", err)
	}
	return nil
}

func scanBranch(row rowScanner) (Branch, error) {
	var (
		b         Branch
		createdAt string
	)
	if err := row.Scan(&b.ID, &b.FileID, &b.Name, &b.FromVersion, &createdAt); err != nil {
		return Branch{}, err
	}
	t, err := time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		t, err = time.Parse(time.RFC3339, createdAt)
		if err != nil {
			return Branch{}, fmt.Errorf("parse branch created_at: %w", err)
		}
	}
	b.CreatedAt = t
	return b, nil
}
