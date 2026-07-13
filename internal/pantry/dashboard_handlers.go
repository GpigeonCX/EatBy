package pantry

import (
	"database/sql"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
)

func (s *Server) warningDays() int {
	var d int
	if s.store.DB.QueryRow(`SELECT expiry_warning_days FROM household WHERE id=1`).Scan(&d) != nil {
		return 7
	}
	return d
}
func (s *Server) dashboardData() (map[string]any, error) {
	rows, e := s.store.DB.Query(batchSelect() + ` ORDER BY COALESCE(b.expiry_date,'9999-12-31')`)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	expired := []Batch{}
	soon := []Batch{}
	for rows.Next() {
		b, e := scanBatch(rows)
		if e != nil {
			continue
		}
		s.hydrateExpiry(&b)
		if b.ExpiryStatus == "expired" {
			expired = append(expired, b)
		} else if b.ExpiryStatus == "soon" {
			soon = append(soon, b)
		}
	}
	products := []Product{}
	prows, e := s.store.DB.Query(productSelect() + ` ORDER BY favorite DESC,name`)
	if e != nil {
		return nil, e
	}
	defer prows.Close()
	for prows.Next() {
		p, e := scanProduct(prows)
		if e != nil {
			continue
		}
		_ = s.store.DB.QueryRow(`SELECT COALESCE(SUM(quantity),0),COUNT(*) FROM batches WHERE product_id=?`, p.ID).Scan(&p.TotalQuantity, &p.BatchCount)
		if p.TrackingMode == "quantity" {
			if p.TotalQuantity <= 0 {
				p.StockState = "empty"
			} else if p.LowThreshold != nil && p.TotalQuantity <= *p.LowThreshold {
				p.StockState = "low"
			}
		} else {
			var level sql.NullString
			_ = s.store.DB.QueryRow(`SELECT level FROM batches WHERE product_id=? ORDER BY CASE level WHEN 'empty' THEN 0 WHEN 'low' THEN 1 WHEN 'half' THEN 2 ELSE 3 END LIMIT 1`, p.ID).Scan(&level)
			if level.Valid {
				p.StockState = level.String
			} else {
				p.StockState = "empty"
			}
		}
		if p.StockState == "low" || p.StockState == "empty" {
			products = append(products, p)
		}
	}
	shopping, _ := s.shoppingData(false)
	var stockCount int
	_ = s.store.DB.QueryRow(`SELECT COUNT(*) FROM batches`).Scan(&stockCount)
	locations := []map[string]any{}
	lrows, _ := s.store.DB.Query(`SELECT l.id,l.name,COUNT(b.id) FROM locations l LEFT JOIN batches b ON b.location_id=l.id GROUP BY l.id ORDER BY l.sort_order,l.name`)
	if lrows != nil {
		defer lrows.Close()
		for lrows.Next() {
			var id, n string
			var c int
			_ = lrows.Scan(&id, &n, &c)
			locations = append(locations, map[string]any{"id": id, "name": n, "count": c})
		}
	}
	var recent any = nil
	var eid, action, created string
	if s.store.DB.QueryRow(`SELECT id,action,created_at FROM inventory_events WHERE undone=0 ORDER BY created_at DESC LIMIT 1`).Scan(&eid, &action, &created) == nil {
		recent = map[string]string{"id": eid, "action": action, "created_at": created}
	}
	return map[string]any{"expired": expired, "expiring": soon, "low_stock": products, "shopping": shopping, "locations": locations, "stock_count": stockCount, "warning_days": s.warningDays(), "recent_event": recent, "generated_at": time.Now()}, nil
}
func (s *Server) dashboard(w http.ResponseWriter, r *http.Request) {
	d, e := s.dashboardData()
	if e != nil {
		fail(w, 500, e.Error())
		return
	}
	jsonOut(w, 200, d)
}
func (s *Server) displaySummary(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	var count int
	_ = s.store.DB.QueryRow(`SELECT COUNT(*) FROM display_tokens WHERE token_hash=?`, hashToken(token)).Scan(&count)
	if count == 0 {
		fail(w, 401, "看板链接无效")
		return
	}
	d, e := s.dashboardData()
	if e != nil {
		fail(w, 500, e.Error())
		return
	}
	jsonOut(w, 200, d)
}

func (s *Server) shoppingData(all bool) ([]ShoppingItem, error) {
	q := `SELECT id,product_id,name,quantity,unit,checked FROM shopping_items`
	if !all {
		q += ` WHERE checked=0`
	}
	q += ` ORDER BY checked,created_at`
	rows, e := s.store.DB.Query(q)
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
	out, e := s.shoppingData(r.URL.Query().Get("all") == "1")
	if e != nil {
		fail(w, 500, e.Error())
		return
	}
	jsonOut(w, 200, out)
}
func (s *Server) createShopping(w http.ResponseWriter, r *http.Request) {
	var v ShoppingItem
	if decode(r, &v) != nil || v.Name == "" {
		fail(w, 400, "采购项名称不能为空")
		return
	}
	v.ID = newID("shp")
	t := now()
	_, e := s.store.DB.Exec(`INSERT INTO shopping_items(id,product_id,name,quantity,unit,checked,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?)`, v.ID, v.ProductID, v.Name, v.Quantity, v.Unit, v.Checked, t, t)
	if e != nil {
		fail(w, 400, e.Error())
		return
	}
	jsonOut(w, 201, v)
}
func (s *Server) updateShopping(w http.ResponseWriter, r *http.Request) {
	var v ShoppingItem
	if decode(r, &v) != nil || v.Name == "" {
		fail(w, 400, "采购项名称不能为空")
		return
	}
	v.ID = chi.URLParam(r, "id")
	res, _ := s.store.DB.Exec(`UPDATE shopping_items SET product_id=?,name=?,quantity=?,unit=?,checked=?,updated_at=? WHERE id=?`, v.ProductID, v.Name, v.Quantity, v.Unit, v.Checked, now(), v.ID)
	n, _ := res.RowsAffected()
	if n == 0 {
		fail(w, 404, "采购项不存在")
		return
	}
	jsonOut(w, 200, v)
}
func (s *Server) deleteShopping(w http.ResponseWriter, r *http.Request) {
	_, _ = s.store.DB.Exec(`DELETE FROM shopping_items WHERE id=?`, chi.URLParam(r, "id"))
	jsonOut(w, 200, map[string]bool{"ok": true})
}
func (s *Server) shoppingFromProduct(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var name, unit string
	if s.store.DB.QueryRow(`SELECT name,default_unit FROM products WHERE id=?`, id).Scan(&name, &unit) != nil {
		fail(w, 404, "商品不存在")
		return
	}
	var exists int
	_ = s.store.DB.QueryRow(`SELECT COUNT(*) FROM shopping_items WHERE product_id=? AND checked=0`, id).Scan(&exists)
	if exists > 0 {
		jsonOut(w, 200, map[string]bool{"exists": true})
		return
	}
	v := ShoppingItem{ID: newID("shp"), ProductID: &id, Name: name, Unit: &unit}
	t := now()
	_, _ = s.store.DB.Exec(`INSERT INTO shopping_items(id,product_id,name,unit,created_at,updated_at) VALUES(?,?,?,?,?,?)`, v.ID, id, name, unit, t, t)
	jsonOut(w, 201, v)
}

func (s *Server) getSettings(w http.ResponseWriter, r *http.Request) {
	var name string
	var days int
	_ = s.store.DB.QueryRow(`SELECT name,expiry_warning_days FROM household WHERE id=1`).Scan(&name, &days)
	jsonOut(w, 200, map[string]any{"name": name, "expiry_warning_days": days})
}
func (s *Server) updateSettings(w http.ResponseWriter, r *http.Request) {
	if getActor(r).Role != "owner" {
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
	_, _ = s.store.DB.Exec(`UPDATE household SET name=?,expiry_warning_days=? WHERE id=1`, in.Name, in.ExpiryWarningDays)
	jsonOut(w, 200, in)
}
