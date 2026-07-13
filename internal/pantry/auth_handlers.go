package pantry

import (
	"database/sql"
	"github.com/go-chi/chi/v5"
	"net"
	"net/http"
	"os"
	"strings"
	"time"
)

func (s *Server) status(w http.ResponseWriter, r *http.Request) {
	var admins int
	_ = s.store.DB.QueryRow(`SELECT COUNT(*) FROM users WHERE role='platform_admin'`).Scan(&admins)
	jsonOut(w, 200, map[string]any{"registration_mode": "invite", "admin_configured": admins > 0, "icp_number": os.Getenv("EATBY_ICP_NUMBER")})
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	key := r.RemoteAddr
	if host, _, e := net.SplitHostPort(r.RemoteAddr); e == nil {
		key = host
	}
	s.loginMu.Lock()
	attempt := s.loginAttempts[key]
	if attempt.Count >= 5 && time.Now().Before(attempt.Reset) {
		s.loginMu.Unlock()
		fail(w, 429, "密码尝试过多，请稍后再试")
		return
	}
	s.loginMu.Unlock()
	var in struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if decode(r, &in) != nil {
		fail(w, 400, "请求格式错误")
		return
	}
	in.Username = normalizeUsername(in.Username)
	var id, hash, status string
	if s.store.DB.QueryRow(`SELECT id,password_hash,status FROM users WHERE username=?`, in.Username).Scan(&id, &hash, &status) != nil || status != "active" || !checkPassword(hash, in.Password) {
		s.loginMu.Lock()
		attempt = s.loginAttempts[key]
		if time.Now().After(attempt.Reset) {
			attempt = loginAttempt{Reset: time.Now().Add(10 * time.Minute)}
		}
		attempt.Count++
		s.loginAttempts[key] = attempt
		s.loginMu.Unlock()
		time.Sleep(300 * time.Millisecond)
		fail(w, 401, "用户名或密码错误")
		return
	}
	s.loginMu.Lock()
	delete(s.loginAttempts, key)
	s.loginMu.Unlock()
	token := randomToken()
	t := now()
	_, e := s.store.DB.Exec(`INSERT INTO sessions(id,user_id,token_hash,created_at,last_seen_at) VALUES(?,?,?,?,?)`, newID("ses"), id, hashToken(token), t, t)
	if e != nil {
		fail(w, 500, e.Error())
		return
	}
	setSession(w, r, token)
	jsonOut(w, 200, map[string]bool{"ok": true})
}
func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	if c, e := r.Cookie("eatby_session"); e == nil {
		_, _ = s.store.DB.Exec(`DELETE FROM sessions WHERE token_hash=?`, hashToken(c.Value))
	}
	clearSession(w, r)
	jsonOut(w, 200, map[string]bool{"ok": true})
}
func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	a := getActor(r)
	out := map[string]any{"id": a.UserID, "username": a.Username, "name": a.Name, "role": a.Role, "household_id": a.HouseholdID, "must_change_password": a.MustChangePassword}
	if a.HouseholdID != "" {
		var n string
		_ = s.store.DB.QueryRow(`SELECT name FROM households WHERE id=?`, a.HouseholdID).Scan(&n)
		out["household_name"] = n
	}
	jsonOut(w, 200, out)
}
func (s *Server) changePassword(w http.ResponseWriter, r *http.Request) {
	a := getActor(r)
	var in struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	if decode(r, &in) != nil || len(in.NewPassword) < 10 {
		fail(w, 400, "新密码至少 10 位")
		return
	}
	var hash string
	_ = s.store.DB.QueryRow(`SELECT password_hash FROM users WHERE id=?`, a.UserID).Scan(&hash)
	if !checkPassword(hash, in.CurrentPassword) {
		fail(w, 401, "当前密码错误")
		return
	}
	_, _ = s.store.DB.Exec(`UPDATE users SET password_hash=?,must_change_password=0 WHERE id=?`, hashPassword(in.NewPassword), a.UserID)
	jsonOut(w, 200, map[string]bool{"ok": true})
}

func (s *Server) registerHousehold(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Code          string `json:"code"`
		Username      string `json:"username"`
		Password      string `json:"password"`
		DisplayName   string `json:"display_name"`
		HouseholdName string `json:"household_name"`
	}
	if decode(r, &in) != nil || strings.TrimSpace(in.DisplayName) == "" || strings.TrimSpace(in.HouseholdName) == "" {
		fail(w, 400, "请完整填写注册信息")
		return
	}
	in.Username = normalizeUsername(in.Username)
	if e := validateCredentials(in.Username, in.Password); e != nil {
		fail(w, 400, e.Error())
		return
	}
	var inviteID, expires string
	var used sql.NullString
	if s.store.DB.QueryRow(`SELECT id,expires_at,used_at FROM platform_invites WHERE token_hash=?`, hashToken(in.Code)).Scan(&inviteID, &expires, &used) != nil || used.Valid {
		fail(w, 400, "平台邀请码无效或已使用")
		return
	}
	exp, _ := time.Parse(time.RFC3339, expires)
	if time.Now().After(exp) {
		fail(w, 400, "平台邀请码已过期")
		return
	}
	tx, e := s.store.DB.Begin()
	if e != nil {
		fail(w, 500, e.Error())
		return
	}
	defer tx.Rollback()
	hid := newID("hh")
	uid := newID("usr")
	t := now()
	if _, e = tx.Exec(`INSERT INTO households(id,name,created_at) VALUES(?,?,?)`, hid, strings.TrimSpace(in.HouseholdName), t); e != nil {
		fail(w, 500, e.Error())
		return
	}
	if _, e = tx.Exec(`INSERT INTO users(id,household_id,username,password_hash,display_name,role,status,created_at) VALUES(?,?,?,?,?,'owner','active',?)`, uid, hid, in.Username, hashPassword(in.Password), strings.TrimSpace(in.DisplayName), t); e != nil {
		fail(w, 409, "用户名已被使用")
		return
	}
	for i, n := range []string{"冰箱", "冷冻室", "橱柜"} {
		if _, e = tx.Exec(`INSERT INTO locations(id,household_id,name,sort_order,created_at) VALUES(?,?,?,?,?)`, newID("loc"), hid, n, i, t); e != nil {
			fail(w, 500, e.Error())
			return
		}
	}
	_, _ = tx.Exec(`UPDATE platform_invites SET used_at=? WHERE id=?`, t, inviteID)
	token := randomToken()
	_, e = tx.Exec(`INSERT INTO sessions(id,user_id,token_hash,created_at,last_seen_at) VALUES(?,?,?,?,?)`, newID("ses"), uid, hashToken(token), t, t)
	if e != nil {
		fail(w, 500, e.Error())
		return
	}
	if e = tx.Commit(); e != nil {
		fail(w, 500, e.Error())
		return
	}
	setSession(w, r, token)
	jsonOut(w, 201, map[string]bool{"ok": true})
}
func (s *Server) createHouseholdInvite(w http.ResponseWriter, r *http.Request) {
	a := getActor(r)
	if a.Role != "owner" {
		fail(w, 403, "仅户主可邀请成员")
		return
	}
	token := randomToken()
	t := now()
	exp := time.Now().Add(24 * time.Hour).UTC()
	_, e := s.store.DB.Exec(`INSERT INTO household_invites(id,household_id,token_hash,expires_at,created_by,created_at) VALUES(?,?,?,?,?,?)`, newID("hinv"), a.HouseholdID, hashToken(token), exp.Format(time.RFC3339), a.UserID, t)
	if e != nil {
		fail(w, 500, e.Error())
		return
	}
	jsonOut(w, 201, map[string]any{"token": token, "expires_at": exp})
}
func (s *Server) acceptHouseholdInvite(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Username    string `json:"username"`
		Password    string `json:"password"`
		DisplayName string `json:"display_name"`
	}
	if decode(r, &in) != nil || strings.TrimSpace(in.DisplayName) == "" {
		fail(w, 400, "请完整填写成员信息")
		return
	}
	in.Username = normalizeUsername(in.Username)
	if e := validateCredentials(in.Username, in.Password); e != nil {
		fail(w, 400, e.Error())
		return
	}
	var iid, hid, expires string
	var used sql.NullString
	if s.store.DB.QueryRow(`SELECT id,household_id,expires_at,used_at FROM household_invites WHERE token_hash=?`, hashToken(chi.URLParam(r, "token"))).Scan(&iid, &hid, &expires, &used) != nil || used.Valid {
		fail(w, 400, "家庭邀请无效或已使用")
		return
	}
	exp, _ := time.Parse(time.RFC3339, expires)
	if time.Now().After(exp) {
		fail(w, 400, "家庭邀请已过期")
		return
	}
	tx, _ := s.store.DB.Begin()
	defer tx.Rollback()
	uid := newID("usr")
	t := now()
	_, e := tx.Exec(`INSERT INTO users(id,household_id,username,password_hash,display_name,role,status,created_at) VALUES(?,?,?,?,?,'member','active',?)`, uid, hid, in.Username, hashPassword(in.Password), strings.TrimSpace(in.DisplayName), t)
	if e != nil {
		fail(w, 409, "用户名已被使用")
		return
	}
	_, _ = tx.Exec(`UPDATE household_invites SET used_at=? WHERE id=?`, t, iid)
	token := randomToken()
	_, _ = tx.Exec(`INSERT INTO sessions(id,user_id,token_hash,created_at,last_seen_at) VALUES(?,?,?,?,?)`, newID("ses"), uid, hashToken(token), t, t)
	if e = tx.Commit(); e != nil {
		fail(w, 500, e.Error())
		return
	}
	setSession(w, r, token)
	jsonOut(w, 200, map[string]bool{"ok": true})
}

func (s *Server) listMembers(w http.ResponseWriter, r *http.Request) {
	a := getActor(r)
	rows, e := s.store.DB.Query(`SELECT id,username,display_name,role,status,must_change_password FROM users WHERE household_id=? ORDER BY role='owner' DESC,created_at`, a.HouseholdID)
	if e != nil {
		fail(w, 500, e.Error())
		return
	}
	defer rows.Close()
	out := []UserSummary{}
	for rows.Next() {
		var u UserSummary
		var m int
		_ = rows.Scan(&u.ID, &u.Username, &u.DisplayName, &u.Role, &u.Status, &m)
		u.MustChangePassword = m == 1
		out = append(out, u)
	}
	jsonOut(w, 200, out)
}
func (s *Server) deleteMember(w http.ResponseWriter, r *http.Request) {
	a := getActor(r)
	if a.Role != "owner" {
		fail(w, 403, "仅户主可移除成员")
		return
	}
	id := chi.URLParam(r, "id")
	var role string
	if s.store.DB.QueryRow(`SELECT role FROM users WHERE id=? AND household_id=?`, id, a.HouseholdID).Scan(&role) != nil {
		fail(w, 404, "成员不存在")
		return
	}
	if role == "owner" {
		fail(w, 400, "不能移除户主")
		return
	}
	_, _ = s.store.DB.Exec(`DELETE FROM users WHERE id=? AND household_id=?`, id, a.HouseholdID)
	jsonOut(w, 200, map[string]bool{"ok": true})
}
func (s *Server) listSessions(w http.ResponseWriter, r *http.Request) {
	a := getActor(r)
	q := `SELECT s.id,u.display_name,u.username,u.role,s.created_at,s.last_seen_at FROM sessions s JOIN users u ON u.id=s.user_id WHERE u.household_id=?`
	args := []any{a.HouseholdID}
	if a.Role != "owner" {
		q += ` AND u.id=?`
		args = append(args, a.UserID)
	}
	q += ` ORDER BY s.last_seen_at DESC`
	rows, e := s.store.DB.Query(q, args...)
	if e != nil {
		fail(w, 500, e.Error())
		return
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, n, user, role, c, l string
		_ = rows.Scan(&id, &n, &user, &role, &c, &l)
		out = append(out, map[string]any{"id": id, "display_name": n, "username": user, "role": role, "created_at": c, "last_seen_at": l})
	}
	jsonOut(w, 200, out)
}
func (s *Server) deleteSession(w http.ResponseWriter, r *http.Request) {
	a := getActor(r)
	id := chi.URLParam(r, "id")
	var uid, hid string
	if s.store.DB.QueryRow(`SELECT u.id,u.household_id FROM sessions s JOIN users u ON u.id=s.user_id WHERE s.id=?`, id).Scan(&uid, &hid) != nil || hid != a.HouseholdID {
		fail(w, 404, "设备不存在")
		return
	}
	if a.Role != "owner" && uid != a.UserID {
		fail(w, 403, "无权撤销该设备")
		return
	}
	_, _ = s.store.DB.Exec(`DELETE FROM sessions WHERE id=?`, id)
	jsonOut(w, 200, map[string]bool{"ok": true})
}

func (s *Server) createDisplayToken(w http.ResponseWriter, r *http.Request) {
	a := getActor(r)
	if a.Role != "owner" {
		fail(w, 403, "仅户主可创建看板")
		return
	}
	var in struct {
		Name string `json:"name"`
	}
	_ = decode(r, &in)
	if strings.TrimSpace(in.Name) == "" {
		in.Name = "Kindle"
	}
	token := randomToken()
	id := newID("dsp")
	_, e := s.store.DB.Exec(`INSERT INTO display_tokens(id,household_id,token_hash,name,created_at) VALUES(?,?,?,?,?)`, id, a.HouseholdID, hashToken(token), in.Name, now())
	if e != nil {
		fail(w, 500, e.Error())
		return
	}
	jsonOut(w, 201, map[string]string{"id": id, "token": token, "name": in.Name})
}
func (s *Server) listDisplayTokens(w http.ResponseWriter, r *http.Request) {
	a := getActor(r)
	rows, e := s.store.DB.Query(`SELECT id,name,created_at FROM display_tokens WHERE household_id=? ORDER BY created_at DESC`, a.HouseholdID)
	if e != nil {
		fail(w, 500, e.Error())
		return
	}
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
	a := getActor(r)
	_, _ = s.store.DB.Exec(`DELETE FROM display_tokens WHERE id=? AND household_id=?`, chi.URLParam(r, "id"), a.HouseholdID)
	jsonOut(w, 200, map[string]bool{"ok": true})
}

func (s *Server) adminCreateSignupCode(w http.ResponseWriter, r *http.Request) {
	a := getActor(r)
	token := randomToken()
	exp := time.Now().Add(7 * 24 * time.Hour).UTC()
	id := newID("pinv")
	_, e := s.store.DB.Exec(`INSERT INTO platform_invites(id,token_hash,expires_at,created_by,created_at) VALUES(?,?,?,?,?)`, id, hashToken(token), exp.Format(time.RFC3339), a.UserID, now())
	if e != nil {
		fail(w, 500, e.Error())
		return
	}
	jsonOut(w, 201, map[string]any{"id": id, "token": token, "expires_at": exp})
}
func (s *Server) adminListSignupCodes(w http.ResponseWriter, r *http.Request) {
	rows, e := s.store.DB.Query(`SELECT id,expires_at,used_at,created_at FROM platform_invites ORDER BY created_at DESC LIMIT 100`)
	if e != nil {
		fail(w, 500, e.Error())
		return
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, exp, c string
		var used sql.NullString
		_ = rows.Scan(&id, &exp, &used, &c)
		out = append(out, map[string]any{"id": id, "expires_at": exp, "used": used.Valid, "created_at": c})
	}
	jsonOut(w, 200, out)
}
func (s *Server) adminListHouseholds(w http.ResponseWriter, r *http.Request) {
	rows, e := s.store.DB.Query(`SELECT h.id,h.name,h.status,h.created_at,COUNT(DISTINCT u.id),COUNT(DISTINCT b.id) FROM households h LEFT JOIN users u ON u.household_id=h.id LEFT JOIN batches b ON b.household_id=h.id GROUP BY h.id ORDER BY h.created_at DESC`)
	if e != nil {
		fail(w, 500, e.Error())
		return
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, n, status, c string
		var users, batches int
		_ = rows.Scan(&id, &n, &status, &c, &users, &batches)
		out = append(out, map[string]any{"id": id, "name": n, "status": status, "created_at": c, "user_count": users, "batch_count": batches})
	}
	jsonOut(w, 200, out)
}
func (s *Server) adminSetHouseholdStatus(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Status string `json:"status"`
	}
	if decode(r, &in) != nil || (in.Status != "active" && in.Status != "suspended") {
		fail(w, 400, "状态无效")
		return
	}
	res, _ := s.store.DB.Exec(`UPDATE households SET status=? WHERE id=?`, in.Status, chi.URLParam(r, "id"))
	n, _ := res.RowsAffected()
	if n == 0 {
		fail(w, 404, "家庭不存在")
		return
	}
	jsonOut(w, 200, map[string]bool{"ok": true})
}
func (s *Server) adminListUsers(w http.ResponseWriter, r *http.Request) {
	rows, e := s.store.DB.Query(`SELECT id,username,display_name,role,status,must_change_password FROM users ORDER BY created_at DESC`)
	if e != nil {
		fail(w, 500, e.Error())
		return
	}
	defer rows.Close()
	out := []UserSummary{}
	for rows.Next() {
		var u UserSummary
		var m int
		_ = rows.Scan(&u.ID, &u.Username, &u.DisplayName, &u.Role, &u.Status, &m)
		u.MustChangePassword = m == 1
		out = append(out, u)
	}
	jsonOut(w, 200, out)
}
func (s *Server) adminResetPassword(w http.ResponseWriter, r *http.Request) {
	var in struct {
		TemporaryPassword string `json:"temporary_password"`
	}
	if decode(r, &in) != nil || len(in.TemporaryPassword) < 10 {
		fail(w, 400, "临时密码至少 10 位")
		return
	}
	id := chi.URLParam(r, "id")
	res, _ := s.store.DB.Exec(`UPDATE users SET password_hash=?,must_change_password=1 WHERE id=? AND role!='platform_admin'`, hashPassword(in.TemporaryPassword), id)
	n, _ := res.RowsAffected()
	if n == 0 {
		fail(w, 404, "用户不存在或不可重置")
		return
	}
	_, _ = s.store.DB.Exec(`DELETE FROM sessions WHERE user_id=?`, id)
	jsonOut(w, 200, map[string]bool{"ok": true})
}
