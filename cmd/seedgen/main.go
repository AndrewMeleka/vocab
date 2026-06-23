// seedgen builds wordnet_seed.db.gz from raw WordNet 3.1 data.* files.
//
// NOT part of the shipped binary; run at build time only:
//
//	go run ./cmd/seedgen --src /tmp/wordnet-dl/dict --out internal/db/wordnet_seed.db.gz
package main

import (
	"bufio"
	"compress/gzip"
	"database/sql"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"
)

var posMap = map[string]string{
	"n": "noun",
	"v": "verb",
	"a": "adj",
	"s": "adj",
	"r": "adv",
}

// posPriority: when a lemma appears with multiple POS tags, keep the highest-priority sense.
var posPriority = map[string]int{"noun": 4, "verb": 3, "adj": 2, "adv": 1}

type entry struct {
	name       string
	pos        string
	definition string
}

func main() {
	src := flag.String("src", "", "directory containing data.{noun,verb,adj,adv}")
	out := flag.String("out", "internal/db/wordnet_seed.db.gz", "output gzipped sqlite file")
	flag.Parse()
	if *src == "" {
		log.Fatal("--src is required")
	}

	entries := map[string]entry{} // lemma (lowercase) -> best entry

	for _, name := range []string{"data.noun", "data.verb", "data.adj", "data.adv"} {
		path := filepath.Join(*src, name)
		f, err := os.Open(path)
		if err != nil {
			log.Fatalf("open %s: %v", path, err)
		}
		s := bufio.NewScanner(f)
		s.Buffer(make([]byte, 0, 1024*1024), 1024*1024)
		for s.Scan() {
			line := s.Text()
			if strings.HasPrefix(line, "  ") {
				continue // license header
			}
			parsed := parseLine(line)
			for _, e := range parsed {
				key := strings.ToLower(e.name)
				if existing, ok := entries[key]; ok {
					if posPriority[e.pos] <= posPriority[existing.pos] {
						continue
					}
				}
				entries[key] = e
			}
		}
		f.Close()
		log.Printf("processed %s: %d unique lemmas so far", name, len(entries))
	}

	tmpDB := *out + ".tmp.db"
	_ = os.Remove(tmpDB)
	conn, err := sql.Open("sqlite", tmpDB)
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()
	if _, err := conn.Exec(`CREATE TABLE words (
        name TEXT NOT NULL UNIQUE COLLATE NOCASE,
        definition TEXT NOT NULL,
        type TEXT NOT NULL
    )`); err != nil {
		log.Fatal(err)
	}
	tx, err := conn.Begin()
	if err != nil {
		log.Fatal(err)
	}
	stmt, err := tx.Prepare(`INSERT INTO words(name, definition, type) VALUES(?,?,?)`)
	if err != nil {
		log.Fatal(err)
	}
	n := 0
	for _, e := range entries {
		if _, err := stmt.Exec(strings.ToLower(e.name), e.definition, e.pos); err != nil {
			log.Fatal(err)
		}
		n++
	}
	stmt.Close()
	if err := tx.Commit(); err != nil {
		log.Fatal(err)
	}
	log.Printf("inserted %d rows into %s", n, tmpDB)

	if err := gzipFile(tmpDB, *out); err != nil {
		log.Fatal(err)
	}
	_ = os.Remove(tmpDB)
	info, _ := os.Stat(*out)
	log.Printf("wrote %s (%d bytes)", *out, info.Size())
}

// parseLine extracts (lemma, pos, definition) entries from one WordNet data line.
// Skips multi-word lemmas (containing '_') and lemmas with non-letter chars.
func parseLine(line string) []entry {
	pipe := strings.Index(line, "|")
	if pipe < 0 {
		return nil
	}
	header := line[:pipe]
	gloss := strings.TrimSpace(line[pipe+1:])
	def := gloss
	if i := strings.Index(def, ";"); i >= 0 {
		def = def[:i]
	}
	def = strings.TrimSpace(def)
	if def == "" {
		return nil
	}

	fields := strings.Fields(header)
	if len(fields) < 5 {
		return nil
	}
	posCode := fields[2]
	pos, ok := posMap[posCode]
	if !ok {
		return nil
	}
	wCntHex := fields[3]
	wCnt := 0
	if _, err := fmt.Sscanf(wCntHex, "%x", &wCnt); err != nil || wCnt < 1 {
		return nil
	}
	// words start at fields[4], each followed by a lex_id; need 2*wCnt tokens.
	if len(fields) < 4+2*wCnt {
		return nil
	}
	var out []entry
	for i := 0; i < wCnt; i++ {
		lemma := fields[4+i*2]
		if lemma == "" || !isCleanLemma(lemma) {
			continue
		}
		out = append(out, entry{name: lemma, pos: pos, definition: def})
	}
	return out
}

func isCleanLemma(s string) bool {
	if len(s) < 2 || len(s) > 30 {
		return false
	}
	for _, r := range s {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r == '-' || r == '\'') {
			return false
		}
	}
	return true
}

func gzipFile(in, out string) error {
	src, err := os.Open(in)
	if err != nil {
		return err
	}
	defer src.Close()
	dst, err := os.Create(out)
	if err != nil {
		return err
	}
	defer dst.Close()
	gz := gzip.NewWriter(dst)
	defer gz.Close()
	_, err = io.Copy(gz, src)
	return err
}
