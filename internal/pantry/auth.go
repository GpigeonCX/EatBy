package pantry

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/crypto/argon2"
)

type actor struct{ SessionID, Name, Role string }
type actorKey struct{}

func randomToken() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}
func hashToken(s string) string { h := sha256.Sum256([]byte(s)); return hex.EncodeToString(h[:]) }
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

func (s *Server) authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("pantry_session")
		if err != nil {
			fail(w, http.StatusUnauthorized, "请先登录")
			return
		}
		var a actor
		err = s.store.DB.QueryRow(`SELECT id,display_name,role FROM sessions WHERE token_hash=?`, hashToken(cookie.Value)).Scan(&a.SessionID, &a.Name, &a.Role)
		if err != nil {
			fail(w, http.StatusUnauthorized, "登录已失效")
			return
		}
		_, _ = s.store.DB.Exec(`UPDATE sessions SET last_seen_at=? WHERE id=?`, now(), a.SessionID)
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), actorKey{}, a)))
	})
}

func (s *Server) sameOrigin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead && r.Method != http.MethodOptions {
			if raw := r.Header.Get("Origin"); raw != "" {
				u, err := url.Parse(raw)
				if err != nil || !strings.EqualFold(u.Host, r.Host) {
					fail(w, http.StatusForbidden, "请求来源无效")
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
	http.SetCookie(w, &http.Cookie{Name: "pantry_session", Value: token, Path: "/", HttpOnly: true, Secure: secure, SameSite: http.SameSiteLaxMode, MaxAge: int((180 * 24 * time.Hour).Seconds())})
}
