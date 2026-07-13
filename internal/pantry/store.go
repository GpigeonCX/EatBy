package pantry

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

type Store struct{ DB *sql.DB }

func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path+"?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, err
	}
	// Dashboard aggregation performs small nested reads while iterating result sets.
	// WAL plus a short busy timeout keeps these safe without serializing every read.
	db.SetMaxOpenConns(5)
	db.SetMaxIdleConns(5)
	if _, err = db.Exec(schema); err != nil {
		db.Close()
		return nil, err
	}
	return &Store{DB: db}, nil
}

func (s *Store) Close() error { return s.DB.Close() }

func RunBackups(s *Store, dataDir string, keep int) {
	backup := func() {
		dir := filepath.Join(dataDir, "backups")
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return
		}
		path := filepath.Join(dir, "pantry-"+time.Now().Format("20060102-150405")+".db")
		_, _ = s.DB.Exec("VACUUM INTO ?", path)
		files, _ := filepath.Glob(filepath.Join(dir, "pantry-*.db"))
		if len(files) > keep {
			for _, name := range files[:len(files)-keep] {
				_ = os.Remove(name)
			}
		}
	}
	for {
		now := time.Now()
		next := time.Date(now.Year(), now.Month(), now.Day()+1, 3, 0, 0, 0, now.Location())
		time.Sleep(time.Until(next))
		backup()
	}
}

func now() string                { return time.Now().UTC().Format(time.RFC3339) }
func newID(prefix string) string { return fmt.Sprintf("%s_%d", prefix, time.Now().UnixNano()) }

const schema = `
CREATE TABLE IF NOT EXISTS household (
  id INTEGER PRIMARY KEY CHECK (id = 1), name TEXT NOT NULL, password_hash TEXT NOT NULL,
  expiry_warning_days INTEGER NOT NULL DEFAULT 7, created_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS sessions (
  id TEXT PRIMARY KEY, token_hash TEXT UNIQUE NOT NULL, display_name TEXT NOT NULL,
  role TEXT NOT NULL CHECK(role IN ('owner','member')), created_at TEXT NOT NULL, last_seen_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS invites (
  id TEXT PRIMARY KEY, token_hash TEXT UNIQUE NOT NULL, expires_at TEXT NOT NULL, used_at TEXT
);
CREATE TABLE IF NOT EXISTS display_tokens (
  id TEXT PRIMARY KEY, token_hash TEXT UNIQUE NOT NULL, name TEXT NOT NULL, created_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS locations (
  id TEXT PRIMARY KEY, name TEXT NOT NULL UNIQUE, sort_order INTEGER NOT NULL DEFAULT 0, created_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS products (
  id TEXT PRIMARY KEY, name TEXT NOT NULL, category TEXT NOT NULL DEFAULT '', default_unit TEXT NOT NULL DEFAULT '件',
  tracking_mode TEXT NOT NULL DEFAULT 'quantity' CHECK(tracking_mode IN ('quantity','level')),
  low_threshold REAL, after_open_days INTEGER, barcode TEXT UNIQUE, favorite INTEGER NOT NULL DEFAULT 0,
  created_at TEXT NOT NULL, updated_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS batches (
  id TEXT PRIMARY KEY, product_id TEXT NOT NULL REFERENCES products(id) ON DELETE CASCADE,
  location_id TEXT NOT NULL REFERENCES locations(id), quantity REAL, level TEXT,
  expiry_date TEXT, opened_at TEXT, note TEXT NOT NULL DEFAULT '', version INTEGER NOT NULL DEFAULT 1,
  created_at TEXT NOT NULL, updated_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS shopping_items (
  id TEXT PRIMARY KEY, product_id TEXT REFERENCES products(id) ON DELETE SET NULL, name TEXT NOT NULL,
  quantity REAL, unit TEXT, checked INTEGER NOT NULL DEFAULT 0, created_at TEXT NOT NULL, updated_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS inventory_events (
  id TEXT PRIMARY KEY, action TEXT NOT NULL, batch_id TEXT NOT NULL, before_json TEXT, after_json TEXT,
  undone INTEGER NOT NULL DEFAULT 0, created_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_batches_product ON batches(product_id);
CREATE INDEX IF NOT EXISTS idx_batches_location ON batches(location_id);
CREATE INDEX IF NOT EXISTS idx_events_created ON inventory_events(created_at DESC);
`
