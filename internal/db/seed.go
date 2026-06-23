package db

import (
	"bytes"
	"compress/gzip"
	"database/sql"
	_ "embed"
	"fmt"
	"io"
	"os"
)

//go:embed wordnet_seed.db.gz
var seedGz []byte

// seedIfEmpty populates the words table from the embedded WordNet snapshot
// when the live DB has zero rows. Idempotent: noop on subsequent runs.
func seedIfEmpty(conn *sql.DB) error {
	var count int
	if err := conn.QueryRow(`SELECT COUNT(*) FROM words`).Scan(&count); err != nil {
		return fmt.Errorf("count words: %w", err)
	}
	if count > 0 {
		return nil
	}
	tmp, err := os.CreateTemp("", "vocab-seed-*.db")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	gz, err := gzip.NewReader(bytes.NewReader(seedGz))
	if err != nil {
		tmp.Close()
		return fmt.Errorf("gunzip seed: %w", err)
	}
	if _, err := io.Copy(tmp, gz); err != nil {
		tmp.Close()
		return fmt.Errorf("write seed tmp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if _, err := conn.Exec(fmt.Sprintf(`ATTACH DATABASE '%s' AS seed`, tmpPath)); err != nil {
		return fmt.Errorf("attach seed: %w", err)
	}
	defer conn.Exec(`DETACH DATABASE seed`)

	_, err = conn.Exec(`
        INSERT INTO words(name, definition, type, language, source)
        SELECT name, definition, type, 'en', 'wordnet' FROM seed.words
    `)
	if err != nil {
		return fmt.Errorf("copy seed: %w", err)
	}
	return nil
}

