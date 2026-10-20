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
	// Minimal dashboard: only four panels, no brand, timestamp or stock header.
	gap := 12
	margin := 12
	cw := (w - 2*margin - gap) / 2
	footer := 0
	if s.ShowStatus {
		footer = 30
		text(margin, h-10, w-2*margin, subtitle)
	}
	ch := (h - 2*margin - gap - footer) / 2
	for i, section := range sections {
		if i >= 4 {
			break
		}
		x := margin + (i%2)*(cw+gap)
		y := margin + (i/2)*(ch+gap)
		draw.Draw(page, image.Rect(x, y, x+cw, y+ch), image.NewUniform(color.Gray{0}), image.Point{}, draw.Src)
		draw.Draw(page, image.Rect(x+2, y+2, x+cw-2, y+ch-2), image.White, image.Point{}, draw.Src)
		text(x+10, y+29, cw-20, section.Title)
		lines := section.Lines
		if len(lines) == 0 {
			lines = []string{"暂无事项"}
		}
		capacity := (ch - 50) / 30
		shown := len(lines)
		if shown > capacity {
			shown = capacity - 1
		}
		for j := 0; j < shown; j++ {
			text(x+10, y+61+j*30, cw-20, lines[j])
		}
		if shown < len(lines) {
			text(x+10, y+61+shown*30, cw-20, fmt.Sprintf("还有 %d 项，请在手机查看", len(lines)-shown))
		}
	}
	return s.PanelImage(page)
}
