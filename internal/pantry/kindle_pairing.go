package pantry

import (
	"crypto/rand"
	"database/sql"
	"encoding/base32"
	"errors"
	"net/http"
	"strings"
	"time"
)

// Fixed bounded buckets avoid attacker-controlled key growth. Pairing starts are
// globally capped; authenticated claim attempts are also globally bounded.
func (s *Server) pairingLimit(w http.ResponseWriter, key string, limit int) bool {
	s.loginMu.Lock()
	defer s.loginMu.Unlock()
	key = "kindle-pair:" + key
	a := s.loginAttempts[key]
	n := time.Now()
	if n.After(a.Reset) {
		a = loginAttempt{Reset: n.Add(time.Minute)}
	}
	a.Count++
	s.loginAttempts[key] = a
	if a.Count > limit {
		w.Header().Set("Retry-After", "60")
		fail(w, 429, "操作过于频繁，请稍后重试")
		return false
	}
	return true
}
func (s *Server) startKindlePairing(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if !s.pairingLimit(w, "start", 10) {
		return
	}
	b := make([]byte, 5)
	if _, err := rand.Read(b); err != nil {
		fail(w, 500, "生成配对码失败")
		return
	}
	code := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(b)
	secret := randomToken()
	expires := time.Now().Add(10 * time.Minute).Unix()
	_, err := s.store.DB.Exec(`DELETE FROM kindle_pairings WHERE expires_at<?`, time.Now().Unix())
	if err == nil {
		_, err = s.store.DB.Exec(`INSERT INTO kindle_pairings(secret_hash,code_hash,expires_at) VALUES(?,?,?)`, hashToken(secret), hashToken(code), expires)
	}
	if err != nil {
		fail(w, 500, "创建配对失败")
		return
	}
	// Text response is intentionally easy to consume with Kindle's stock shell.
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Write([]byte(code[:4] + "-" + code[4:] + "\n" + secret + "\n"))
}
func (s *Server) pollKindlePairing(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if !s.pairingLimit(w, "poll", 120) {
		return
	}
	secret := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	var expires int64
	var claimed sql.NullString
	err := s.store.DB.QueryRow(`SELECT expires_at,claimed_id FROM kindle_pairings WHERE secret_hash=?`, hashToken(secret)).Scan(&expires, &claimed)
	if err != nil || expires <= time.Now().Unix() {
		fail(w, 410, "配对已过期或不存在")
		return
	}
	if !claimed.Valid {
		w.WriteHeader(http.StatusAccepted)
		return
	}
	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte("ready\n"))
}
func (s *Server) claimKindlePairing(w http.ResponseWriter, r *http.Request) {
	a := getActor(r)
	if a.Role != "owner" {
		fail(w, 403, "仅户主可绑定设备")
		return
	}
	if !s.pairingLimit(w, "claim", 20) {
		return
	}
	var in struct {
		Code string `json:"code"`
		Name string `json:"name"`
	}
	if decode(r, &in) != nil {
		fail(w, 400, "无效请求")
		return
	}
	code := strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(in.Code), "-", ""))
	if len(code) != 8 {
		fail(w, 400, "请输入八位配对码")
		return
	}
	name := strings.TrimSpace(in.Name)
	if name == "" {
		name = "Kindle"
	}
	if len([]rune(name)) > 60 {
		fail(w, 400, "名称过长")
		return
	}
	tx, err := s.store.DB.Begin()
	if err != nil {
		fail(w, 500, "绑定失败")
		return
	}
	defer tx.Rollback()
	var secretHash string
	var expires int64
	var claimed sql.NullString
	err = tx.QueryRow(`SELECT secret_hash,expires_at,claimed_id FROM kindle_pairings WHERE code_hash=?`, hashToken(code)).Scan(&secretHash, &expires, &claimed)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && (expires <= time.Now().Unix() || claimed.Valid)) {
		fail(w, 400, "配对码无效、已使用或已过期")
		return
	}
	if err != nil {
		fail(w, 500, "读取配对失败")
		return
	}
	id := newID("dsp")
	_, err = tx.Exec(`INSERT INTO display_tokens(id,household_id,token_hash,name,created_at) VALUES(?,?,?,?,?)`, id, a.HouseholdID, secretHash, name, now())
	if err == nil {
		_, err = tx.Exec(`UPDATE kindle_pairings SET claimed_id=? WHERE secret_hash=? AND claimed_id IS NULL`, id, secretHash)
	}
	if err == nil {
		err = tx.Commit()
	}
	if err != nil {
		fail(w, 409, "绑定失败，请重试")
		return
	}
	jsonOut(w, 201, map[string]string{"id": id, "name": name})
}
