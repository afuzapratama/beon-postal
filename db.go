package main

import (
"database/sql"
"fmt"
"log"
"os"

_ "modernc.org/sqlite"
)

const dbPath = "data/postal.db"

var db *sql.DB

// initDB opens (or creates) the SQLite database and ensures the schema exists.
func initDB() error {
if err := os.MkdirAll("data", 0750); err != nil {
return fmt.Errorf("mkdir data: %w", err)
}

var err error
// WAL mode for better concurrent read performance.
db, err = sql.Open("sqlite", dbPath+"?_journal=WAL&_timeout=5000")
if err != nil {
return fmt.Errorf("open db: %w", err)
}

_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS postal (
postal_code      TEXT NOT NULL,
prefecture_kana  TEXT NOT NULL,
city_kana        TEXT NOT NULL,
town_kana        TEXT NOT NULL,
prefecture       TEXT NOT NULL,
city             TEXT NOT NULL,
town             TEXT NOT NULL,
UNIQUE(postal_code, city, town)
);
		CREATE INDEX IF NOT EXISTS idx_postal_code ON postal(postal_code);
	`)
if err != nil {
return fmt.Errorf("create schema: %w", err)
}
return nil
}

// isDBPopulated returns true when the postal table already has rows.
func isDBPopulated() (bool, error) {
var n int
err := db.QueryRow("SELECT COUNT(*) FROM postal").Scan(&n)
return n > 0, err
}

// countEntries returns total rows in the postal table.
func countEntries() (int, error) {
var n int
err := db.QueryRow("SELECT COUNT(*) FROM postal").Scan(&n)
return n, err
}

// bulkInsert inserts all entries inside a single transaction for speed.
// Duplicates (same postal_code + city + town) are silently ignored.
func bulkInsert(entries []PostalEntry) error {
tx, err := db.Begin()
if err != nil {
return fmt.Errorf("begin tx: %w", err)
}
defer tx.Rollback()

stmt, err := tx.Prepare(`
		INSERT OR IGNORE INTO postal
			(postal_code, prefecture_kana, city_kana, town_kana, prefecture, city, town)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`)
if err != nil {
return fmt.Errorf("prepare stmt: %w", err)
}
defer stmt.Close()

for _, e := range entries {
if _, err := stmt.Exec(
e.PostalCode, e.PrefectureKana, e.CityKana, e.TownKana,
e.Prefecture, e.City, e.Town,
); err != nil {
return fmt.Errorf("insert %s: %w", e.PostalCode, err)
}
}

if err := tx.Commit(); err != nil {
return fmt.Errorf("commit: %w", err)
}
log.Printf("Inserted %d entries into SQLite", len(entries))
return nil
}

// queryByCode returns all postal entries for the given 7-digit code.
func queryByCode(code string) ([]PostalEntry, error) {
rows, err := db.Query(`
		SELECT postal_code, prefecture_kana, city_kana, town_kana, prefecture, city, town
		FROM postal
		WHERE postal_code = ?
	`, code)
if err != nil {
return nil, fmt.Errorf("query: %w", err)
}
defer rows.Close()

var results []PostalEntry
for rows.Next() {
var e PostalEntry
if err := rows.Scan(
&e.PostalCode, &e.PrefectureKana, &e.CityKana, &e.TownKana,
&e.Prefecture, &e.City, &e.Town,
); err != nil {
return nil, fmt.Errorf("scan: %w", err)
}
results = append(results, e)
}
return results, rows.Err()
}
