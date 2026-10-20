package kindle

import (
	"bytes"
	"image/png"
	"os"
	"testing"
)

func TestRenderWithConfiguredFont(t *testing.T) {
	if os.Getenv("EATBY_KINDLE_FONT") == "" {
		t.Skip("set EATBY_KINDLE_FONT for Chinese font integration test")
	}
	sections := []Section{{"临期食材", []string{"牛奶 · 2026-10-21", "很长很长的中文食材名称应该自动截断而不是超过边框"}}, {"快没了", nil}, {"采购清单", []string{"□ 鸡蛋"}}, {"待办清单", nil}}
	for _, orientation := range []string{"portrait", "landscape_cw", "landscape_ccw"} {
		s := DefaultSchedule()
		s.Orientation = orientation
		img, err := Render(s, "更新 10-20 11:00 · 库存 5 批", sections)
		if err != nil {
			t.Fatal(err)
		}
		var b bytes.Buffer
		if err = png.Encode(&b, img); err != nil {
			t.Fatal(err)
		}
		cfg, err := png.DecodeConfig(&b)
		if err != nil || cfg.Width != 600 || cfg.Height != 800 {
			t.Fatal(cfg, err)
		}
	}
}
