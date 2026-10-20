package pantry

import (
	"database/sql"
	"encoding/json"
	"errors"
	"github.com/go-chi/chi/v5"
	"net/http"
	"pantry/internal/kindle"
	"time"
)

func (s *Server) ownedDisplay(w http.ResponseWriter, r *http.Request) (string, bool) {
	a := getActor(r)
	if a.Role != "owner" {
		fail(w, 403, "仅户主可配置看板")
		return "", false
	}
	id := chi.URLParam(r, "id")
	var found string
	err := s.store.DB.QueryRow(`SELECT id FROM display_tokens WHERE id=? AND household_id=?`, id, a.HouseholdID).Scan(&found)
	if errors.Is(err, sql.ErrNoRows) {
		fail(w, 404, "看板不存在")
		return "", false
	}
	if err != nil {
		fail(w, 500, "读取看板失败")
		return "", false
	}
	return id, true
}
func (s *Server) readDisplaySchedule(id string) (kindle.Schedule, error) {
	var data string
	err := s.store.DB.QueryRow(`SELECT schedule_json FROM display_schedules WHERE display_id=?`, id).Scan(&data)
	if errors.Is(err, sql.ErrNoRows) {
		return kindle.DefaultSchedule(), nil
	}
	if err != nil {
		return kindle.Schedule{}, err
	}
	var schedule kindle.Schedule
	if err = json.Unmarshal([]byte(data), &schedule); err != nil {
		return schedule, err
	}
	if schedule.Orientation == "" {
		schedule.Orientation = "portrait"
	}
	return schedule, schedule.Validate()
}
func (s *Server) scheduleResponse(w http.ResponseWriter, id string) {
	w.Header().Set("Cache-Control", "no-store")
	schedule, err := s.readDisplaySchedule(id)
	if err != nil {
		fail(w, 500, "读取计划失败")
		return
	}
	serverTime := time.Now()
	next, err := schedule.Next(serverTime)
	if err != nil {
		fail(w, 500, "计算计划失败")
		return
	}
	var nextValue any
	if !next.IsZero() {
		nextValue = next
	}
	layoutWidth, layoutHeight := schedule.LayoutSize()
	jsonOut(w, 200, map[string]any{"schedule": schedule, "next_refresh_at": nextValue, "server_time": serverTime, "width": 600, "height": 800, "layout_width": layoutWidth, "layout_height": layoutHeight})
}
func (s *Server) getDisplaySchedule(w http.ResponseWriter, r *http.Request) {
	if id, ok := s.ownedDisplay(w, r); ok {
		s.scheduleResponse(w, id)
	}
}
func (s *Server) putDisplaySchedule(w http.ResponseWriter, r *http.Request) {
	id, ok := s.ownedDisplay(w, r)
	if !ok {
		return
	}
	var schedule kindle.Schedule
	if decode(r, &schedule) != nil {
		fail(w, 400, "无效计划")
		return
	}
	if schedule.Orientation == "" {
		schedule.Orientation = "portrait"
	}
	// First version intentionally supports China local time only.
	if schedule.Timezone != "Asia/Shanghai" {
		fail(w, 400, "时区须为 Asia/Shanghai")
		return
	}
	if err := schedule.Validate(); err != nil {
		fail(w, 400, err.Error())
		return
	}
	data, err := json.Marshal(schedule)
	if err != nil {
		fail(w, 400, "无效计划")
		return
	}
	_, err = s.store.DB.Exec(`INSERT INTO display_schedules(display_id,schedule_json,updated_at) VALUES(?,?,?) ON CONFLICT(display_id) DO UPDATE SET schedule_json=excluded.schedule_json,updated_at=excluded.updated_at`, id, string(data), now())
	if err != nil {
		fail(w, 500, "保存计划失败")
		return
	}
	s.scheduleResponse(w, id)
}
func (s *Server) deviceSchedule(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	var id string
	err := s.store.DB.QueryRow(`SELECT d.id FROM display_tokens d JOIN households h ON h.id=d.household_id WHERE d.token_hash=? AND h.status='active'`, hashToken(chi.URLParam(r, "token"))).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		fail(w, 401, "看板链接无效")
		return
	}
	if err != nil {
		fail(w, 500, "读取看板失败")
		return
	}
	s.scheduleResponse(w, id)
}
