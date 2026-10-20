package kindle

import (
	"fmt"
	"image"
)

// LayoutSize is the logical page size before rotation. The PNG delivered to
// Kindle is always 600x800; system rotation and touch coordinates stay unchanged.
func (s Schedule) LayoutSize() (int, int) {
	if s.Orientation == "landscape_cw" || s.Orientation == "landscape_ccw" {
		return 800, 600
	}
	return 600, 800
}

// PanelImage converts the laid-out page to the physical panel orientation.
// cw/ccw describe image rotation, not the direction to turn the device.
func (s Schedule) PanelImage(src image.Image) (*image.Gray, error) {
	if err := s.Validate(); err != nil {
		return nil, err
	}
	w, h := s.LayoutSize()
	b := src.Bounds()
	if b.Dx() != w || b.Dy() != h {
		return nil, fmt.Errorf("expected %dx%d page, got %dx%d", w, h, b.Dx(), b.Dy())
	}
	dst := image.NewGray(image.Rect(0, 0, 600, 800))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			dx, dy := x, y
			switch s.Orientation {
			case "landscape_cw":
				dx, dy = h-1-y, x
			case "landscape_ccw":
				dx, dy = y, w-1-x
			}
			dst.Set(dx, dy, src.At(b.Min.X+x, b.Min.Y+y))
		}
	}
	return dst, nil
}
