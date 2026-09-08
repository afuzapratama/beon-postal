package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

const dbPath = "data/postal.db"

var db *sql.DB

const postalSchema = `
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
`

// initDB opens (or creates) the SQLite database and ensures the schema exists.
func initDB() error {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0750); err != nil {
		return fmt.Errorf("mkdir data: %w", err)
	}

	var err error
	db, err = openPostalDB(dbPath)
	return err
}

func openPostalDB(path string) (*sql.DB, error) {
	// These modernc.org/sqlite pragmas are applied to every new connection.
	dsn := fmt.Sprintf("file:%s?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)", filepath.ToSlash(path))
	conn, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	if err := conn.Ping(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("ping db: %w", err)
	}

	if _, err := conn.Exec(postalSchema); err != nil {
		conn.Close()
		return nil, fmt.Errorf("create schema: %w", err)
	}

	return conn, nil
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
	if len(entries) == 0 {
		return fmt.Errorf("no postal entries to insert")
	}

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

	inserted := int64(0)
	for _, e := range entries {
		result, err := stmt.Exec(
			e.PostalCode, e.PrefectureKana, e.CityKana, e.TownKana,
			e.Prefecture, e.City, e.Town,
		)
		if err != nil {
			return fmt.Errorf("insert %s: %w", e.PostalCode, err)
		}
		if n, err := result.RowsAffected(); err == nil {
			inserted += n
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	log.Printf("Inserted %d unique entries into SQLite", inserted)
	return nil
}
