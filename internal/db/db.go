package db

import (
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

// File is a tracked file row.
type File struct {
	ID      int64
	Path    string
	AddedAt time.Time
	Active  bool
}

// Version is a snapshot metadata row.
type Version struct {
	ID        int64
	FileID    int64
	Hash      string
	Size      int64
	CreatedAt time.Time
}

// DB wraps the SQLite connection and schema helpers.
type DB struct {
	sql *sql.DB
}

// Open opens (or creates) the database at path and runs migrations.
func Open(path string) (*DB, error) {
	sqlDB, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	// SQLite handles one writer; keep usage simple for the CLI.
	sqlDB.SetMaxOpenConns(1)

	if _, err := sqlDB.Exec(`PRAGMA foreign_keys = ON`); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("enable foreign keys: %w", err)
	}

	d := &DB{sql: sqlDB}
	if err := d.migrate(); err != nil {
		sqlDB.Close()
		return nil, err
	}
	return d, nil
}

// Close closes the underlying database.
func (d *DB) Close() error {
	return d.sql.Close()
}

func (d *DB) migrate() error {
	if _, err := d.sql.Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY
		)`); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	var current int
	err := d.sql.QueryRow(`SELECT COALESCE(MAX(version), 0) FROM schema_migrations`).Scan(&current)
	if err != nil {
		return fmt.Errorf("read migration version: %w", err)
	}

	migrations := []struct {
		version int
		sql     string
	}{
		{
			version: 1,
			sql: `
CREATE TABLE files (
	id INTEGER PRIMARY KEY,
	path TEXT NOT NULL UNIQUE,
	added_at TEXT NOT NULL,
	active INTEGER NOT NULL DEFAULT 1
);
CREATE TABLE versions (
	id INTEGER PRIMARY KEY,
	file_id INTEGER NOT NULL REFERENCES files(id),
	hash TEXT NOT NULL,
	size INTEGER NOT NULL,
	created_at TEXT NOT NULL,
	lines_added INTEGER,
	lines_removed INTEGER
);
CREATE INDEX idx_versions_file_id ON versions(file_id);
`,
		},
	}

	for _, m := range migrations {
		if m.version <= current {
			continue
		}
		tx, err := d.sql.Begin()
		if err != nil {
			return fmt.Errorf("begin migration %d: %w", m.version, err)
		}
		if _, err := tx.Exec(m.sql); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("apply migration %d: %w", m.version, err)
		}
		if _, err := tx.Exec(`INSERT INTO schema_migrations(version) VALUES (?)`, m.version); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("record migration %d: %w", m.version, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit migration %d: %w", m.version, err)
		}
	}
	return nil
}

// UpsertActiveFile inserts a file or reactivates an existing row. Returns the file id.
func (d *DB) UpsertActiveFile(path string, addedAt time.Time) (int64, error) {
	res, err := d.sql.Exec(`
		INSERT INTO files(path, added_at, active)
		VALUES (?, ?, 1)
		ON CONFLICT(path) DO UPDATE SET active = 1
	`, path, addedAt.UTC().Format(time.RFC3339Nano))
	if err != nil {
		return 0, fmt.Errorf("upsert file: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil || id == 0 {
		// ON CONFLICT update may not return last insert id; look up by path.
		err = d.sql.QueryRow(`SELECT id FROM files WHERE path = ?`, path).Scan(&id)
		if err != nil {
			return 0, fmt.Errorf("lookup file id: %w", err)
		}
	}
	return id, nil
}

// DeactivateFile marks a file inactive but keeps its history.
func (d *DB) DeactivateFile(path string) error {
	res, err := d.sql.Exec(`UPDATE files SET active = 0 WHERE path = ? AND active = 1`, path)
	if err != nil {
		return fmt.Errorf("deactivate file: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("file not tracked: %s", path)
	}
	return nil
}

// ListActiveFiles returns currently tracked files.
func (d *DB) ListActiveFiles() ([]File, error) {
	rows, err := d.sql.Query(`
		SELECT id, path, added_at, active
		FROM files
		WHERE active = 1
		ORDER BY path`)
	if err != nil {
		return nil, fmt.Errorf("list files: %w", err)
	}
	defer rows.Close()

	var out []File
	for rows.Next() {
		f, err := scanFile(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

// GetActiveFileByPath returns an active file by absolute path.
func (d *DB) GetActiveFileByPath(path string) (File, error) {
	row := d.sql.QueryRow(`
		SELECT id, path, added_at, active
		FROM files
		WHERE path = ? AND active = 1`, path)
	f, err := scanFile(row)
	if err == sql.ErrNoRows {
		return File{}, fmt.Errorf("file not tracked: %s", path)
	}
	if err != nil {
		return File{}, err
	}
	return f, nil
}

// InsertVersion appends a version row for a file.
func (d *DB) InsertVersion(fileID int64, hash string, size int64, createdAt time.Time) (int64, error) {
	res, err := d.sql.Exec(`
		INSERT INTO versions(file_id, hash, size, created_at)
		VALUES (?, ?, ?, ?)`,
		fileID, hash, size, createdAt.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return 0, fmt.Errorf("insert version: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	return id, nil
}

// LatestVersion returns the newest version for a file, or false if none exist.
func (d *DB) LatestVersion(fileID int64) (Version, bool, error) {
	row := d.sql.QueryRow(`
		SELECT id, file_id, hash, size, created_at
		FROM versions
		WHERE file_id = ?
		ORDER BY id DESC
		LIMIT 1`, fileID)
	v, err := scanVersion(row)
	if err == sql.ErrNoRows {
		return Version{}, false, nil
	}
	if err != nil {
		return Version{}, false, err
	}
	return v, true, nil
}

// CountVersions returns how many versions a file has.
func (d *DB) CountVersions(fileID int64) (int, error) {
	var n int
	err := d.sql.QueryRow(`SELECT COUNT(*) FROM versions WHERE file_id = ?`, fileID).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("count versions: %w", err)
	}
	return n, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanFile(row rowScanner) (File, error) {
	var (
		f       File
		addedAt string
		active  int
	)
	if err := row.Scan(&f.ID, &f.Path, &addedAt, &active); err != nil {
		return File{}, err
	}
	t, err := time.Parse(time.RFC3339Nano, addedAt)
	if err != nil {
		t, err = time.Parse(time.RFC3339, addedAt)
		if err != nil {
			return File{}, fmt.Errorf("parse added_at: %w", err)
		}
	}
	f.AddedAt = t
	f.Active = active != 0
	return f, nil
}

func scanVersion(row rowScanner) (Version, error) {
	var (
		v         Version
		createdAt string
	)
	if err := row.Scan(&v.ID, &v.FileID, &v.Hash, &v.Size, &createdAt); err != nil {
		return Version{}, err
	}
	t, err := time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		t, err = time.Parse(time.RFC3339, createdAt)
		if err != nil {
			return Version{}, fmt.Errorf("parse created_at: %w", err)
		}
	}
	v.CreatedAt = t
	return v, nil
}
