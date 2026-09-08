package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/transform"
)

func useTestDB(t *testing.T) *sql.DB {
	t.Helper()

	previous := db
	previousCache := postalCache
	testDB, err := openPostalDB(filepath.Join(t.TempDir(), "postal.db"))
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	db = testDB
	postalCache = nil
	t.Cleanup(func() {
		testDB.Close()
		db = previous
		postalCache = previousCache
	})
	return testDB
}

func insertTestEntries(t *testing.T) {
	t.Helper()
	err := bulkInsert([]PostalEntry{
		{
			PostalCode:     "1130021",
			PrefectureKana: "ﾄｳｷｮｳﾄ",
			CityKana:       "ﾌﾞﾝｷｮｳｸ",
			TownKana:       "ﾎﾝｺﾏｺﾞﾒ",
			Prefecture:     "東京都",
			City:           "文京区",
			Town:           "本駒込",
		},
	})
	if err != nil {
		t.Fatalf("insert test entries: %v", err)
	}
	if err := loadPostalCache(); err != nil {
		t.Fatalf("load test cache: %v", err)
	}
}

func TestNormalizePostalCode(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
		ok   bool
	}{
		{name: "plain", in: "1130021", want: "1130021", ok: true},
		{name: "hyphenated", in: "113-0021", want: "1130021", ok: true},
		{name: "surrounding spaces", in: " 113-0021 ", want: "1130021", ok: true},
		{name: "letters", in: "abcdefg", ok: false},
		{name: "misplaced hyphen", in: "11-30021", ok: false},
		{name: "multiple hyphens", in: "1-13-0021", ok: false},
		{name: "too short", in: "113002", ok: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := normalizePostalCode(tt.in)
			if got != tt.want || ok != tt.ok {
				t.Fatalf("normalizePostalCode(%q) = (%q, %v), want (%q, %v)", tt.in, got, ok, tt.want, tt.ok)
			}
		})
	}
}

func TestPostalHandler(t *testing.T) {
	useTestDB(t)
	insertTestEntries(t)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /postal/{code}", postalHandler)

	tests := []struct {
		name       string
		path       string
		wantStatus int
		wantError  string
	}{
		{name: "plain code", path: "/postal/1130021", wantStatus: http.StatusOK},
		{name: "hyphenated code", path: "/postal/113-0021", wantStatus: http.StatusOK},
		{name: "unknown code", path: "/postal/0000000", wantStatus: http.StatusNotFound, wantError: "postal code not found"},
		{name: "letters", path: "/postal/abcdefg", wantStatus: http.StatusBadRequest, wantError: "postal code must be 7 digits"},
		{name: "bad hyphen", path: "/postal/11-30021", wantStatus: http.StatusBadRequest, wantError: "postal code must be 7 digits"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, tt.path, nil)
			mux.ServeHTTP(recorder, request)

			if recorder.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d; body: %s", recorder.Code, tt.wantStatus, recorder.Body.String())
			}

			var response struct {
				Success bool            `json:"success"`
				Data    json.RawMessage `json:"data"`
				Error   string          `json:"error"`
			}
			if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if response.Error != tt.wantError {
				t.Fatalf("error = %q, want %q", response.Error, tt.wantError)
			}
			if tt.wantStatus == http.StatusOK && !response.Success {
				t.Fatal("successful lookup returned success=false")
			}
		})
	}
}

func TestHealthHandlerReportsDatabaseFailure(t *testing.T) {
	testDB := useTestDB(t)
	insertTestEntries(t)

	recorder := httptest.NewRecorder()
	healthHandler(recorder, httptest.NewRequest(http.MethodGet, "/health", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("healthy status = %d, want %d", recorder.Code, http.StatusOK)
	}

	if err := testDB.Close(); err != nil {
		t.Fatalf("close test database: %v", err)
	}
	recorder = httptest.NewRecorder()
	healthHandler(recorder, httptest.NewRequest(http.MethodGet, "/health", nil))
	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("unhealthy status = %d, want %d; body: %s", recorder.Code, http.StatusServiceUnavailable, recorder.Body.String())
	}
}

func TestHealthHandlerRejectsEmptyDatabase(t *testing.T) {
	useTestDB(t)

	recorder := httptest.NewRecorder()
	healthHandler(recorder, httptest.NewRequest(http.MethodGet, "/health", nil))
	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("empty database status = %d, want %d; body: %s", recorder.Code, http.StatusServiceUnavailable, recorder.Body.String())
	}
}

func TestCORSMiddleware(t *testing.T) {
	t.Setenv("CORS_ORIGIN", "https://example.test")
	handler := corsMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodOptions, "/postal/1130021", nil)
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNoContent)
	}
	if got := recorder.Header().Get("Access-Control-Allow-Origin"); got != "https://example.test" {
		t.Fatalf("Access-Control-Allow-Origin = %q", got)
	}
}

func TestParseAndInsertCSV(t *testing.T) {
	useTestDB(t)

	source := strings.Join([]string{
		`"13101","100","1000001","ﾄｳｷｮｳﾄ","ﾁﾖﾀﾞｸ","ﾁｵﾀﾞ","東京都","千代田区","千代田"`,
		`"13101","100","1000000","ﾄｳｷｮｳﾄ","ﾁﾖﾀﾞｸ","ｲｶﾆｹｲｻｲｶﾞﾅｲﾊﾞｱｲ","東京都","千代田区","以下に掲載がない場合"`,
	}, "\r\n")
	encoded, _, err := transform.String(japanese.ShiftJIS.NewEncoder(), source)
	if err != nil {
		t.Fatalf("encode fixture: %v", err)
	}

	if err := parseAndInsertCSV(strings.NewReader(encoded)); err != nil {
		t.Fatalf("parseAndInsertCSV: %v", err)
	}
	if err := loadPostalCache(); err != nil {
		t.Fatalf("load parsed entries into cache: %v", err)
	}

	count, err := countEntries()
	if err != nil {
		t.Fatalf("count entries: %v", err)
	}
	if count != 2 {
		t.Fatalf("count = %d, want 2", count)
	}

	entries, err := queryByCode("1000000")
	if err != nil {
		t.Fatalf("query placeholder entry: %v", err)
	}
	if len(entries) != 1 || entries[0].Town != "" {
		t.Fatalf("placeholder town was not normalized: %#v", entries)
	}
}

func TestPostalCacheServesAfterDatabaseClose(t *testing.T) {
	testDB := useTestDB(t)
	insertTestEntries(t)
	if err := testDB.Close(); err != nil {
		t.Fatalf("close test database: %v", err)
	}

	entries, err := queryByCode("1130021")
	if err != nil {
		t.Fatalf("query memory cache: %v", err)
	}
	if len(entries) != 1 || entries[0].Town != "本駒込" {
		t.Fatalf("unexpected cached entries: %#v", entries)
	}
}

func TestParseAndInsertCSVRejectsMalformedInput(t *testing.T) {
	useTestDB(t)
	if err := parseAndInsertCSV(strings.NewReader("too,few,fields\n")); err == nil {
		t.Fatal("parseAndInsertCSV accepted malformed input")
	}
}

func TestDatabasePragmas(t *testing.T) {
	testDB := useTestDB(t)

	var journalMode string
	if err := testDB.QueryRow("PRAGMA journal_mode").Scan(&journalMode); err != nil {
		t.Fatalf("read journal_mode: %v", err)
	}
	if strings.ToLower(journalMode) != "wal" {
		t.Fatalf("journal_mode = %q, want wal", journalMode)
	}

	var busyTimeout int
	if err := testDB.QueryRow("PRAGMA busy_timeout").Scan(&busyTimeout); err != nil {
		t.Fatalf("read busy_timeout: %v", err)
	}
	if busyTimeout != 5000 {
		t.Fatalf("busy_timeout = %d, want 5000", busyTimeout)
	}
}

func TestReadAtMost(t *testing.T) {
	got, err := readAtMost(strings.NewReader("12345"), 5)
	if err != nil || !bytes.Equal(got, []byte("12345")) {
		t.Fatalf("readAtMost at limit = (%q, %v)", got, err)
	}
	if _, err := readAtMost(strings.NewReader("123456"), 5); err == nil {
		t.Fatal("readAtMost accepted oversized input")
	}
}

func TestNormTown(t *testing.T) {
	tests := map[string]string{
		"本駒込（１丁目）":    "本駒込",
		"以下に掲載がない場合":  "",
		"市の次に番地がくる場合": "",
		"  千代田  ":     "千代田",
	}
	for input, want := range tests {
		if got := normTown(input); got != want {
			t.Errorf("normTown(%q) = %q, want %q", input, got, want)
		}
	}
}

func BenchmarkPostalCacheLookup(b *testing.B) {
	previous := postalCache
	postalCache = &postalMemoryCache{
		byCode: map[string][]PostalEntry{
			"1130021": {{PostalCode: "1130021", Prefecture: "東京都", City: "文京区", Town: "本駒込"}},
		},
		recordCount: 1,
	}
	b.Cleanup(func() { postalCache = previous })
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			entries, err := queryByCode("1130021")
			if err != nil || len(entries) != 1 {
				b.Fatalf("cache lookup failed: entries=%d err=%v", len(entries), err)
			}
		}
	})
}
