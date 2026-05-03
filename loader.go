package main

import (
	"archive/zip"
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"

	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/transform"
)

const (
	kenAllURL  = "https://www.post.japanpost.jp/zipcode/dl/kogaki/zip/ken_all.zip"
	kenAllPath = "data/KEN_ALL.CSV"
)

// PostalEntry holds address data for a single postal code entry.
type PostalEntry struct {
	PostalCode     string `json:"postalCode"`
	PrefectureKana string `json:"prefectureKana"`
	CityKana       string `json:"cityKana"`
	TownKana       string `json:"townKana"`
	Prefecture     string `json:"prefecture"`
	City           string `json:"city"`
	Town           string `json:"town"`
}

// loadPostalDB initialises SQLite and seeds it from CSV if the table is empty.
func loadPostalDB() error {
	if err := initDB(); err != nil {
		return err
	}

	populated, err := isDBPopulated()
	if err != nil {
		return err
	}
	if populated {
		return nil
	}

	// Try locally cached CSV before hitting the network.
	f, err := os.Open(kenAllPath)
	if err == nil {
		defer f.Close()
		log.Printf("Importing cached CSV into SQLite...")
		return parseAndInsertCSV(f)
	}

	log.Println("Downloading KEN_ALL.ZIP from Japan Post...")
	return downloadAndParse()
}

func downloadAndParse() error {
	resp, err := http.Get(kenAllURL)
	if err != nil {
		return fmt.Errorf("download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected HTTP status: %d", resp.StatusCode)
	}

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read body: %w", err)
	}

	zr, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		return fmt.Errorf("unzip: %w", err)
	}

	for _, zf := range zr.File {
		if !strings.HasSuffix(strings.ToLower(zf.Name), ".csv") {
			continue
		}

		rc, err := zf.Open()
		if err != nil {
			return fmt.Errorf("open zip entry %q: %w", zf.Name, err)
		}
		csvBytes, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return fmt.Errorf("read csv bytes: %w", err)
		}

		if mkErr := os.MkdirAll("data", 0750); mkErr == nil {
			if writeErr := os.WriteFile(kenAllPath, csvBytes, 0640); writeErr == nil {
				log.Printf("Cached to %s", kenAllPath)
			}
		}

		return parseAndInsertCSV(bytes.NewReader(csvBytes))
	}

	return fmt.Errorf("no CSV file found in archive")
}

// ---------- CSV parsing ----------

var parenRe = regexp.MustCompile(`（[^）]*）`)

// normTown cleans the raw town cell: strips parenthetical info and
// replaces generic "no address" placeholders with an empty string.
func normTown(s string) string {
	s = parenRe.ReplaceAllString(s, "")
	s = strings.TrimSpace(s)
	if strings.Contains(s, "以下に掲載がない場合") ||
		strings.Contains(s, "の次に番地がくる場合") {
		return ""
	}
	return s
}

// parseAndInsertCSV parses a Shift-JIS encoded KEN_ALL.CSV reader and
// bulk-inserts all entries into SQLite. Deduplication is handled by the
// UNIQUE(postal_code, city, town) constraint via INSERT OR IGNORE.
func parseAndInsertCSV(r io.Reader) error {
	utf8r := transform.NewReader(r, japanese.ShiftJIS.NewDecoder())
	cr := csv.NewReader(utf8r)
	cr.LazyQuotes = true
	cr.FieldsPerRecord = -1

	var entries []PostalEntry

	type multiRow struct {
		code  string
		entry PostalEntry
	}
	var pending *multiRow

	for {
		rec, err := cr.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}
		if len(rec) < 9 {
			continue
		}

		code := strings.TrimSpace(rec[2])
		if len(code) != 7 {
			continue
		}

		prefKana := strings.TrimSpace(rec[3])
		cityKana := strings.TrimSpace(rec[4])
		townKana := strings.TrimSpace(rec[5])
		pref := strings.TrimSpace(rec[6])
		city := strings.TrimSpace(rec[7])
		town := strings.TrimSpace(rec[8])

		// multi-row town name continuation (town spans multiple CSV rows)
		if pending != nil {
			if pending.code == code {
				pending.entry.Town += town
				pending.entry.TownKana += townKana
				if strings.Contains(town, "\u300d") {
					pending.entry.Town = normTown(pending.entry.Town)
					entries = append(entries, pending.entry)
					pending = nil
				}
				continue
			}
			pending.entry.Town = normTown(pending.entry.Town)
			entries = append(entries, pending.entry)
			pending = nil
		}

		if strings.Contains(town, "\u300c") && !strings.Contains(town, "\u300d") {
			pending = &multiRow{
				code: code,
				entry: PostalEntry{
					PostalCode:     code,
					PrefectureKana: prefKana,
					CityKana:       cityKana,
					TownKana:       townKana,
					Prefecture:     pref,
					City:           city,
					Town:           town,
				},
			}
			continue
		}

		entries = append(entries, PostalEntry{
			PostalCode:     code,
			PrefectureKana: prefKana,
			CityKana:       cityKana,
			TownKana:       townKana,
			Prefecture:     pref,
			City:           city,
			Town:           normTown(town),
		})
	}

	if pending != nil {
		pending.entry.Town = normTown(pending.entry.Town)
		entries = append(entries, pending.entry)
	}

	return bulkInsert(entries)
}
