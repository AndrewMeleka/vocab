package db

import (
	"database/sql"
	"errors"
	"fmt"
	"os"

	_ "modernc.org/sqlite"
)

// Open opens the SQLite database at path. When the file does not yet exist it is
// created, the schema applied, and the words table seeded from the bundled
// WordNet snapshot. An existing file is opened as-is (schema only re-applied for
// any missing tables); it is never re-seeded.
func Open(path string) (*sql.DB, error) {
	fresh := false
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		fresh = true
	} else if err != nil {
		return nil, fmt.Errorf("stat db: %w", err)
	}

	dsn := fmt.Sprintf("file:%s?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)", path)
	conn, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	if _, err := conn.Exec(Schema); err != nil {
		conn.Close()
		return nil, fmt.Errorf("apply schema: %w", err)
	}
	if fresh {
		if err := seed(conn); err != nil {
			conn.Close()
			return nil, fmt.Errorf("seed: %w", err)
		}
	}
	return conn, nil
}
