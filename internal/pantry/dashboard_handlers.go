package pantry

import (
	"database/sql"
	"github.com/go-chi/chi/v5"
	"net/http"
	"time"
)

func (s *Server) warningDays(hid string) int {
	var d int
	if s.store.DB.QueryRow(`SELECT expiry_warning_days FROM households WHERE id=?`, hid).Scan(&d) != nil {
		return 7
	}
	return d
}
func (s *Server) dashboardData(hid string) (map[string]any, error) {
	rows, e := s.store.DB.Query(batchSelect()+` WHERE b.household_id=? ORDER BY COALESCE(b.expiry_date,'9999-12-31')`, hid)
	if e != nil {
		return nil, e
	}
	expired := []Batch{}
	soon := []Batch{}
	for rows.Next() {
		b, e := scanBatch(rows)
		if e == nil {
			s.hydrateExpiry(hid, &b)
			if b.ExpiryStatus == "expired" {
				expired = append(expired, b)
			} else if b.ExpiryStatus == "soon" {
				soon = append(soon, b)
			}
		}
	}
	rows.Close()
	prows, e := s.store.DB.Query(productSelect()+` WHERE household_id=? ORDER BY favorite DESC,name`, hid)
	if e != nil {
		return nil, e
	}
	low := []Product{}
	for prows.Next() {
		p, e := scanProduct(prows)
		if e == nil {
			s.fillProductStock(hid, &p)
			if p.StockState == "low" || p.StockState == "empty" {
				low = append(low, p)
			}
		}
	}
	prows.Close()
	shopping, _ := s.shoppingData(hid, false)
	todos, _ := s.todoData(hid, false)
	var stockCount int
	_ = s.store.DB.QueryRow(`SELECT COUNT(*) FROM batches WHERE household_id=?`, hid).Scan(&stockCount)
	locations := []map[string]any{}
	lrows, e := s.store.DB.Query(`SELECT l.id,l.name,COUNT(b.id) FROM locations l LEFT JOIN batches b ON b.location_id=l.id AND b.household_id=l.household_id WHERE l.household_id=? GROUP BY l.id ORDER BY l.sort_order,l.name`, hid)
	if e == nil {
		for lrows.Next() {
			var id, n string
			var c int
			_ = lrows.Scan(&id, &n, &c)
			locations = append(locations, map[string]any{"id": id, "name": n, "count": c})
		}
		lrows.Close()
	}
	var recent any
	var eid, action, created string
	if s.store.DB.QueryRow(`SELECT id,action,created_at FROM inventory_events WHERE household_id=? AND undone=0 ORDER BY created_at DESC LIMIT 1`, hid).Scan(&eid, &action, &created) == nil {
		recent = map[string]string{"id": eid, "action": action, "created_at": created}
	}
	return map[string]any{"expired": expired, "expiring": soon, "low_stock": low, "shopping": shopping, "todos": todos, "locations": locations, "stock_count": stockCount, "warning_days": s.warningDays(hid), "recent_event": recent, "generated_at": time.Now()}, nil
}
func (s *Server) dashboard(w http.ResponseWriter, r *http.Request) {
	d, e := s.dashboardData(getActor(r).HouseholdID)
	if e != nil {
		fail(w, 500, e.Error())
		return
	}
	jsonOut(w, 200, d)
}
func (s *Server) displaySummary(w http.ResponseWriter, r *http.Request) {
	var hid, status string
	if s.store.DB.QueryRow(`SELECT d.household_id,h.status FROM display_tokens d JOIN households h ON h.id=d.household_id WHERE d.token_hash=?`, hashToken(chi.URLParam(r, "token"))).Scan(&hid, &status) != nil || status != "active" {
		fail(w, 401, "看板链接无效")
		return
	}
	d, e := s.dashboardData(hid)
	if e != nil {
		fail(w, 500, e.Error())
		return
	}
	jsonOut(w, 200, d)
}

func (s *Server) shoppingData(hid string, all bool) ([]ShoppingItem, error) {
	q := `SELECT id,product_id,name,quantity,unit,checked FROM shopping_items WHERE household_id=?`
	if !all {
		q += ` AND checked=0`
	}
	q += ` ORDER BY checked,created_at`
	rows, e := s.store.DB.Query(q, hid)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []ShoppingItem{}
	for rows.Next() {
		var v ShoppingItem
		var pid, unit sql.NullString
		var qty sql.NullFloat64
		var checked int
		_ = rows.Scan(&v.ID, &pid, &v.Name, &qty, &unit, &checked)
		if pid.Valid {
			v.ProductID = &pid.String
		}
		if unit.Valid {
			v.Unit = &unit.String
		}
		if qty.Valid {
			v.Quantity = &qty.Float64
		}
		v.Checked = checked == 1
		out = append(out, v)
	}
	return out, nil
}
func (s *Server) listShopping(w http.ResponseWriter, r *http.Request) {
	a := getActor(r)
	out, e := s.shoppingData(a.HouseholdID, r.URL.Query().Get("all") == "1")
	if e != nil {
		fail(w, 500, e.Error())
		return
	}
	jsonOut(w, 200, out)
}
func (s *Server) createShopping(w http.ResponseWriter, r *http.Request) {
	a := getActor(r)
	var v ShoppingItem
	if decode(r, &v) != nil || v.Name == "" {
		fail(w, 400, "采购项名称不能为空")
		return
	}
	if v.ProductID != nil {
		var n int
		_ = s.store.DB.QueryRow(`SELECT COUNT(*) FROM products WHERE id=? AND household_id=?`, *v.ProductID, a.HouseholdID).Scan(&n)
		if n == 0 {
			fail(w, 404, "商品不存在")
			return
		}
	}
	v.ID = newID("shp")
	t := now()
	_, e := s.store.DB.Exec(`INSERT INTO shopping_items(id,household_id,product_id,name,quantity,unit,checked,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?)`, v.ID, a.HouseholdID, v.ProductID, v.Name, v.Quantity, v.Unit, v.Checked, t, t)
	if e != nil {
		fail(w, 400, e.Error())
		return
	}
	jsonOut(w, 201, v)
}
func (s *Server) updateShopping(w http.ResponseWriter, r *http.Request) {
	a := getActor(r)
	var v ShoppingItem
	if decode(r, &v) != nil || v.Name == "" {
		fail(w, 400, "采购项名称不能为空")
		return
	}
	v.ID = chi.URLParam(r, "id")
	res, _ := s.store.DB.Exec(`UPDATE shopping_items SET name=?,quantity=?,unit=?,checked=?,updated_at=? WHERE id=? AND household_id=?`, v.Name, v.Quantity, v.Unit, v.Checked, now(), v.ID, a.HouseholdID)
	n, _ := res.RowsAffected()
	if n == 0 {
		fail(w, 404, "采购项不存在")
		return
	}
	jsonOut(w, 200, v)
}
func (s *Server) deleteShopping(w http.ResponseWriter, r *http.Request) {
	a := getActor(r)
	res, _ := s.store.DB.Exec(`DELETE FROM shopping_items WHERE id=? AND household_id=?`, chi.URLParam(r, "id"), a.HouseholdID)
	n, _ := res.RowsAffected()
	if n == 0 {
		fail(w, 404, "采购项不存在")
		return
	}
	jsonOut(w, 200, map[string]bool{"ok": true})
}
func (s *Server) shoppingFromProduct(w http.ResponseWriter, r *http.Request) {
	a := getActor(r)
	id := chi.URLParam(r, "id")
	var name, unit string
	if s.store.DB.QueryRow(`SELECT name,default_unit FROM products WHERE id=? AND household_id=?`, id, a.HouseholdID).Scan(&name, &unit) != nil {
		fail(w, 404, "商品不存在")
		return
	}
	var exists int
	_ = s.store.DB.QueryRow(`SELECT COUNT(*) FROM shopping_items WHERE household_id=? AND product_id=? AND checked=0`, a.HouseholdID, id).Scan(&exists)
	if exists > 0 {
		jsonOut(w, 200, map[string]bool{"exists": true})
		return
	}
	v := ShoppingItem{ID: newID("shp"), ProductID: &id, Name: name, Unit: &unit}
	t := now()
	_, _ = s.store.DB.Exec(`INSERT INTO shopping_items(id,household_id,product_id,name,unit,created_at,updated_at) VALUES(?,?,?,?,?,?,?)`, v.ID, a.HouseholdID, id, name, unit, t, t)
	jsonOut(w, 201, v)
}
func (s *Server) getSettings(w http.ResponseWriter, r *http.Request) {
	a := getActor(r)
	var name string
	var days int
	_ = s.store.DB.QueryRow(`SELECT name,expiry_warning_days FROM households WHERE id=?`, a.HouseholdID).Scan(&name, &days)
	jsonOut(w, 200, map[string]any{"name": name, "expiry_warning_days": days})
}
func (s *Server) updateSettings(w http.ResponseWriter, r *http.Request) {
	a := getActor(r)
	if a.Role != "owner" {
		fail(w, 403, "仅户主可修改设置")
		return
	}
	var in struct {
		Name              string `json:"name"`
		ExpiryWarningDays int    `json:"expiry_warning_days"`
	}
	if decode(r, &in) != nil || in.Name == "" || in.ExpiryWarningDays < 1 || in.ExpiryWarningDays > 90 {
		fail(w, 400, "设置值无效")
		return
	}
	_, _ = s.store.DB.Exec(`UPDATE households SET name=?,expiry_warning_days=? WHERE id=?`, in.Name, in.ExpiryWarningDays, a.HouseholdID)
	jsonOut(w, 200, in)
}
