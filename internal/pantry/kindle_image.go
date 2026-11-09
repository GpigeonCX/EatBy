package pantry

import (
	"bytes"
	"database/sql"
	"errors"
	"fmt"
	"github.com/go-chi/chi/v5"
	"image/png"
	"net/http"
	"pantry/internal/kindle"
	"strconv"
	"time"
)

func (s *Server) deviceImage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	var id, hid string
	err := s.store.DB.QueryRow(`SELECT d.id,d.household_id FROM display_tokens d JOIN households h ON h.id=d.household_id WHERE d.token_hash=? AND h.status='active'`, hashToken(chi.URLParam(r, "token"))).Scan(&id, &hid)
	if errors.Is(err, sql.ErrNoRows) {
		fail(w, 401, "看板链接无效")
		return
	}
	if err != nil {
		fail(w, 500, "读取看板失败")
		return
	}
	schedule, err := s.readDisplaySchedule(id)
	if err != nil {
		fail(w, 500, "读取计划失败")
		return
	}
	data, err := s.dashboardData(hid)
	if err != nil {
		fail(w, 500, "读取数据失败")
		return
	}
	sections := []kindle.Section{{Title: "快没了"}, {Title: "采购清单"}, {Title: "待办清单"}, {Title: "所有库存"}}
	for _, p := range data["low_stock"].([]Product) {
		sections[0].Lines = append(sections[0].Lines, p.Name)
	}
	for _, v := range data["shopping"].([]ShoppingItem) {
		sections[1].Lines = append(sections[1].Lines, "□ "+v.Name)
	}
	for _, v := range data["todos"].([]TodoItem) {
		sections[2].Lines = append(sections[2].Lines, "□ "+v.Title)
	}
	// Share inventory data with the web dashboard; the renderer handles fitting.
	for _, p := range data["inventory"].([]Product) {
		if p.BatchCount == 0 {
			continue
		}
		value := map[string]string{"enough": "充足", "half": "一半", "low": "快没了", "empty": "用完"}[p.StockState]
		if p.TrackingMode == "quantity" {
			value = fmt.Sprintf("%g%s", p.TotalQuantity, p.DefaultUnit)
		}
		sections[3].Lines = append(sections[3].Lines, fmt.Sprintf("%s\t%s", p.Name, value))
	}
	now := time.Now()
	loc, _ := time.LoadLocation("Asia/Shanghai")
	battery := "未知"
	if n, e := strconv.Atoi(r.URL.Query().Get("battery")); e == nil && n >= 0 && n <= 100 {
		battery = fmt.Sprintf("%d%%", n)
	}
	img, err := kindle.Render(schedule, fmt.Sprintf("更新 %s · 电量 %s", now.In(loc).Format("01-02 15:04"), battery), sections)
	if err != nil {
		fail(w, 503, "图片渲染不可用，请检查服务器中文字体")
		return
	}
	var buf bytes.Buffer
	if err = png.Encode(&buf, img); err != nil {
		fail(w, 500, "图片编码失败")
		return
	}
	next, err := schedule.Next(now)
	if err != nil {
		fail(w, 500, "计划计算失败")
		return
	}
	nextUnix := int64(0)
	if !next.IsZero() {
		nextUnix = next.Unix()
	}
	w.Header().Set("X-Eatby-Next-Refresh", strconv.FormatInt(nextUnix, 10))
	w.Header().Set("X-Eatby-Server-Time", strconv.FormatInt(now.Unix(), 10))
	w.Header().Set("Content-Type", "image/png")
	w.Write(buf.Bytes())
}
