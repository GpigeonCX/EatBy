package pantry

import (
	"encoding/json"
	"errors"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"strings"
	"sync"
	"time"
)

type loginAttempt struct {
	Count int
	Reset time.Time
}
type Server struct {
	store         *Store
	web           fs.FS
	loginMu       sync.Mutex
	loginAttempts map[string]loginAttempt
}

func NewServer(store *Store, web fs.FS) http.Handler {
	s := &Server{store: store, web: web, loginAttempts: map[string]loginAttempt{}}
	r := chi.NewRouter()
	r.Use(middleware.Recoverer, middleware.Compress(5), middleware.RequestID)
	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		if e := store.DB.Ping(); e != nil {
			fail(w, 503, "database unavailable")
			return
		}
		jsonOut(w, 200, map[string]bool{"ok": true})
	})
	r.Route("/api/v1", func(r chi.Router) {
		r.Use(s.sameOrigin)
		r.Get("/status", s.status)
		r.Post("/auth/login", s.login)
		r.Post("/auth/logout", s.logout)
		r.Post("/register", s.registerHousehold)
		r.Post("/household-invites/{token}/accept", s.acceptHouseholdInvite)
		r.Get("/display/{token}", s.displaySummary)
		r.Group(func(r chi.Router) {
			r.Use(s.authenticate)
			r.Get("/me", s.me)
			r.Post("/auth/change-password", s.changePassword)
			r.Group(func(r chi.Router) {
				r.Use(s.requireHousehold)
				r.Get("/dashboard", s.dashboard)
				r.Route("/locations", func(r chi.Router) {
					r.Get("/", s.listLocations)
					r.Post("/", s.createLocation)
					r.Put("/{id}", s.updateLocation)
					r.Delete("/{id}", s.deleteLocation)
				})
				r.Route("/products", func(r chi.Router) {
					r.Get("/", s.listProducts)
					r.Post("/", s.createProduct)
					r.Put("/{id}", s.updateProduct)
					r.Delete("/{id}", s.deleteProduct)
					r.Get("/barcode/{code}", s.productByBarcode)
				})
				r.Route("/batches", func(r chi.Router) {
					r.Get("/", s.listBatches)
					r.Post("/", s.createBatch)
					r.Put("/{id}", s.updateBatch)
					r.Delete("/{id}", s.deleteBatch)
					r.Post("/{id}/open", s.openBatch)
					r.Post("/{id}/adjust", s.adjustBatch)
				})
				r.Route("/shopping", func(r chi.Router) {
					r.Get("/", s.listShopping)
					r.Post("/", s.createShopping)
					r.Put("/{id}", s.updateShopping)
					r.Delete("/{id}", s.deleteShopping)
					r.Post("/from-product/{id}", s.shoppingFromProduct)
				})
				r.Route("/todos", func(r chi.Router) {
					r.Get("/", s.listTodos)
					r.Post("/", s.createTodo)
					r.Put("/{id}", s.updateTodo)
					r.Delete("/{id}", s.deleteTodo)
				})
				r.Post("/events/{id}/undo", s.undoEvent)
				r.Get("/settings", s.getSettings)
				r.Put("/settings", s.updateSettings)
				r.Get("/sessions", s.listSessions)
				r.Delete("/sessions/{id}", s.deleteSession)
				r.Get("/members", s.listMembers)
				r.Delete("/members/{id}", s.deleteMember)
				r.Post("/household-invites", s.createHouseholdInvite)
				r.Post("/display-tokens", s.createDisplayToken)
				r.Get("/display-tokens", s.listDisplayTokens)
				r.Delete("/display-tokens/{id}", s.deleteDisplayToken)
			})
			r.Route("/admin", func(r chi.Router) {
				r.Use(s.requireAdmin)
				r.Get("/households", s.adminListHouseholds)
				r.Put("/households/{id}/status", s.adminSetHouseholdStatus)
				r.Get("/users", s.adminListUsers)
				r.Post("/users/{id}/reset-password", s.adminResetPassword)
				r.Get("/signup-codes", s.adminListSignupCodes)
				r.Post("/signup-codes", s.adminCreateSignupCode)
			})
		})
	})
	r.NotFound(s.serveWeb)
	return r
}
func (s *Server) serveWeb(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
	if name == "." || name == "" {
		name = "index.html"
	}
	b, e := fs.ReadFile(s.web, name)
	if errors.Is(e, fs.ErrNotExist) {
		b, e = fs.ReadFile(s.web, "index.html")
	}
	if e != nil {
		http.NotFound(w, r)
		return
	}
	if t := mime.TypeByExtension(path.Ext(name)); t != "" {
		w.Header().Set("Content-Type", t)
	}
	_, _ = w.Write(b)
}
func jsonOut(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func fail(w http.ResponseWriter, status int, message string) {
	jsonOut(w, status, map[string]string{"error": message})
}
func decode(r *http.Request, v any) error {
	d := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	d.DisallowUnknownFields()
	return d.Decode(v)
}
