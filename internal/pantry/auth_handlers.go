package pantry

import (
	"database/sql"
	"net"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
)

func (s *Server) status(w http.ResponseWriter, r *http.Request) {
	var count int
	_ = s.store.DB.QueryRow(`SELECT COUNT(*) FROM household`).Scan(&count)
	jsonOut(w, 200, map[string]bool{"initialized": count > 0})
}
func (s *Server) setup(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Name        string `json:"name"`
		Password    string `json:"password"`
		DisplayName string `json:"display_name"`
	}
	if decode(r, &in) != nil || len(in.Name) < 1 || len(in.Password) < 8 {
		fail(w, 400, "家庭名称不能为空，密码至少 8 位")
		return
	}
	var count int
	_ = s.store.DB.QueryRow(`SELECT COUNT(*) FROM household`).Scan(&count)
	if count > 0 {
		fail(w, 409, "系统已经初始化")
		return
	}
	tx, err := s.store.DB.Begin()
	if err != nil {
		fail(w, 500, err.Error())
		return
	}
	defer tx.Rollback()
	if in.DisplayName == "" {
		in.DisplayName = "户主"
	}
	t := now()
	if _, err = tx.Exec(`INSERT INTO household(id,name,password_hash,created_at) VALUES(1,?,?,?)`, in.Name, hashPassword(in.Password), t); err != nil {
		fail(w, 500, err.Error())
		return
	}
	for i, name := range []string{"冰箱", "冷冻室", "橱柜"} {
		_, err = tx.Exec(`INSERT INTO locations(id,name,sort_order,created_at) VALUES(?,?,?,?)`, newID("loc"), name, i, t)
		if err != nil {
			fail(w, 500, err.Error())
			return
		}
	}
	token := randomToken()
	_, err = tx.Exec(`INSERT INTO sessions(id,token_hash,display_name,role,created_at,last_seen_at) VALUES(?,?,?,?,?,?)`, newID("ses"), hashToken(token), in.DisplayName, "owner", t, t)
	if err != nil {
		fail(w, 500, err.Error())
		return
	}
	if err = tx.Commit(); err != nil {
		fail(w, 500, err.Error())
		return
	}
	setSession(w, r, token)
	jsonOut(w, 201, map[string]bool{"ok": true})
}
func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	key := r.RemoteAddr
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		key = host
	}
	s.loginMu.Lock()
	attempt := s.loginAttempts[key]
	if attempt.Count >= 5 && time.Now().Before(attempt.Reset) {
		s.loginMu.Unlock()
		fail(w, http.StatusTooManyRequests, "密码尝试过多，请稍后再试")
		return
	}
	s.loginMu.Unlock()
	var in struct {
		Password    string `json:"password"`
		DisplayName string `json:"display_name"`
	}
	if decode(r, &in) != nil {
		fail(w, 400, "请求格式错误")
		return
	}
	var hash string
	if err := s.store.DB.QueryRow(`SELECT password_hash FROM household WHERE id=1`).Scan(&hash); err != nil {
		fail(w, 400, "系统尚未初始化")
		return
	}
	if !checkPassword(hash, in.Password) {
		s.loginMu.Lock()
		attempt = s.loginAttempts[key]
		if time.Now().After(attempt.Reset) {
			attempt = loginAttempt{Reset: time.Now().Add(10 * time.Minute)}
		}
		attempt.Count++
		s.loginAttempts[key] = attempt
		s.loginMu.Unlock()
		time.Sleep(300 * time.Millisecond)
		fail(w, 401, "密码错误")
		return
	}
	s.loginMu.Lock()
	delete(s.loginAttempts, key)
	s.loginMu.Unlock()
	if in.DisplayName == "" {
		in.DisplayName = "户主"
	}
	token := randomToken()
	t := now()
	_, err := s.store.DB.Exec(`INSERT INTO sessions(id,token_hash,display_name,role,created_at,last_seen_at) VALUES(?,?,?,?,?,?)`, newID("ses"), hashToken(token), in.DisplayName, "owner", t, t)
	if err != nil {
		fail(w, 500, err.Error())
		return
	}
	setSession(w, r, token)
	jsonOut(w, 200, map[string]bool{"ok": true})
}
func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	if c, e := r.Cookie("pantry_session"); e == nil {
		_, _ = s.store.DB.Exec(`DELETE FROM sessions WHERE token_hash=?`, hashToken(c.Value))
	}
	secure := r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"
	http.SetCookie(w, &http.Cookie{Name: "pantry_session", Value: "", Path: "/", MaxAge: -1, HttpOnly: true, Secure: secure, SameSite: http.SameSiteLaxMode})
	jsonOut(w, 200, map[string]bool{"ok": true})
}
func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	a := getActor(r)
	var name string
	_ = s.store.DB.QueryRow(`SELECT name FROM household WHERE id=1`).Scan(&name)
	jsonOut(w, 200, map[string]any{"name": a.Name, "role": a.Role, "household_name": name})
}
func (s *Server) createInvite(w http.ResponseWriter, r *http.Request) {
	if getActor(r).Role != "owner" {
		fail(w, 403, "仅户主可邀请成员")
		return
	}
	token := randomToken()
	exp := time.Now().Add(24 * time.Hour).UTC()
	_, err := s.store.DB.Exec(`INSERT INTO invites(id,token_hash,expires_at) VALUES(?,?,?)`, newID("inv"), hashToken(token), exp.Format(time.RFC3339))
	if err != nil {
		fail(w, 500, err.Error())
		return
	}
	jsonOut(w, 201, map[string]any{"token": token, "expires_at": exp})
}
func (s *Server) acceptInvite(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Token       string `json:"token"`
		DisplayName string `json:"display_name"`
	}
	if decode(r, &in) != nil || in.DisplayName == "" {
		fail(w, 400, "请输入成员名称")
		return
	}
	var id, expires string
	var used sql.NullString
	err := s.store.DB.QueryRow(`SELECT id,expires_at,used_at FROM invites WHERE token_hash=?`, hashToken(in.Token)).Scan(&id, &expires, &used)
	if err != nil || used.Valid {
		fail(w, 400, "邀请链接无效或已使用")
		return
	}
	exp, _ := time.Parse(time.RFC3339, expires)
	if time.Now().After(exp) {
		fail(w, 400, "邀请链接已过期")
		return
	}
	tx, _ := s.store.DB.Begin()
	defer tx.Rollback()
	t := now()
	token := randomToken()
	_, _ = tx.Exec(`UPDATE invites SET used_at=? WHERE id=?`, t, id)
	_, err = tx.Exec(`INSERT INTO sessions(id,token_hash,display_name,role,created_at,last_seen_at) VALUES(?,?,?,?,?,?)`, newID("ses"), hashToken(token), in.DisplayName, "member", t, t)
	if err != nil {
		fail(w, 500, err.Error())
		return
	}
	_ = tx.Commit()
	setSession(w, r, token)
	jsonOut(w, 200, map[string]bool{"ok": true})
}
func (s *Server) listSessions(w http.ResponseWriter, r *http.Request) {
	if getActor(r).Role != "owner" {
		fail(w, 403, "仅户主可查看设备")
		return
	}
	rows, e := s.store.DB.Query(`SELECT id,display_name,role,created_at,last_seen_at FROM sessions ORDER BY last_seen_at DESC`)
	if e != nil {
		fail(w, 500, e.Error())
		return
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, n, role, c, l string
		_ = rows.Scan(&id, &n, &role, &c, &l)
		out = append(out, map[string]any{"id": id, "display_name": n, "role": role, "created_at": c, "last_seen_at": l})
	}
	jsonOut(w, 200, out)
}
func (s *Server) deleteSession(w http.ResponseWriter, r *http.Request) {
	a := getActor(r)
	if a.Role != "owner" {
		fail(w, 403, "仅户主可撤销设备")
		return
	}
	id := chi.URLParam(r, "id")
	if id == a.SessionID {
		fail(w, 400, "不能撤销当前设备")
		return
	}
	_, _ = s.store.DB.Exec(`DELETE FROM sessions WHERE id=?`, id)
	jsonOut(w, 200, map[string]bool{"ok": true})
}
func (s *Server) createDisplayToken(w http.ResponseWriter, r *http.Request) {
	if getActor(r).Role != "owner" {
		fail(w, 403, "仅户主可创建看板")
		return
	}
	var in struct{ Name string }
	_ = decode(r, &in)
	if in.Name == "" {
		in.Name = "Kindle"
	}
	token := randomToken()
	id := newID("dsp")
	_, e := s.store.DB.Exec(`INSERT INTO display_tokens(id,token_hash,name,created_at) VALUES(?,?,?,?)`, id, hashToken(token), in.Name, now())
	if e != nil {
		fail(w, 500, e.Error())
		return
	}
	jsonOut(w, 201, map[string]string{"id": id, "token": token, "name": in.Name})
}
func (s *Server) listDisplayTokens(w http.ResponseWriter, r *http.Request) {
	if getActor(r).Role != "owner" {
		fail(w, 403, "仅户主可查看看板")
		return
	}
	rows, _ := s.store.DB.Query(`SELECT id,name,created_at FROM display_tokens ORDER BY created_at DESC`)
	defer rows.Close()
	out := []map[string]string{}
	for rows.Next() {
		var id, n, c string
		_ = rows.Scan(&id, &n, &c)
		out = append(out, map[string]string{"id": id, "name": n, "created_at": c})
	}
	jsonOut(w, 200, out)
}
func (s *Server) deleteDisplayToken(w http.ResponseWriter, r *http.Request) {
	if getActor(r).Role != "owner" {
		fail(w, 403, "仅户主可撤销看板")
		return
	}
	_, _ = s.store.DB.Exec(`DELETE FROM display_tokens WHERE id=?`, chi.URLParam(r, "id"))
	jsonOut(w, 200, map[string]bool{"ok": true})
}
