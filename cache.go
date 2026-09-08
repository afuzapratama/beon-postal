package main

import (
	"fmt"
	"log"
	"time"
)

type postalMemoryCache struct {
	byCode      map[string][]PostalEntry
	recordCount int
}

// postalCache is built once during startup and is read-only after the HTTP
// server starts, so lookups do not need locks.
var postalCache *postalMemoryCache

func loadPostalCache() error {
	started := time.Now()
	count, err := countEntries()
	if err != nil {
		return fmt.Errorf("load cache count: %w", err)
	}

	rows, err := db.Query(`
		SELECT postal_code, prefecture_kana, city_kana, town_kana, prefecture, city, town
		FROM postal
		ORDER BY prefecture, city, town
	`)
	if err != nil {
		return fmt.Errorf("load cache query: %w", err)
	}
	defer rows.Close()

	byCode := make(map[string][]PostalEntry, count)
	interned := make(map[string]string)
	intern := func(value string) string {
		if existing, ok := interned[value]; ok {
			return existing
		}
		interned[value] = value
		return value
	}

	recordCount := 0
	for rows.Next() {
		var entry PostalEntry
		if err := rows.Scan(
			&entry.PostalCode, &entry.PrefectureKana, &entry.CityKana, &entry.TownKana,
			&entry.Prefecture, &entry.City, &entry.Town,
		); err != nil {
			return fmt.Errorf("load cache scan: %w", err)
		}

		entry.PostalCode = intern(entry.PostalCode)
		entry.PrefectureKana = intern(entry.PrefectureKana)
		entry.CityKana = intern(entry.CityKana)
		entry.TownKana = intern(entry.TownKana)
		entry.Prefecture = intern(entry.Prefecture)
		entry.City = intern(entry.City)
		entry.Town = intern(entry.Town)
		byCode[entry.PostalCode] = append(byCode[entry.PostalCode], entry)
		recordCount++
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("load cache rows: %w", err)
	}
	if recordCount == 0 {
		return fmt.Errorf("postal database is empty")
	}

	postalCache = &postalMemoryCache{
		byCode:      byCode,
		recordCount: recordCount,
	}
	log.Printf(
		"Loaded %d entries for %d postal codes into memory in %s",
		recordCount,
		len(byCode),
		time.Since(started).Round(time.Millisecond),
	)
	return nil
}

// queryByCode serves lookups from the immutable in-memory cache.
func queryByCode(code string) ([]PostalEntry, error) {
	if postalCache == nil {
		return nil, fmt.Errorf("postal cache is not loaded")
	}
	return postalCache.byCode[code], nil
}

func cachedEntryCount() (int, bool) {
	if postalCache == nil {
		return 0, false
	}
	return postalCache.recordCount, true
}
