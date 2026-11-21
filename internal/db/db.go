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
	Missing bool
}

// Version is a snapshot metadata row.
type Version struct {
	ID           int64
	FileID       int64
	Hash         string
	Size         int64
	CreatedAt    time.Time
	LinesAdded   *int // nil when unknown / binary / first version
	LinesRemoved *int
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
		{
			version: 2,
			sql: `
ALTER TABLE files ADD COLUMN missing INTEGER NOT NULL DEFAULT 0;
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
		INSERT INTO files(path, added_at, active, missing)
		VALUES (?, ?, 1, 0)
		ON CONFLICT(path) DO UPDATE SET active = 1, missing = 0
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
		SELECT id, path, added_at, active, missing
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
		SELECT id, path, added_at, active, missing
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

// GetFileByPath returns a file by absolute path, active or not.
func (d *DB) GetFileByPath(path string) (File, error) {
	row := d.sql.QueryRow(`
		SELECT id, path, added_at, active, missing
		FROM files
		WHERE path = ?`, path)
	f, err := scanFile(row)
	if err == sql.ErrNoRows {
		return File{}, fmt.Errorf("file not known: %s", path)
	}
	if err != nil {
		return File{}, err
	}
	return f, nil
}

// ListVersions returns versions for a file oldest-first (version 1, 2, ...).
func (d *DB) ListVersions(fileID int64) ([]Version, error) {
	rows, err := d.sql.Query(`
		SELECT id, file_id, hash, size, created_at, lines_added, lines_removed
		FROM versions
		WHERE file_id = ?
		ORDER BY id ASC`, fileID)
	if err != nil {
		return nil, fmt.Errorf("list versions: %w", err)
	}
	defer rows.Close()

	var out []Version
	for rows.Next() {
		v, err := scanVersion(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// VersionByNumber returns the 1-based version for a file (oldest = 1).
func (d *DB) VersionByNumber(fileID int64, number int) (Version, error) {
	if number < 1 {
		return Version{}, fmt.Errorf("version must be >= 1")
	}
	row := d.sql.QueryRow(`
		SELECT id, file_id, hash, size, created_at, lines_added, lines_removed
		FROM versions
		WHERE file_id = ?
		ORDER BY id ASC
		LIMIT 1 OFFSET ?`, fileID, number-1)
	v, err := scanVersion(row)
	if err == sql.ErrNoRows {
		return Version{}, fmt.Errorf("version %d not found", number)
	}
	if err != nil {
		return Version{}, err
	}
	return v, nil
}

// InsertVersion appends a version row for a file.
// linesAdded/linesRemoved may be nil when stats are unknown (first version or binary).
func (d *DB) InsertVersion(fileID int64, hash string, size int64, createdAt time.Time, linesAdded, linesRemoved *int) (int64, error) {
	res, err := d.sql.Exec(`
		INSERT INTO versions(file_id, hash, size, created_at, lines_added, lines_removed)
		VALUES (?, ?, ?, ?, ?, ?)`,
		fileID, hash, size, createdAt.UTC().Format(time.RFC3339Nano),
		nullInt(linesAdded), nullInt(linesRemoved),
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

func nullInt(v *int) any {
	if v == nil {
		return nil
	}
	return *v
}

// LatestVersion returns the newest version for a file, or false if none exist.
func (d *DB) LatestVersion(fileID int64) (Version, bool, error) {
	row := d.sql.QueryRow(`
		SELECT id, file_id, hash, size, created_at, lines_added, lines_removed
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

// VersionEvent is a version joined with its file path for stats aggregation.
type VersionEvent struct {
	Path         string
	Size         int64
	CreatedAt    time.Time
	LinesAdded   *int
	LinesRemoved *int
}

// ListActiveVersionEvents returns all versions for active files, oldest first.
func (d *DB) ListActiveVersionEvents() ([]VersionEvent, error) {
	rows, err := d.sql.Query(`
		SELECT f.path, v.size, v.created_at, v.lines_added, v.lines_removed
		FROM versions v
		INNER JOIN files f ON f.id = v.file_id
		WHERE f.active = 1
		ORDER BY v.created_at ASC, v.id ASC`)
	if err != nil {
		return nil, fmt.Errorf("list version events: %w", err)
	}
	defer rows.Close()

	var out []VersionEvent
	for rows.Next() {
		var (
			ev           VersionEvent
			createdAt    string
			linesAdded   sql.NullInt64
			linesRemoved sql.NullInt64
		)
		if err := rows.Scan(&ev.Path, &ev.Size, &createdAt, &linesAdded, &linesRemoved); err != nil {
			return nil, err
		}
		t, err := time.Parse(time.RFC3339Nano, createdAt)
		if err != nil {
			t, err = time.Parse(time.RFC3339, createdAt)
			if err != nil {
				return nil, fmt.Errorf("parse created_at: %w", err)
			}
		}
		ev.CreatedAt = t
		if linesAdded.Valid {
			n := int(linesAdded.Int64)
			ev.LinesAdded = &n
		}
		if linesRemoved.Valid {
			n := int(linesRemoved.Int64)
			ev.LinesRemoved = &n
		}
		out = append(out, ev)
	}
	return out, rows.Err()
}

// SetMissing marks whether a tracked file is currently missing on disk.
func (d *DB) SetMissing(path string, missing bool) error {
	flag := 0
	if missing {
		flag = 1
	}
	res, err := d.sql.Exec(`UPDATE files SET missing = ? WHERE path = ? AND active = 1`, flag, path)
	if err != nil {
		return fmt.Errorf("set missing: %w", err)
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

type rowScanner interface {
	Scan(dest ...any) error
}

func scanFile(row rowScanner) (File, error) {
	var (
		f       File
		addedAt string
		active  int
		missing int
	)
	if err := row.Scan(&f.ID, &f.Path, &addedAt, &active, &missing); err != nil {
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
	f.Missing = missing != 0
	return f, nil
}

func scanVersion(row rowScanner) (Version, error) {
	var (
		v            Version
		createdAt    string
		linesAdded   sql.NullInt64
		linesRemoved sql.NullInt64
	)
	if err := row.Scan(&v.ID, &v.FileID, &v.Hash, &v.Size, &createdAt, &linesAdded, &linesRemoved); err != nil {
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
	if linesAdded.Valid {
		n := int(linesAdded.Int64)
		v.LinesAdded = &n
	}
	if linesRemoved.Valid {
		n := int(linesRemoved.Int64)
		v.LinesRemoved = &n
	}
	return v, nil
}
