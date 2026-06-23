package db

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

// Open opens (and creates if needed) the SQLite database at path, applies the
// schema, enables FK enforcement, and seeds the words table from the bundled
// WordNet snapshot on first run.
func Open(path string) (*sql.DB, error) {
	dsn := fmt.Sprintf("file:%s?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)", path)
	conn, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	if _, err := conn.Exec(Schema); err != nil {
		conn.Close()
		return nil, fmt.Errorf("apply schema: %w", err)
	}
	if err := seedIfEmpty(conn); err != nil {
		conn.Close()
		return nil, fmt.Errorf("seed: %w", err)
	}
	return conn, nil
}
