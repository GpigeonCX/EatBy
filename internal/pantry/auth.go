package pantry

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"golang.org/x/crypto/argon2"
)

type actor struct {
	UserID, HouseholdID, Username, Name, Role string
	MustChangePassword                        bool
}
type actorKey struct{}

var usernamePattern = regexp.MustCompile(`^[a-z0-9_.-]{3,32}$`)

func randomToken() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}
func hashToken(v string) string { h := sha256.Sum256([]byte(v)); return hex.EncodeToString(h[:]) }
func hashPassword(password string) string {
	salt := make([]byte, 16)
	_, _ = rand.Read(salt)
	h := argon2.IDKey([]byte(password), salt, 1, 64*1024, 4, 32)
	return base64.RawStdEncoding.EncodeToString(salt) + "." + base64.RawStdEncoding.EncodeToString(h)
}
func checkPassword(encoded, password string) bool {
	p := strings.Split(encoded, ".")
	if len(p) != 2 {
		return false
	}
	salt, e1 := base64.RawStdEncoding.DecodeString(p[0])
	expected, e2 := base64.RawStdEncoding.DecodeString(p[1])
	if e1 != nil || e2 != nil {
		return false
	}
	actual := argon2.IDKey([]byte(password), salt, 1, 64*1024, 4, 32)
	return subtleEqual(expected, actual)
}
func subtleEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	var v byte
	for i := range a {
		v |= a[i] ^ b[i]
	}
	return v == 0
}
func validateCredentials(username, password string) error {
	if !usernamePattern.MatchString(normalizeUsername(username)) {
		return fmt.Errorf("用户名需为 3-32 位小写字母、数字、点、横线或下划线")
	}
	if len(password) < 10 {
		return fmt.Errorf("密码至少 10 位")
	}
	return nil
}

func (s *Server) authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("eatby_session")
		if err != nil {
			fail(w, 401, "请先登录")
			return
		}
		var a actor
		var household, householdStatus *string
		var must int
		err = s.store.DB.QueryRow(`SELECT u.id,u.username,u.display_name,u.role,u.household_id,u.must_change_password,h.status FROM sessions s JOIN users u ON u.id=s.user_id LEFT JOIN households h ON h.id=u.household_id WHERE s.token_hash=? AND u.status='active'`, hashToken(cookie.Value)).Scan(&a.UserID, &a.Username, &a.Name, &a.Role, &household, &must, &householdStatus)
		if err != nil {
			fail(w, 401, "登录已失效")
			return
		}
		if household != nil {
			a.HouseholdID = *household
		}
		a.MustChangePassword = must == 1
		if householdStatus != nil && *householdStatus != "active" {
			fail(w, 403, "该家庭已暂停使用")
			return
		}
		_, _ = s.store.DB.Exec(`UPDATE sessions SET last_seen_at=? WHERE token_hash=?`, now(), hashToken(cookie.Value))
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), actorKey{}, a)))
	})
}
func (s *Server) requireHousehold(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		a := getActor(r)
		if a.HouseholdID == "" || a.Role == "platform_admin" {
			fail(w, 403, "需要家庭成员权限")
			return
		}
		if a.MustChangePassword {
			fail(w, 403, "请先修改临时密码")
			return
		}
		next.ServeHTTP(w, r)
	})
}
func (s *Server) requireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if getActor(r).Role != "platform_admin" {
			fail(w, 403, "需要平台管理员权限")
			return
		}
		next.ServeHTTP(w, r)
	})
}
func (s *Server) sameOrigin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead && r.Method != http.MethodOptions {
			if raw := r.Header.Get("Origin"); raw != "" {
				u, e := url.Parse(raw)
				if e != nil || !strings.EqualFold(u.Host, r.Host) {
					fail(w, 403, "请求来源无效")
					return
				}
			}
		}
		next.ServeHTTP(w, r)
	})
}
func getActor(r *http.Request) actor { a, _ := r.Context().Value(actorKey{}).(actor); return a }
func setSession(w http.ResponseWriter, r *http.Request, token string) {
	secure := r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"
	http.SetCookie(w, &http.Cookie{Name: "eatby_session", Value: token, Path: "/", HttpOnly: true, Secure: secure, SameSite: http.SameSiteLaxMode, MaxAge: int((180 * 24 * time.Hour).Seconds())})
}
func clearSession(w http.ResponseWriter, r *http.Request) {
	secure := r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"
	http.SetCookie(w, &http.Cookie{Name: "eatby_session", Value: "", Path: "/", HttpOnly: true, Secure: secure, SameSite: http.SameSiteLaxMode, MaxAge: -1})
}
