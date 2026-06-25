package db

const Schema = `
CREATE TABLE IF NOT EXISTS words (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    name        TEXT    NOT NULL UNIQUE COLLATE NOCASE,
    language    TEXT    NOT NULL DEFAULT 'en',
    definition  TEXT    NOT NULL,
    type        TEXT    CHECK(type IN ('noun','verb','adj','adv','pron','prep','conj','interj','phrase')),
    source      TEXT    NOT NULL DEFAULT 'ai'
);

CREATE TABLE IF NOT EXISTS examples (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    word_id    INTEGER NOT NULL REFERENCES words(id) ON DELETE CASCADE,
    example    TEXT    NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_examples_word_id ON examples(word_id);

CREATE TABLE IF NOT EXISTS collections (
    id               INTEGER PRIMARY KEY AUTOINCREMENT,
    word_id          INTEGER NOT NULL UNIQUE REFERENCES words(id) ON DELETE CASCADE,
    box              INTEGER NOT NULL DEFAULT 0,
    added_at         DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_reviewed_at DATETIME,
    next_due_at      DATETIME NOT NULL,
    correct_streak   INTEGER NOT NULL DEFAULT 0,
    wrong_count      INTEGER NOT NULL DEFAULT 0,
    topic            TEXT
);
CREATE INDEX IF NOT EXISTS idx_collections_next_due ON collections(next_due_at);

CREATE TABLE IF NOT EXISTS reviews (
    id      INTEGER PRIMARY KEY AUTOINCREMENT,
    card_id INTEGER NOT NULL REFERENCES collections(id) ON DELETE CASCADE,
    at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    result  TEXT    NOT NULL CHECK(result IN ('knew','forgot')),
    box     INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_reviews_card_id ON reviews(card_id);
`
