package db

import (
	"database/sql"
	"errors"
	"fmt"
	"os"

	_ "modernc.org/sqlite"
)

// Open opens the SQLite database at path. When the file does not yet exist it is
// created and the schema applied. The words table is seeded from the bundled
// WordNet snapshot whenever it is empty — so a brand-new database, or a stale
// one left behind by an older/interrupted run, self-heals on next open. A words
// table that already holds rows is never re-seeded.
func Open(path string) (*sql.DB, error) {
	if _, err := os.Stat(path); err != nil && !errors.Is(err, os.ErrNotExist) {
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

	var words int
	if err := conn.QueryRow(`SELECT COUNT(*) FROM words`).Scan(&words); err != nil {
		conn.Close()
		return nil, fmt.Errorf("count words: %w", err)
	}
	if words == 0 {
		if err := seed(conn); err != nil {
			conn.Close()
			return nil, fmt.Errorf("seed: %w", err)
		}
	}
	return conn, nil
}
