package kindle

import (
	"image"
	"image/color"
	"testing"
)

func TestPanelImage(t *testing.T) {
	for _, tc := range []struct {
		orientation string
		corners     [4]image.Point
	}{
		{"portrait", [4]image.Point{{0, 0}, {599, 0}, {0, 799}, {599, 799}}},
		{"landscape_cw", [4]image.Point{{599, 0}, {599, 799}, {0, 0}, {0, 799}}},
		{"landscape_ccw", [4]image.Point{{0, 799}, {0, 0}, {599, 799}, {599, 0}}},
	} {
		t.Run(tc.orientation, func(t *testing.T) {
			s := DefaultSchedule()
			s.Orientation = tc.orientation
			w, h := s.LayoutSize()
			// Non-zero origin verifies correct source indexing.
			src := image.NewGray(image.Rect(3, 5, 3+w, 5+h))
			for i, p := range []image.Point{{3, 5}, {w + 2, 5}, {3, h + 4}, {w + 2, h + 4}} {
				src.SetGray(p.X, p.Y, color.Gray{Y: uint8(40 + i*50)})
			}
			dst, err := s.PanelImage(src)
			if err != nil {
				t.Fatal(err)
			}
			if dst.Bounds() != image.Rect(0, 0, 600, 800) {
				t.Fatal(dst.Bounds())
			}
			for i, p := range tc.corners {
				if dst.GrayAt(p.X, p.Y).Y != uint8(40+i*50) {
					t.Fatalf("corner %d misplaced", i)
				}
			}
		})
	}
	s := DefaultSchedule()
	s.Orientation = "upsideways"
	if s.Validate() == nil {
		t.Fatal("accepted invalid orientation")
	}
	s = DefaultSchedule()
	if _, err := s.PanelImage(image.NewGray(image.Rect(0, 0, 800, 600))); err == nil {
		t.Fatal("accepted wrong size")
	}
}
