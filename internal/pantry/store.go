package pantry

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

type Store struct{ DB *sql.DB }

func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path+"?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(5)
	db.SetMaxIdleConns(5)
	var legacy int
	_ = db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='household'`).Scan(&legacy)
	if legacy > 0 {
		db.Close()
		return nil, fmt.Errorf("检测到单家庭旧数据库，请更换空数据目录启动多家庭版本")
	}
	if _, err = db.Exec(schema); err != nil {
		db.Close()
		return nil, err
	}
	return &Store{DB: db}, nil
}

func (s *Store) Close() error { return s.DB.Close() }

func (s *Store) BootstrapAdmin(username, password string) error {
	username = normalizeUsername(username)
	if username == "" || password == "" {
		return nil
	}
	if err := validateCredentials(username, password); err != nil {
		return fmt.Errorf("平台管理员配置无效: %w", err)
	}
	var count int
	_ = s.DB.QueryRow(`SELECT COUNT(*) FROM users WHERE role='platform_admin'`).Scan(&count)
	if count > 0 {
		return nil
	}
	_, err := s.DB.Exec(`INSERT INTO users(id,username,password_hash,display_name,role,status,must_change_password,created_at) VALUES(?,?,?,?,?,?,0,?)`, newID("usr"), username, hashPassword(password), "平台管理员", "platform_admin", "active", now())
	return err
}

func (s *Store) ResetAdminPassword(username, password string) error {
	username = normalizeUsername(username)
	if err := validateCredentials(username, password); err != nil {
		return err
	}
	res, err := s.DB.Exec(`UPDATE users SET password_hash=?,must_change_password=0,status='active' WHERE username=? AND role='platform_admin'`, hashPassword(password), username)
	if err != nil {
		return err
	}
	count, _ := res.RowsAffected()
	if count == 0 {
		return fmt.Errorf("平台管理员 %q 不存在", username)
	}
	_, _ = s.DB.Exec(`DELETE FROM sessions WHERE user_id IN (SELECT id FROM users WHERE username=? AND role='platform_admin')`, username)
	return nil
}

func RunBackups(s *Store, dataDir string, keep int) {
	backup := func() {
		dir := filepath.Join(dataDir, "backups")
		if os.MkdirAll(dir, 0o750) != nil {
			return
		}
		path := filepath.Join(dir, "eatby-"+time.Now().Format("20060102-150405")+".db")
		if _, err := s.DB.Exec("VACUUM INTO ?", path); err != nil {
			return
		}
		files, _ := filepath.Glob(filepath.Join(dir, "eatby-*.db"))
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

func now() string                       { return time.Now().UTC().Format(time.RFC3339) }
func newID(prefix string) string        { return fmt.Sprintf("%s_%d", prefix, time.Now().UnixNano()) }
func normalizeUsername(v string) string { return strings.ToLower(strings.TrimSpace(v)) }

const schema = `
CREATE TABLE IF NOT EXISTS schema_migrations (version INTEGER PRIMARY KEY, applied_at TEXT NOT NULL);
INSERT OR IGNORE INTO schema_migrations(version,applied_at) VALUES(1,CURRENT_TIMESTAMP);
CREATE TABLE IF NOT EXISTS households (
 id TEXT PRIMARY KEY, name TEXT NOT NULL, expiry_warning_days INTEGER NOT NULL DEFAULT 7,
 status TEXT NOT NULL DEFAULT 'active' CHECK(status IN ('active','suspended')), created_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS users (
 id TEXT PRIMARY KEY, household_id TEXT REFERENCES households(id) ON DELETE CASCADE,
 username TEXT NOT NULL COLLATE NOCASE UNIQUE, password_hash TEXT NOT NULL, display_name TEXT NOT NULL,
 role TEXT NOT NULL CHECK(role IN ('platform_admin','owner','member')),
 status TEXT NOT NULL DEFAULT 'active' CHECK(status IN ('active','disabled')),
 must_change_password INTEGER NOT NULL DEFAULT 0, created_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS sessions (
 id TEXT PRIMARY KEY, user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 token_hash TEXT UNIQUE NOT NULL, created_at TEXT NOT NULL, last_seen_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS platform_invites (
 id TEXT PRIMARY KEY, token_hash TEXT UNIQUE NOT NULL, expires_at TEXT NOT NULL, used_at TEXT,
 created_by TEXT NOT NULL REFERENCES users(id), created_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS household_invites (
 id TEXT PRIMARY KEY, household_id TEXT NOT NULL REFERENCES households(id) ON DELETE CASCADE,
 token_hash TEXT UNIQUE NOT NULL, expires_at TEXT NOT NULL, used_at TEXT, created_by TEXT NOT NULL REFERENCES users(id), created_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS display_tokens (
 id TEXT PRIMARY KEY, household_id TEXT NOT NULL REFERENCES households(id) ON DELETE CASCADE,
 token_hash TEXT UNIQUE NOT NULL, name TEXT NOT NULL, created_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS locations (
 id TEXT PRIMARY KEY, household_id TEXT NOT NULL REFERENCES households(id) ON DELETE CASCADE,
 name TEXT NOT NULL, sort_order INTEGER NOT NULL DEFAULT 0, created_at TEXT NOT NULL,
 UNIQUE(household_id,name)
);
CREATE TABLE IF NOT EXISTS products (
 id TEXT PRIMARY KEY, household_id TEXT NOT NULL REFERENCES households(id) ON DELETE CASCADE,
 name TEXT NOT NULL, category TEXT NOT NULL DEFAULT '', default_unit TEXT NOT NULL DEFAULT '件',
 tracking_mode TEXT NOT NULL DEFAULT 'quantity' CHECK(tracking_mode IN ('quantity','level')),
 low_threshold REAL, after_open_days INTEGER, barcode TEXT, favorite INTEGER NOT NULL DEFAULT 0,
 created_at TEXT NOT NULL, updated_at TEXT NOT NULL, UNIQUE(household_id,barcode)
);
CREATE TABLE IF NOT EXISTS batches (
 id TEXT PRIMARY KEY, household_id TEXT NOT NULL REFERENCES households(id) ON DELETE CASCADE,
 product_id TEXT NOT NULL REFERENCES products(id) ON DELETE CASCADE, location_id TEXT NOT NULL REFERENCES locations(id),
 quantity REAL, level TEXT, expiry_date TEXT, opened_at TEXT, note TEXT NOT NULL DEFAULT '',
 version INTEGER NOT NULL DEFAULT 1, created_at TEXT NOT NULL, updated_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS shopping_items (
 id TEXT PRIMARY KEY, household_id TEXT NOT NULL REFERENCES households(id) ON DELETE CASCADE,
 product_id TEXT REFERENCES products(id) ON DELETE SET NULL, name TEXT NOT NULL, quantity REAL, unit TEXT,
 checked INTEGER NOT NULL DEFAULT 0, created_at TEXT NOT NULL, updated_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS todo_items (
 id TEXT PRIMARY KEY, household_id TEXT NOT NULL REFERENCES households(id) ON DELETE CASCADE,
 title TEXT NOT NULL, note TEXT NOT NULL DEFAULT '', checked INTEGER NOT NULL DEFAULT 0,
 created_at TEXT NOT NULL, updated_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS inventory_events (
 id TEXT PRIMARY KEY, household_id TEXT NOT NULL REFERENCES households(id) ON DELETE CASCADE,
 action TEXT NOT NULL, batch_id TEXT NOT NULL, before_json TEXT, after_json TEXT,
 undone INTEGER NOT NULL DEFAULT 0, created_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_sessions_user ON sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_users_household ON users(household_id);
CREATE INDEX IF NOT EXISTS idx_batches_household_product ON batches(household_id,product_id);
CREATE INDEX IF NOT EXISTS idx_batches_household_location ON batches(household_id,location_id);
CREATE INDEX IF NOT EXISTS idx_events_household_created ON inventory_events(household_id,created_at DESC);
`
