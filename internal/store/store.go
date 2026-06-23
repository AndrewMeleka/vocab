package store

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/andrewnageh/vocab/internal/config"
	"github.com/andrewnageh/vocab/internal/db"
)

type ReviewResult string

const (
	ResultKnew   ReviewResult = "knew"
	ResultForgot ReviewResult = "forgot"
)

type ReviewEvent struct {
	At     time.Time    `json:"at"`
	Result ReviewResult `json:"result"`
	Box    int          `json:"box"`
}

// Word is a dictionary entry independent of any user's study state.
type Word struct {
	ID         int64
	Name       string
	Definition string
	Type       string
	Level      sql.NullString
	Language   string
	Source     string
}

// Card is one row in the user's study set, flat for caller convenience.
// Word/Definition/Examples mirror the joined words row + examples table.
type Card struct {
	ID             int64
	WordID         int64
	Word           string
	Definition     string
	Examples       []string
	Box            int
	AddedAt        time.Time
	LastReviewedAt *time.Time
	NextDueAt      time.Time
	CorrectStreak  int
	WrongCount     int
}

// Forgotten reports whether this card was missed after reaching box ≥ 2.
func (c Card) Forgotten(s *Store) bool {
	if s == nil || c.ID == 0 {
		return false
	}
	var x int
	err := s.conn.QueryRow(
		`SELECT 1 FROM reviews WHERE card_id = ? AND result = 'forgot' AND box >= 2 LIMIT 1`,
		c.ID,
	).Scan(&x)
	return err == nil
}

type Store struct {
	conn *sql.DB
}

func Load() (*Store, error) {
	path, err := config.DBPath()
	if err != nil {
		return nil, err
	}
	conn, err := db.Open(path)
	if err != nil {
		return nil, err
	}
	return &Store{conn: conn}, nil
}

// Save is a no-op — all mutators commit inline. Kept for caller compatibility.
func (s *Store) Save() error { return nil }

func (s *Store) Close() error { return s.conn.Close() }

// FindWord looks up a word by name (case-insensitive). Returns nil if absent.
func (s *Store) FindWord(name string) (*Word, error) {
	row := s.conn.QueryRow(
		`SELECT id, name, definition, COALESCE(type,''), level, language, source FROM words WHERE name = ?`,
		strings.ToLower(strings.TrimSpace(name)),
	)
	var w Word
	if err := row.Scan(&w.ID, &w.Name, &w.Definition, &w.Type, &w.Level, &w.Language, &w.Source); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &w, nil
}

// InsertWord adds a new dictionary entry and returns its id.
func (s *Store) InsertWord(name, definition, wordType string) (int64, error) {
	res, err := s.conn.Exec(
		`INSERT INTO words(name, definition, type, language, source) VALUES(?,?,?,?,?)`,
		strings.ToLower(strings.TrimSpace(name)), definition, nullableType(wordType), "en", "ai",
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func nullableType(t string) any {
	t = strings.ToLower(strings.TrimSpace(t))
	if t == "" {
		return nil
	}
	return t
}

// Examples returns the stored example sentences for a word, in insertion order.
func (s *Store) Examples(wordID int64) ([]string, error) {
	rows, err := s.conn.Query(`SELECT example FROM examples WHERE word_id = ? ORDER BY id`, wordID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var ex string
		if err := rows.Scan(&ex); err != nil {
			return nil, err
		}
		out = append(out, ex)
	}
	return out, rows.Err()
}

// AddExamples appends example sentences to a word.
func (s *Store) AddExamples(wordID int64, examples []string) error {
	if wordID == 0 || len(examples) == 0 {
		return nil
	}
	tx, err := s.conn.Begin()
	if err != nil {
		return err
	}
	stmt, err := tx.Prepare(`INSERT INTO examples(word_id, example) VALUES(?,?)`)
	if err != nil {
		_ = tx.Rollback()
		return err
	}
	defer stmt.Close()
	for _, ex := range examples {
		ex = strings.TrimSpace(ex)
		if ex == "" {
			continue
		}
		if _, err := stmt.Exec(wordID, ex); err != nil {
			_ = tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

// CreateCard inserts a new card row pointing at wordID, due immediately.
// Errors with a clean message if the word already has a card (UNIQUE constraint).
func (s *Store) CreateCard(wordID int64, now time.Time) (int64, error) {
	res, err := s.conn.Exec(
		`INSERT INTO collections(word_id, box, added_at, next_due_at, correct_streak, wrong_count)
		 VALUES(?, 0, ?, ?, 0, 0)`,
		wordID, now, now,
	)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return 0, fmt.Errorf("this word is already in your collection")
		}
		return 0, err
	}
	return res.LastInsertId()
}

// Find returns the card for a given word (case-insensitive) plus a placeholder
// index for API compat with the legacy JSON store; callers discard the int.
func (s *Store) Find(word string) (*Card, int) {
	c, err := s.findCardByWord(strings.ToLower(strings.TrimSpace(word)))
	if err != nil || c == nil {
		return nil, -1
	}
	return c, 0
}

func (s *Store) findCardByWord(name string) (*Card, error) {
	row := s.conn.QueryRow(`
		SELECT c.id, c.word_id, w.name, w.definition, c.box, c.added_at,
		       c.last_reviewed_at, c.next_due_at, c.correct_streak, c.wrong_count
		FROM collections c JOIN words w ON w.id = c.word_id
		WHERE w.name = ?`, name)
	c, err := scanCard(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if err := s.loadExamples(c); err != nil {
		return nil, err
	}
	return c, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanCard(r rowScanner) (*Card, error) {
	var c Card
	var lastReviewed sql.NullTime
	if err := r.Scan(&c.ID, &c.WordID, &c.Word, &c.Definition, &c.Box, &c.AddedAt,
		&lastReviewed, &c.NextDueAt, &c.CorrectStreak, &c.WrongCount); err != nil {
		return nil, err
	}
	if lastReviewed.Valid {
		t := lastReviewed.Time
		c.LastReviewedAt = &t
	}
	return &c, nil
}

func (s *Store) loadExamples(c *Card) error {
	rows, err := s.conn.Query(`SELECT example FROM examples WHERE word_id = ? ORDER BY id`, c.WordID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var ex string
		if err := rows.Scan(&ex); err != nil {
			return err
		}
		c.Examples = append(c.Examples, ex)
	}
	return rows.Err()
}

// Upsert writes review-state mutations back to the cards table (and appends a
// ReviewEvent to reviews if provided). Inserts are handled by CreateCard.
func (s *Store) Upsert(c Card) error {
	if c.ID == 0 {
		return fmt.Errorf("Upsert: card.ID is zero — use CreateCard for new cards")
	}
	_, err := s.conn.Exec(
		`UPDATE collections SET box = ?, last_reviewed_at = ?, next_due_at = ?,
		                  correct_streak = ?, wrong_count = ? WHERE id = ?`,
		c.Box, c.LastReviewedAt, c.NextDueAt, c.CorrectStreak, c.WrongCount, c.ID,
	)
	return err
}

// RecordReview appends a row to the reviews table — call after a knew/forgot.
func (s *Store) RecordReview(cardID int64, ev ReviewEvent) error {
	_, err := s.conn.Exec(
		`INSERT INTO reviews(card_id, at, result, box) VALUES(?,?,?,?)`,
		cardID, ev.At, string(ev.Result), ev.Box,
	)
	return err
}

// Due returns cards whose next_due_at is on or before now, oldest first.
// Examples are pre-loaded.
func (s *Store) Due(now time.Time) []Card {
	rows, err := s.conn.Query(`
		SELECT c.id, c.word_id, w.name, w.definition, c.box, c.added_at,
		       c.last_reviewed_at, c.next_due_at, c.correct_streak, c.wrong_count
		FROM collections c JOIN words w ON w.id = c.word_id
		WHERE c.next_due_at <= ? ORDER BY c.next_due_at`, now)
	if err != nil {
		return nil
	}
	return s.collectCards(rows, true)
}

// Recent returns the n most recently added cards. Examples NOT pre-loaded.
func (s *Store) Recent(n int) []Card {
	rows, err := s.conn.Query(`
		SELECT c.id, c.word_id, w.name, w.definition, c.box, c.added_at,
		       c.last_reviewed_at, c.next_due_at, c.correct_streak, c.wrong_count
		FROM collections c JOIN words w ON w.id = c.word_id
		ORDER BY c.added_at DESC LIMIT ?`, n)
	if err != nil {
		return nil
	}
	return s.collectCards(rows, false)
}

// All returns every card, alphabetically. Examples NOT pre-loaded.
func (s *Store) All() []Card {
	rows, err := s.conn.Query(`
		SELECT c.id, c.word_id, w.name, w.definition, c.box, c.added_at,
		       c.last_reviewed_at, c.next_due_at, c.correct_streak, c.wrong_count
		FROM collections c JOIN words w ON w.id = c.word_id
		ORDER BY w.name`)
	if err != nil {
		return nil
	}
	return s.collectCards(rows, false)
}

// CountCards is a tiny helper for the config command.
func (s *Store) CountCards() int {
	var n int
	_ = s.conn.QueryRow(`SELECT COUNT(*) FROM collections`).Scan(&n)
	return n
}

// CountWords reports the total entries in the dictionary.
func (s *Store) CountWords() int {
	var n int
	_ = s.conn.QueryRow(`SELECT COUNT(*) FROM words`).Scan(&n)
	return n
}

func (s *Store) collectCards(rows *sql.Rows, withExamples bool) []Card {
	defer rows.Close()
	var cards []Card
	for rows.Next() {
		c, err := scanCard(rows)
		if err != nil {
			return cards
		}
		cards = append(cards, *c)
	}
	if withExamples {
		for i := range cards {
			_ = s.loadExamples(&cards[i])
		}
	}
	return cards
}

// SampleNewWords picks n random words that have no card yet, optionally filtered
// by CEFR level. If levels is non-empty but no rows match, falls back to "any".
func (s *Store) SampleNewWords(n int, levels []string) ([]Word, bool, error) {
	query, args := buildSampleQuery(n, levels)
	rows, err := s.conn.Query(query, args...)
	if err != nil {
		return nil, false, err
	}
	out, err := scanWords(rows)
	if err != nil {
		return nil, false, err
	}
	if len(out) > 0 || len(levels) == 0 {
		return out, false, nil
	}
	// Level filter excluded everything; retry without it.
	rows2, err := s.conn.Query(
		`SELECT w.id, w.name, w.definition, COALESCE(w.type,''), w.level, w.language, w.source
		 FROM words w LEFT JOIN collections c ON c.word_id = w.id
		 WHERE c.id IS NULL ORDER BY RANDOM() LIMIT ?`, n)
	if err != nil {
		return nil, true, err
	}
	out, err = scanWords(rows2)
	return out, true, err
}

func buildSampleQuery(n int, levels []string) (string, []any) {
	base := `SELECT w.id, w.name, w.definition, COALESCE(w.type,''), w.level, w.language, w.source
	         FROM words w LEFT JOIN collections c ON c.word_id = w.id
	         WHERE c.id IS NULL`
	args := []any{}
	if len(levels) > 0 {
		placeholders := strings.Repeat("?,", len(levels))
		placeholders = placeholders[:len(placeholders)-1]
		base += " AND w.level IN (" + placeholders + ")"
		for _, l := range levels {
			args = append(args, strings.ToLower(l))
		}
	}
	base += " ORDER BY RANDOM() LIMIT ?"
	args = append(args, n)
	return base, args
}

func scanWords(rows *sql.Rows) ([]Word, error) {
	defer rows.Close()
	var out []Word
	for rows.Next() {
		var w Word
		if err := rows.Scan(&w.ID, &w.Name, &w.Definition, &w.Type, &w.Level, &w.Language, &w.Source); err != nil {
			return out, err
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

// DeleteCardByWord removes the card for the given word from the collection.
// Reviews cascade away via FK. The word entry in the dictionary is untouched.
// Returns true if a card was deleted, false if no such card existed.
func (s *Store) DeleteCardByWord(name string) (bool, error) {
	res, err := s.conn.Exec(`
		DELETE FROM collections WHERE word_id = (SELECT id FROM words WHERE name = ?)`,
		strings.ToLower(strings.TrimSpace(name)),
	)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// ResetCards deletes every card from the collection. Reviews cascade. The dictionary
// (words + examples) is untouched. Returns the number of cards removed.
func (s *Store) ResetCards() (int, error) {
	res, err := s.conn.Exec(`DELETE FROM collections`)
	if err != nil {
		return 0, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, err
	}
	return int(n), nil
}

// RandomDueCard picks one random card whose next_due_at <= now; nil if none.
func (s *Store) RandomDueCard(now time.Time) *Card {
	row := s.conn.QueryRow(`
		SELECT c.id, c.word_id, w.name, w.definition, c.box, c.added_at,
		       c.last_reviewed_at, c.next_due_at, c.correct_streak, c.wrong_count
		FROM collections c JOIN words w ON w.id = c.word_id
		WHERE c.next_due_at <= ? ORDER BY RANDOM() LIMIT 1`, now)
	c, err := scanCard(row)
	if err != nil {
		return nil
	}
	_ = s.loadExamples(c)
	return c
}
