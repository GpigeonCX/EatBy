package kindle

import (
	"fmt"
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
	"image"
	"image/color"
	"image/draw"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type Section struct {
	Title string
	Lines []string
}

var fontOnce sync.Once
var pageFont *opentype.Font
var fontError error

func loadFont() (*opentype.Font, error) {
	fontOnce.Do(func() {
		path := os.Getenv("EATBY_KINDLE_FONT")
		if path == "" {
			_ = filepath.WalkDir("/usr/share/fonts", func(p string, d os.DirEntry, err error) error {
				if err == nil && !d.IsDir() && strings.Contains(filepath.Base(p), "NotoSansCJK-Regular") && (strings.HasSuffix(p, ".ttc") || strings.HasSuffix(p, ".ttf") || strings.HasSuffix(p, ".otf")) {
					path = p
					return filepath.SkipAll
				}
				return nil
			})
		}
		if path == "" {
			fontError = fmt.Errorf("set EATBY_KINDLE_FONT to a Chinese TTF/TTC font")
			return
		}
		data, err := os.ReadFile(path)
		if err != nil {
			fontError = err
			return
		}
		if strings.HasSuffix(strings.ToLower(path), ".ttc") {
			c, e := opentype.ParseCollection(data)
			if e != nil {
				fontError = e
				return
			}
			index := 0
			// Noto CJK collections: JP, KR, SC, TC, HK. Prefer Simplified Chinese.
			if strings.Contains(filepath.Base(path), "NotoSansCJK") && c.NumFonts() >= 3 {
				index = 2
			}
			pageFont, fontError = c.Font(index)
		} else {
			pageFont, fontError = opentype.Parse(data)
		}
	})
	return pageFont, fontError
}
func Render(s Schedule, subtitle string, sections []Section) (*image.Gray, error) {
	f, err := loadFont()
	if err != nil {
		return nil, err
	}
	face, err := opentype.NewFace(f, &opentype.FaceOptions{Size: 20, DPI: 72, Hinting: font.HintingFull})
	if err != nil {
		return nil, err
	}
	defer face.Close()
	w, h := s.LayoutSize()
	page := image.NewGray(image.Rect(0, 0, w, h))
	draw.Draw(page, page.Bounds(), image.White, image.Point{}, draw.Src)
	text := func(x, y, max int, value string) {
		value = strings.Join(strings.Fields(value), " ")
		if font.MeasureString(face, value).Ceil() > max {
			r := []rune(value)
			for len(r) > 0 && font.MeasureString(face, string(r)+"…").Ceil() > max {
				r = r[:len(r)-1]
			}
			value = string(r) + "…"
		}
		d := font.Drawer{Dst: page, Src: image.Black, Face: face, Dot: fixed.P(x, y)}
		d.DrawString(value)
	}
	// Three compact panels on the first row and a full-width inventory panel below.
	gap := 12
	margin := 12
	footer := 0
	if s.ShowStatus {
		footer = 30
		text(margin, h-10, w-2*margin, subtitle)
	}
	topH := (h - 2*margin - gap - footer) * 2 / 5
	bottomH := h - 2*margin - gap - footer - topH
	drawSection := func(section Section, x, y, pw, ph int) {
		draw.Draw(page, image.Rect(x, y, x+pw, y+ph), image.NewUniform(color.Gray{0}), image.Point{}, draw.Src)
		draw.Draw(page, image.Rect(x+2, y+2, x+pw-2, y+ph-2), image.White, image.Point{}, draw.Src)
		text(x+10, y+29, pw-20, section.Title)
		lines := section.Lines
		if len(lines) == 0 {
			lines = []string{"暂无事项"}
		}
		capacity := (ph - 50) / 30
		shown := len(lines)
		if shown > capacity {
			shown = capacity - 1
		}
		for j := 0; j < shown; j++ {
			text(x+10, y+61+j*30, pw-20, lines[j])
		}
		if shown < len(lines) {
			text(x+10, y+61+shown*30, pw-20, fmt.Sprintf("还有 %d 项，请在手机查看", len(lines)-shown))
		}
	}
	if len(sections) > 0 {
		panelW := (w - 2*margin - 2*gap) / 3
		for i := 0; i < len(sections) && i < 3; i++ {
			drawSection(sections[i], margin+i*(panelW+gap), margin, panelW, topH)
		}
	}
	if len(sections) > 3 {
		drawSection(sections[3], margin, margin+topH+gap, w-2*margin, bottomH)
	}
	return s.PanelImage(page)
}
