package db

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

// FileLink is an undirected edge between two tracked files.
type FileLink struct {
	ID        int64
	AID       int64
	BID       int64
	APath     string
	BPath     string
	Note      string
	CreatedAt time.Time
}

// LinkFiles creates an undirected link between two file ids.
// note may be empty.
func (d *DB) LinkFiles(aID, bID int64, note string) (FileLink, error) {
	if aID == bID {
		return FileLink{}, fmt.Errorf("cannot link a file to itself")
	}
	if aID > bID {
		aID, bID = bID, aID
	}
	clean := strings.TrimSpace(note)
	if utf8.RuneCountInString(clean) > 120 {
		clean = string([]rune(clean)[:120])
		clean = strings.TrimSpace(clean)
	}
	var noteArg any
	if clean != "" {
		noteArg = clean
	}
	createdAt := time.Now().UTC()
	res, err := d.sql.Exec(`
		INSERT INTO file_links(a_id, b_id, note, created_at)
		VALUES (?, ?, ?, ?)`,
		aID, bID, noteArg, createdAt.Format(time.RFC3339Nano))
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			return FileLink{}, fmt.Errorf("those files are already linked")
		}
		return FileLink{}, fmt.Errorf("link files: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return FileLink{}, err
	}
	return d.GetLink(id)
}

// UnlinkFiles removes the undirected link between two file ids.
func (d *DB) UnlinkFiles(aID, bID int64) error {
	if aID > bID {
		aID, bID = bID, aID
	}
	res, err := d.sql.Exec(`DELETE FROM file_links WHERE a_id = ? AND b_id = ?`, aID, bID)
	if err != nil {
		return fmt.Errorf("unlink files: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("no link between those files")
	}
	return nil
}

// GetLink returns a link by id.
func (d *DB) GetLink(id int64) (FileLink, error) {
	row := d.sql.QueryRow(`
		SELECT l.id, l.a_id, l.b_id, a.path, b.path, l.note, l.created_at
		FROM file_links l
		INNER JOIN files a ON a.id = l.a_id
		INNER JOIN files b ON b.id = l.b_id
		WHERE l.id = ?`, id)
	return scanLink(row)
}

// ListLinks returns all links (including inactive files).
func (d *DB) ListLinks() ([]FileLink, error) {
	rows, err := d.sql.Query(`
		SELECT l.id, l.a_id, l.b_id, a.path, b.path, l.note, l.created_at
		FROM file_links l
		INNER JOIN files a ON a.id = l.a_id
		INNER JOIN files b ON b.id = l.b_id
		ORDER BY l.id ASC`)
	if err != nil {
		return nil, fmt.Errorf("list links: %w", err)
	}
	defer rows.Close()
	return scanLinks(rows)
}

// ListActiveLinks returns links where both ends are active tracked files.
func (d *DB) ListActiveLinks() ([]FileLink, error) {
	rows, err := d.sql.Query(`
		SELECT l.id, l.a_id, l.b_id, a.path, b.path, l.note, l.created_at
		FROM file_links l
		INNER JOIN files a ON a.id = l.a_id
		INNER JOIN files b ON b.id = l.b_id
		WHERE a.active = 1 AND b.active = 1
		ORDER BY l.id ASC`)
	if err != nil {
		return nil, fmt.Errorf("list active links: %w", err)
	}
	defer rows.Close()
	return scanLinks(rows)
}

// ListLinksForFile returns links touching a file id.
func (d *DB) ListLinksForFile(fileID int64) ([]FileLink, error) {
	rows, err := d.sql.Query(`
		SELECT l.id, l.a_id, l.b_id, a.path, b.path, l.note, l.created_at
		FROM file_links l
		INNER JOIN files a ON a.id = l.a_id
		INNER JOIN files b ON b.id = l.b_id
		WHERE l.a_id = ? OR l.b_id = ?
		ORDER BY l.id ASC`, fileID, fileID)
	if err != nil {
		return nil, fmt.Errorf("list links for file: %w", err)
	}
	defer rows.Close()
	return scanLinks(rows)
}

func scanLinks(rows *sql.Rows) ([]FileLink, error) {
	var out []FileLink
	for rows.Next() {
		link, err := scanLink(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, link)
	}
	return out, rows.Err()
}

func scanLink(row rowScanner) (FileLink, error) {
	var (
		l         FileLink
		note      sql.NullString
		createdAt string
	)
	if err := row.Scan(&l.ID, &l.AID, &l.BID, &l.APath, &l.BPath, &note, &createdAt); err != nil {
		if err == sql.ErrNoRows {
			return FileLink{}, fmt.Errorf("link not found")
		}
		return FileLink{}, err
	}
	if note.Valid {
		l.Note = note.String
	}
	t, err := time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		t, err = time.Parse(time.RFC3339, createdAt)
		if err != nil {
			return FileLink{}, fmt.Errorf("parse link created_at: %w", err)
		}
	}
	l.CreatedAt = t
	return l, nil
}
