# beon-postal

**Japanese Postal Code API** — fast, lightweight REST API built with Go and SQLite.

Data is sourced from the official [Japan Post (郵便局)](https://www.post.japanpost.jp/zipcode/download.html) database (~124,000 postal codes), stored in a local SQLite database for instant lookups.

Powered by **BEON API**

---

## Features

- Zero external dependencies at runtime (pure Go + embedded SQLite)
- Auto-downloads and seeds data from Japan Post on first run
- SQLite with WAL mode for fast concurrent reads
- Instant startup on subsequent runs (reads from cached `data/postal.db`)
- Clean JSON response envelope with `success`, `data`, and `meta`

---

## Requirements

- Go 1.22+

---

## Getting Started

```bash
git clone git@github.com:afuzapratama/beon-postal.git
cd beon-postal
go mod tidy
go run .
```

On first run, the server will automatically download `KEN_ALL.ZIP` from Japan Post, parse and import all records into `data/postal.db`, then start listening.

```
2026/05/03 12:00:00 Downloading KEN_ALL.ZIP from Japan Post...
2026/05/03 12:00:04 Cached to data/KEN_ALL.CSV
2026/05/03 12:00:05 Inserted 124453 entries into SQLite
2026/05/03 12:00:05 Postal API ready — 124453 entries in SQLite, listening on :8080
```

### Custom port

```bash
export PORT=9000
go run .
```

### Build binary

```bash
go build -ldflags="-s -w" -o postal-api .
./postal-api
```

### Pre-download data (optional)

```bash
make download   # downloads and extracts KEN_ALL.CSV into data/
make run        # start server (skips download)
```

---

## API Reference

### `GET /postal/{code}`

Lookup address by 7-digit postal code. Accepts codes with or without hyphen.

| Parameter | Example |
|---|---|
| `{code}` | `1130021` or `113-0021` |

**Single result** — `data` is an object:

```
GET /postal/1130021
```

```json
{
  "success": true,
  "data": {
    "postalCode": "1130021",
    "prefectureKana": "ﾄｳｷｮｳﾄ",
    "cityKana": "ﾌﾞﾝｷｮｳｸ",
    "townKana": "ﾎﾝｺﾏｺﾞﾒ",
    "prefecture": "東京都",
    "city": "文京区",
    "town": "本駒込"
  },
  "meta": {
    "powered_by": "BEON API",
    "timestamp": "2026-05-03T12:00:00Z"
  }
}
```

**Multiple results** — `data` is an array (some codes map to more than one city/town):

```
GET /postal/0040000
```

```json
{
  "success": true,
  "data": [
    {
      "postalCode": "0040000",
      "prefectureKana": "ﾎｯｶｲﾄﾞｳ",
      "cityKana": "ｻｯﾎﾟﾛｼｱﾂﾍﾞﾂｸ",
      "townKana": "ｲｶﾆｹｲｻｲｶﾞﾅｲﾊﾞｱｲ",
      "prefecture": "北海道",
      "city": "札幌市厚別区",
      "town": ""
    },
    {
      "postalCode": "0040000",
      "prefectureKana": "ﾎｯｶｲﾄﾞｳ",
      "cityKana": "ｻｯﾎﾟﾛｼｷﾖﾀｸ",
      "townKana": "ｲｶﾆｹｲｻｲｶﾞﾅｲﾊﾞｱｲ",
      "prefecture": "北海道",
      "city": "札幌市清田区",
      "town": ""
    }
  ],
  "meta": {
    "powered_by": "BEON API",
    "timestamp": "2026-05-03T12:00:00Z"
  }
}
```

**Error responses:**

```json
{ "success": false, "error": "postal code not found", "meta": { ... } }     // 404
{ "success": false, "error": "postal code must be 7 digits", "meta": { ... } } // 400
```

---

### `GET /health`

```json
{
  "success": true,
  "data": {
    "records": 124453,
    "status": "ok"
  },
  "meta": {
    "powered_by": "BEON API",
    "timestamp": "2026-05-03T12:00:00Z"
  }
}
```

---

## Response Fields

| Field | Description |
|---|---|
| `postalCode` | 7-digit postal code (no hyphen) |
| `prefecture` | Prefecture name in kanji (e.g. `東京都`) |
| `city` | City/ward name in kanji (e.g. `文京区`) |
| `town` | Town/district name in kanji (e.g. `本駒込`) |
| `prefectureKana` | Prefecture name in half-width katakana |
| `cityKana` | City name in half-width katakana |
| `townKana` | Town name in half-width katakana |

---

## Project Structure

```
beon-postal/
├── main.go     — HTTP server, route handlers, response envelope
├── db.go       — SQLite init, schema, query, bulk insert
├── loader.go   — CSV download, Shift-JIS decoding, data seeding
├── go.mod
├── go.sum
├── Makefile
└── data/
    ├── KEN_ALL.CSV   (auto-downloaded, git-ignored)
    └── postal.db     (SQLite database, git-ignored)
```

---

## Data Source

- General page: https://www.post.japanpost.jp/zipcode/download.html
- Download URL: https://www.post.japanpost.jp/zipcode/dl/kogaki/zip/ken_all.zip
- Encoding: Windows-31J (Shift-JIS) — decoded to UTF-8 automatically
- Latest update: check Japan Post website for data freshness

To refresh data, delete `data/postal.db` and restart the server.

---

## License

MIT
