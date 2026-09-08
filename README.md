# beon-postal

**Japanese Postal Code API** — fast, lightweight REST API built with Go and SQLite.

Data is sourced from the official [Japan Post (郵便局)](https://www.post.japanpost.jp/service/search/zipcode/download/) database (~124,000 postal codes), stored in a local SQLite database for instant lookups.

Powered by **BEON API**

---

## Features

- Zero external dependencies at runtime (pure Go + embedded SQLite)
- Auto-downloads and seeds data from Japan Post on first run
- Loads all postal records into an in-memory cache for database-free lookups
- SQLite with WAL mode for fast concurrent reads
- Instant startup on subsequent runs (reads from cached `data/postal.db`)
- Clean JSON response envelope with `success`, `data`, and `meta`

---

## Requirements

- Go 1.25+
- At least 128 MB free memory; 256 MB is recommended for deployment

---

## Deploy di aaPanel

Panduan lengkap untuk menjalankan API ini sebagai service permanen di server dengan **aaPanel**.

### 1. Install Go

SSH ke server, lalu install Go:

```bash
GO_VERSION="$(curl -fsSL 'https://go.dev/VERSION?m=text' | sed -n '1p')"
wget "https://go.dev/dl/${GO_VERSION}.linux-amd64.tar.gz"
tar -C /usr/local -xzf "${GO_VERSION}.linux-amd64.tar.gz"
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
source ~/.bashrc
go version
```

### 2. Clone & Build

```bash
cd /www/wwwroot
git clone https://github.com/afuzapratama/beon-postal.git
cd beon-postal
go mod download
go build -trimpath -ldflags="-s -w" -o postal-api .
install -d -o www -g www -m 750 data
chown www:www postal-api
chmod 750 postal-api
```

Folder `data` harus writable oleh user `www` karena SQLite membuat file database, WAL, dan cache CSV di sana.

### 3. Tambahkan sebagai Go Project

Buka **Website → Go Project → Add Project**, lalu isi:

| Field | Value |
|---|---|
| Executable File | `/www/wwwroot/beon-postal/postal-api` |
| Project Name | `beon-postal` |
| Project Port | `8090` |
| Release port | Jangan dicentang jika memakai domain/reverse proxy |
| Execution Command | `/www/wwwroot/beon-postal/postal-api` |
| Environment Variables | `PORT=8090` |
| Run User | `www` |
| Startup | Aktifkan |
| Remark | `Japanese Postal Code API` |
| Domain name | Domain API, misalnya `postal.example.com` |

Jika API hanya boleh diakses browser dari frontend tertentu, tambahkan environment variable berikut:

```text
CORS_ORIGIN=https://app.example.com
```

Setelah klik **Confirm**, first-run akan otomatis mengunduh data Japan Post, mengisi SQLite, lalu membangun in-memory cache. Periksa **Project logs** sampai muncul:

```text
Loaded 124493 entries for 120717 postal codes into memory
Postal API ready — 124493 entries in SQLite, listening on :8090
```

### 4. Aktifkan Domain dan SSL

Pastikan DNS `A` domain sudah mengarah ke IP server. Di detail Go Project:

- Tambahkan domain melalui **Domain Manager** jika belum diisi saat membuat project.
- Aktifkan **External network mapping** melalui menu **Mapping**.
- Pasang sertifikat Let's Encrypt melalui menu **SSL**.
- Biarkan port `8090` tertutup dari akses publik; trafik masuk melalui Nginx pada port 80/443.

### 5. Test

```bash
curl https://postal.example.com/postal/1130021
curl https://postal.example.com/health
```

### Update Aplikasi (setelah push ke GitHub)

```bash
cd /www/wwwroot/beon-postal
git pull --ff-only origin main
go mod download
go build -trimpath -ldflags="-s -w" -o postal-api.new .
chown www:www postal-api.new
chmod 750 postal-api.new
mv postal-api.new postal-api
```

Setelah build selesai, klik **Restart** pada Go Project dan periksa **Project logs**.

### Update Data (opsional)

Klik **Stop** pada Go Project, lalu jalankan:

```bash
cd /www/wwwroot/beon-postal
rm -f data/postal.db data/postal.db-shm data/postal.db-wal data/KEN_ALL.CSV
```

Klik **Start** kembali. Server akan mengunduh dataset terbaru dan membangun ulang cache.

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
2026/09/08 12:00:00 Downloading KEN_ALL.ZIP from Japan Post...
2026/09/08 12:00:04 Cached to data/KEN_ALL.CSV
2026/09/08 12:00:05 Inserted 124493 unique entries into SQLite
2026/09/08 12:00:05 Loaded 124493 entries for 120717 postal codes into memory in 437ms
2026/09/08 12:00:05 Postal API ready — 124493 entries in SQLite, listening on :8080
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

### Run checks

```bash
make check
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
    "records": 124493,
    "status": "ok",
    "cache": "memory",
    "cachedRecords": 124493
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
├── main_test.go — automated API, database, CSV, and CORS tests
├── cache.go    — immutable in-memory postal-code lookup cache
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

- General page: https://www.post.japanpost.jp/service/search/zipcode/download/
- Download URL: https://www.post.japanpost.jp/service/search/zipcode/download/kogaki/zip/ken_all.zip
- Encoding: Windows-31J (Shift-JIS) — decoded to UTF-8 automatically
- Latest update: check Japan Post website for data freshness

To refresh data, delete `data/postal.db` and restart the server.

---

## License

MIT
