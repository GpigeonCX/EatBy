package pantry

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
)

func (s *Server) listLocations(w http.ResponseWriter, r *http.Request) {
	rows, e := s.store.DB.Query(`SELECT id,name,sort_order FROM locations ORDER BY sort_order,name`)
	if e != nil {
		fail(w, 500, e.Error())
		return
	}
	defer rows.Close()
	out := []Location{}
	for rows.Next() {
		var v Location
		_ = rows.Scan(&v.ID, &v.Name, &v.SortOrder)
		out = append(out, v)
	}
	jsonOut(w, 200, out)
}
func (s *Server) createLocation(w http.ResponseWriter, r *http.Request) {
	var v Location
	if decode(r, &v) != nil || strings.TrimSpace(v.Name) == "" {
		fail(w, 400, "位置名称不能为空")
		return
	}
	v.ID = newID("loc")
	_, e := s.store.DB.Exec(`INSERT INTO locations(id,name,sort_order,created_at) VALUES(?,?,?,?)`, v.ID, strings.TrimSpace(v.Name), v.SortOrder, now())
	if e != nil {
		fail(w, 409, "位置名称已存在")
		return
	}
	jsonOut(w, 201, v)
}
func (s *Server) updateLocation(w http.ResponseWriter, r *http.Request) {
	var v Location
	if decode(r, &v) != nil || strings.TrimSpace(v.Name) == "" {
		fail(w, 400, "位置名称不能为空")
		return
	}
	v.ID = chi.URLParam(r, "id")
	res, e := s.store.DB.Exec(`UPDATE locations SET name=?,sort_order=? WHERE id=?`, strings.TrimSpace(v.Name), v.SortOrder, v.ID)
	if e != nil {
		fail(w, 409, "位置名称已存在")
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		fail(w, 404, "位置不存在")
		return
	}
	jsonOut(w, 200, v)
}
func (s *Server) deleteLocation(w http.ResponseWriter, r *http.Request) {
	_, e := s.store.DB.Exec(`DELETE FROM locations WHERE id=?`, chi.URLParam(r, "id"))
	if e != nil {
		fail(w, 409, "该位置仍有库存，无法删除")
		return
	}
	jsonOut(w, 200, map[string]bool{"ok": true})
}

func scanProduct(scanner interface{ Scan(...any) error }) (Product, error) {
	var p Product
	var low sql.NullFloat64
	var days sql.NullInt64
	var barcode sql.NullString
	var fav int
	err := scanner.Scan(&p.ID, &p.Name, &p.Category, &p.DefaultUnit, &p.TrackingMode, &low, &days, &barcode, &fav)
	if low.Valid {
		p.LowThreshold = &low.Float64
	}
	if days.Valid {
		v := int(days.Int64)
		p.AfterOpenDays = &v
	}
	if barcode.Valid {
		p.Barcode = &barcode.String
	}
	p.Favorite = fav == 1
	return p, err
}
func productSelect() string {
	return `SELECT id,name,category,default_unit,tracking_mode,low_threshold,after_open_days,barcode,favorite FROM products`
}
func (s *Server) listProducts(w http.ResponseWriter, r *http.Request) {
	rows, e := s.store.DB.Query(productSelect() + ` ORDER BY favorite DESC,name`)
	if e != nil {
		fail(w, 500, e.Error())
		return
	}
	defer rows.Close()
	out := []Product{}
	for rows.Next() {
		p, e := scanProduct(rows)
		if e != nil {
			continue
		}
		_ = s.store.DB.QueryRow(`SELECT COALESCE(SUM(quantity),0),COUNT(*) FROM batches WHERE product_id=?`, p.ID).Scan(&p.TotalQuantity, &p.BatchCount)
		var level sql.NullString
		_ = s.store.DB.QueryRow(`SELECT level FROM batches WHERE product_id=? AND level IS NOT NULL ORDER BY CASE level WHEN 'empty' THEN 0 WHEN 'low' THEN 1 WHEN 'half' THEN 2 ELSE 3 END LIMIT 1`, p.ID).Scan(&level)
		if p.TrackingMode == "quantity" {
			if p.TotalQuantity <= 0 {
				p.StockState = "empty"
			} else if p.LowThreshold != nil && p.TotalQuantity <= *p.LowThreshold {
				p.StockState = "low"
			} else {
				p.StockState = "enough"
			}
		} else if level.Valid {
			p.StockState = level.String
		} else {
			p.StockState = "empty"
		}
		out = append(out, p)
	}
	jsonOut(w, 200, out)
}
func validateProduct(p *Product) error {
	p.Name = strings.TrimSpace(p.Name)
	if p.Name == "" {
		return fmt.Errorf("商品名称不能为空")
	}
	if p.DefaultUnit == "" {
		p.DefaultUnit = "件"
	}
	if p.TrackingMode != "quantity" && p.TrackingMode != "level" {
		return fmt.Errorf("库存模式无效")
	}
	if p.Barcode != nil && strings.TrimSpace(*p.Barcode) == "" {
		p.Barcode = nil
	}
	return nil
}
func (s *Server) createProduct(w http.ResponseWriter, r *http.Request) {
	var p Product
	if decode(r, &p) != nil {
		fail(w, 400, "请求格式错误")
		return
	}
	if e := validateProduct(&p); e != nil {
		fail(w, 400, e.Error())
		return
	}
	p.ID = newID("prd")
	t := now()
	_, e := s.store.DB.Exec(`INSERT INTO products(id,name,category,default_unit,tracking_mode,low_threshold,after_open_days,barcode,favorite,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?)`, p.ID, p.Name, p.Category, p.DefaultUnit, p.TrackingMode, p.LowThreshold, p.AfterOpenDays, p.Barcode, p.Favorite, t, t)
	if e != nil {
		fail(w, 409, "条码已被其他商品使用")
		return
	}
	jsonOut(w, 201, p)
}
func (s *Server) updateProduct(w http.ResponseWriter, r *http.Request) {
	var p Product
	if decode(r, &p) != nil {
		fail(w, 400, "请求格式错误")
		return
	}
	if e := validateProduct(&p); e != nil {
		fail(w, 400, e.Error())
		return
	}
	p.ID = chi.URLParam(r, "id")
	res, e := s.store.DB.Exec(`UPDATE products SET name=?,category=?,default_unit=?,tracking_mode=?,low_threshold=?,after_open_days=?,barcode=?,favorite=?,updated_at=? WHERE id=?`, p.Name, p.Category, p.DefaultUnit, p.TrackingMode, p.LowThreshold, p.AfterOpenDays, p.Barcode, p.Favorite, now(), p.ID)
	if e != nil {
		fail(w, 409, "条码已被其他商品使用")
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		fail(w, 404, "商品不存在")
		return
	}
	jsonOut(w, 200, p)
}
func (s *Server) deleteProduct(w http.ResponseWriter, r *http.Request) {
	res, _ := s.store.DB.Exec(`DELETE FROM products WHERE id=?`, chi.URLParam(r, "id"))
	n, _ := res.RowsAffected()
	if n == 0 {
		fail(w, 404, "商品不存在")
		return
	}
	jsonOut(w, 200, map[string]bool{"ok": true})
}
func (s *Server) productByBarcode(w http.ResponseWriter, r *http.Request) {
	p, e := scanProduct(s.store.DB.QueryRow(productSelect()+` WHERE barcode=?`, chi.URLParam(r, "code")))
	if e != nil {
		fail(w, 404, "未找到该条码")
		return
	}
	jsonOut(w, 200, p)
}

func scanBatch(scanner interface{ Scan(...any) error }) (Batch, error) {
	var b Batch
	var q sql.NullFloat64
	var level, expiry, opened sql.NullString
	err := scanner.Scan(&b.ID, &b.ProductID, &b.LocationID, &q, &level, &expiry, &opened, &b.Note, &b.Version, &b.CreatedAt, &b.UpdatedAt, &b.ProductName, &b.LocationName, &b.Unit)
	if q.Valid {
		b.Quantity = &q.Float64
	}
	if level.Valid {
		b.Level = &level.String
	}
	if expiry.Valid {
		b.ExpiryDate = &expiry.String
	}
	if opened.Valid {
		b.OpenedAt = &opened.String
	}
	return b, err
}
func batchSelect() string {
	return `SELECT b.id,b.product_id,b.location_id,b.quantity,b.level,b.expiry_date,b.opened_at,b.note,b.version,b.created_at,b.updated_at,p.name,l.name,p.default_unit FROM batches b JOIN products p ON p.id=b.product_id JOIN locations l ON l.id=b.location_id`
}
func (s *Server) hydrateExpiry(b *Batch) {
	var days sql.NullInt64
	_ = s.store.DB.QueryRow(`SELECT after_open_days FROM products WHERE id=?`, b.ProductID).Scan(&days)
	effective := b.ExpiryDate
	if b.OpenedAt != nil && days.Valid {
		opened, e := time.Parse("2006-01-02", (*b.OpenedAt)[:10])
		if e == nil {
			d := opened.AddDate(0, 0, int(days.Int64)).Format("2006-01-02")
			if effective == nil || d < *effective {
				effective = &d
			}
		}
	}
	b.EffectiveExpiry = effective
	if effective != nil {
		today := time.Now().Format("2006-01-02")
		warn := time.Now().AddDate(0, 0, s.warningDays()).Format("2006-01-02")
		if *effective < today {
			b.ExpiryStatus = "expired"
		} else if *effective <= warn {
			b.ExpiryStatus = "soon"
		} else {
			b.ExpiryStatus = "ok"
		}
	}
}
func (s *Server) listBatches(w http.ResponseWriter, r *http.Request) {
	query := batchSelect()
	args := []any{}
	clauses := []string{}
	if v := r.URL.Query().Get("product_id"); v != "" {
		clauses = append(clauses, "b.product_id=?")
		args = append(args, v)
	}
	if v := r.URL.Query().Get("location_id"); v != "" {
		clauses = append(clauses, "b.location_id=?")
		args = append(args, v)
	}
	if len(clauses) > 0 {
		query += " WHERE " + strings.Join(clauses, " AND ")
	}
	query += ` ORDER BY COALESCE(b.expiry_date,'9999-12-31'),b.updated_at DESC`
	rows, e := s.store.DB.Query(query, args...)
	if e != nil {
		fail(w, 500, e.Error())
		return
	}
	defer rows.Close()
	out := []Batch{}
	for rows.Next() {
		b, e := scanBatch(rows)
		if e == nil {
			s.hydrateExpiry(&b)
			out = append(out, b)
		}
	}
	jsonOut(w, 200, out)
}
func validateBatch(s *Store, b *Batch) error {
	var mode string
	if e := s.DB.QueryRow(`SELECT tracking_mode FROM products WHERE id=?`, b.ProductID).Scan(&mode); e != nil {
		return fmt.Errorf("商品不存在")
	}
	var x int
	if e := s.DB.QueryRow(`SELECT COUNT(*) FROM locations WHERE id=?`, b.LocationID).Scan(&x); e != nil || x == 0 {
		return fmt.Errorf("位置不存在")
	}
	if mode == "quantity" {
		if b.Quantity == nil || *b.Quantity < 0 {
			return fmt.Errorf("请输入有效数量")
		}
		b.Level = nil
	} else {
		if b.Level == nil || (*b.Level != "enough" && *b.Level != "half" && *b.Level != "low" && *b.Level != "empty") {
			return fmt.Errorf("请选择库存状态")
		}
		b.Quantity = nil
	}
	return nil
}
func eventJSON(v any) *string {
	if v == nil {
		return nil
	}
	b, _ := json.Marshal(v)
	s := string(b)
	return &s
}
func (s *Server) recordEvent(tx *sql.Tx, action string, before, after *Batch) {
	id := newID("evt")
	bid := ""
	if after != nil {
		bid = after.ID
	} else if before != nil {
		bid = before.ID
	}
	_, _ = tx.Exec(`INSERT INTO inventory_events(id,action,batch_id,before_json,after_json,created_at) VALUES(?,?,?,?,?,?)`, id, action, bid, eventJSON(before), eventJSON(after), now())
}
func (s *Server) createBatch(w http.ResponseWriter, r *http.Request) {
	var b Batch
	if decode(r, &b) != nil {
		fail(w, 400, "请求格式错误")
		return
	}
	if e := validateBatch(s.store, &b); e != nil {
		fail(w, 400, e.Error())
		return
	}
	b.ID = newID("bat")
	b.Version = 1
	b.CreatedAt = now()
	b.UpdatedAt = b.CreatedAt
	tx, _ := s.store.DB.Begin()
	defer tx.Rollback()
	_, e := tx.Exec(`INSERT INTO batches(id,product_id,location_id,quantity,level,expiry_date,opened_at,note,version,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?)`, b.ID, b.ProductID, b.LocationID, b.Quantity, b.Level, b.ExpiryDate, b.OpenedAt, b.Note, b.Version, b.CreatedAt, b.UpdatedAt)
	if e != nil {
		fail(w, 400, e.Error())
		return
	}
	s.recordEvent(tx, "create", nil, &b)
	_ = tx.Commit()
	s.hydrateExpiry(&b)
	jsonOut(w, 201, b)
}
func (s *Server) getBatch(id string) (Batch, error) {
	return scanBatch(s.store.DB.QueryRow(batchSelect()+` WHERE b.id=?`, id))
}
func (s *Server) updateBatch(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	before, e := s.getBatch(id)
	if e != nil {
		fail(w, 404, "库存不存在")
		return
	}
	var b Batch
	if decode(r, &b) != nil {
		fail(w, 400, "请求格式错误")
		return
	}
	b.ID = id
	if b.Version == 0 {
		b.Version = before.Version
	}
	if e = validateBatch(s.store, &b); e != nil {
		fail(w, 400, e.Error())
		return
	}
	b.CreatedAt = before.CreatedAt
	b.UpdatedAt = now()
	tx, _ := s.store.DB.Begin()
	defer tx.Rollback()
	res, e := tx.Exec(`UPDATE batches SET product_id=?,location_id=?,quantity=?,level=?,expiry_date=?,opened_at=?,note=?,version=version+1,updated_at=? WHERE id=? AND version=?`, b.ProductID, b.LocationID, b.Quantity, b.Level, b.ExpiryDate, b.OpenedAt, b.Note, b.UpdatedAt, id, b.Version)
	if e != nil {
		fail(w, 500, e.Error())
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		fail(w, 409, "库存已被其他成员修改，请刷新后重试")
		return
	}
	b.Version++
	s.recordEvent(tx, "update", &before, &b)
	_ = tx.Commit()
	s.hydrateExpiry(&b)
	jsonOut(w, 200, b)
}
func (s *Server) deleteBatch(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	before, e := s.getBatch(id)
	if e != nil {
		fail(w, 404, "库存不存在")
		return
	}
	version, _ := strconv.Atoi(r.URL.Query().Get("version"))
	if version == 0 {
		version = before.Version
	}
	tx, _ := s.store.DB.Begin()
	defer tx.Rollback()
	res, _ := tx.Exec(`DELETE FROM batches WHERE id=? AND version=?`, id, version)
	n, _ := res.RowsAffected()
	if n == 0 {
		fail(w, 409, "库存已被其他成员修改，请刷新后重试")
		return
	}
	s.recordEvent(tx, "delete", &before, nil)
	_ = tx.Commit()
	jsonOut(w, 200, map[string]bool{"ok": true})
}
func (s *Server) openBatch(w http.ResponseWriter, r *http.Request) {
	b, e := s.getBatch(chi.URLParam(r, "id"))
	if e != nil {
		fail(w, 404, "库存不存在")
		return
	}
	d := time.Now().Format("2006-01-02")
	b.OpenedAt = &d
	s.updateBatchValue(w, r, b)
}
func (s *Server) adjustBatch(w http.ResponseWriter, r *http.Request) {
	b, e := s.getBatch(chi.URLParam(r, "id"))
	if e != nil {
		fail(w, 404, "库存不存在")
		return
	}
	var in struct {
		Delta   *float64 `json:"delta"`
		Level   *string  `json:"level"`
		Version int      `json:"version"`
	}
	if decode(r, &in) != nil {
		fail(w, 400, "请求格式错误")
		return
	}
	if in.Version != 0 {
		b.Version = in.Version
	}
	if b.Quantity != nil && in.Delta != nil {
		v := *b.Quantity + *in.Delta
		if v < 0 {
			v = 0
		}
		b.Quantity = &v
	} else if in.Level != nil {
		b.Level = in.Level
	} else {
		fail(w, 400, "没有可调整的值")
		return
	}
	s.updateBatchValue(w, r, b)
}
func (s *Server) updateBatchValue(w http.ResponseWriter, r *http.Request, b Batch) {
	before, e := s.getBatch(b.ID)
	if e != nil {
		fail(w, 404, "库存不存在")
		return
	}
	if e = validateBatch(s.store, &b); e != nil {
		fail(w, 400, e.Error())
		return
	}
	b.UpdatedAt = now()
	tx, _ := s.store.DB.Begin()
	defer tx.Rollback()
	res, _ := tx.Exec(`UPDATE batches SET quantity=?,level=?,opened_at=?,version=version+1,updated_at=? WHERE id=? AND version=?`, b.Quantity, b.Level, b.OpenedAt, b.UpdatedAt, b.ID, b.Version)
	n, _ := res.RowsAffected()
	if n == 0 {
		fail(w, 409, "库存已被其他成员修改，请刷新后重试")
		return
	}
	b.Version++
	s.recordEvent(tx, "adjust", &before, &b)
	_ = tx.Commit()
	s.hydrateExpiry(&b)
	jsonOut(w, 200, b)
}

func (s *Server) undoEvent(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var beforeJSON, afterJSON sql.NullString
	var batchID string
	var undone int
	e := s.store.DB.QueryRow(`SELECT batch_id,before_json,after_json,undone FROM inventory_events WHERE id=?`, id).Scan(&batchID, &beforeJSON, &afterJSON, &undone)
	if e != nil || undone == 1 {
		fail(w, 400, "操作无法撤销")
		return
	}
	tx, _ := s.store.DB.Begin()
	defer tx.Rollback()
	if !beforeJSON.Valid {
		_, e = tx.Exec(`DELETE FROM batches WHERE id=?`, batchID)
	} else {
		var b Batch
		_ = json.Unmarshal([]byte(beforeJSON.String), &b)
		_, e = tx.Exec(`INSERT INTO batches(id,product_id,location_id,quantity,level,expiry_date,opened_at,note,version,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET product_id=excluded.product_id,location_id=excluded.location_id,quantity=excluded.quantity,level=excluded.level,expiry_date=excluded.expiry_date,opened_at=excluded.opened_at,note=excluded.note,version=batches.version+1,updated_at=excluded.updated_at`, b.ID, b.ProductID, b.LocationID, b.Quantity, b.Level, b.ExpiryDate, b.OpenedAt, b.Note, b.Version, b.CreatedAt, now())
	}
	if e != nil {
		fail(w, 409, "当前库存状态无法撤销")
		return
	}
	_, _ = tx.Exec(`UPDATE inventory_events SET undone=1 WHERE id=?`, id)
	_ = tx.Commit()
	jsonOut(w, 200, map[string]bool{"ok": true})
}
