package pantry

import (
	"github.com/go-chi/chi/v5"
	"net/http"
	"strings"
)

func (s *Server) todoData(hid string, all bool) ([]TodoItem, error) {
	q := `SELECT id,title,note,checked FROM todo_items WHERE household_id=?`
	if !all {
		q += ` AND checked=0`
	}
	q += ` ORDER BY checked,created_at`
	rows, err := s.store.DB.Query(q, hid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []TodoItem{}
	for rows.Next() {
		var v TodoItem
		var checked int
		if err := rows.Scan(&v.ID, &v.Title, &v.Note, &checked); err != nil {
			return nil, err
		}
		v.Checked = checked == 1
		out = append(out, v)
	}
	return out, rows.Err()
}
func (s *Server) listTodos(w http.ResponseWriter, r *http.Request) {
	out, err := s.todoData(getActor(r).HouseholdID, r.URL.Query().Get("all") == "1")
	if err != nil {
		fail(w, 500, err.Error())
		return
	}
	jsonOut(w, 200, out)
}
func (s *Server) createTodo(w http.ResponseWriter, r *http.Request) {
	a := getActor(r)
	var v TodoItem
	if decode(r, &v) != nil || strings.TrimSpace(v.Title) == "" {
		fail(w, 400, "待办事项不能为空")
		return
	}
	v.Title = strings.TrimSpace(v.Title)
	v.ID = newID("todo")
	t := now()
	_, err := s.store.DB.Exec(`INSERT INTO todo_items(id,household_id,title,note,checked,created_at,updated_at) VALUES(?,?,?,?,?,?,?)`, v.ID, a.HouseholdID, v.Title, v.Note, v.Checked, t, t)
	if err != nil {
		fail(w, 400, err.Error())
		return
	}
	jsonOut(w, 201, v)
}
func (s *Server) updateTodo(w http.ResponseWriter, r *http.Request) {
	a := getActor(r)
	var v TodoItem
	if decode(r, &v) != nil || strings.TrimSpace(v.Title) == "" {
		fail(w, 400, "待办事项不能为空")
		return
	}
	v.ID = chi.URLParam(r, "id")
	v.Title = strings.TrimSpace(v.Title)
	res, err := s.store.DB.Exec(`UPDATE todo_items SET title=?,note=?,checked=?,updated_at=? WHERE id=? AND household_id=?`, v.Title, v.Note, v.Checked, now(), v.ID, a.HouseholdID)
	if err != nil {
		fail(w, 400, err.Error())
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		fail(w, 404, "待办事项不存在")
		return
	}
	jsonOut(w, 200, v)
}
func (s *Server) deleteTodo(w http.ResponseWriter, r *http.Request) {
	a := getActor(r)
	res, _ := s.store.DB.Exec(`DELETE FROM todo_items WHERE id=? AND household_id=?`, chi.URLParam(r, "id"), a.HouseholdID)
	n, _ := res.RowsAffected()
	if n == 0 {
		fail(w, 404, "待办事项不存在")
		return
	}
	jsonOut(w, 200, map[string]bool{"ok": true})
}
