package kindle

import (
	"fmt"
	"testing"
)

func TestInventoryGrid(t *testing.T) {
	items := make([]string, 25)
	for i := range items {
		items[i] = fmt.Sprintf("食材%d\t2袋", i)
	}
	for _, tc := range []struct {
		name           string
		width, columns int
	}{{"landscape", 756, 4}, {"portrait", 556, 3}} {
		t.Run(tc.name, func(t *testing.T) {
			cells, cw, hidden := inventoryGrid(items, tc.width, 9)
			if len(cells) != 25 || hidden != 0 {
				t.Fatalf("missing items: %d, hidden %d", len(cells), hidden)
			}
			for i, c := range cells {
				if c.Row != i/tc.columns || c.X != (i%tc.columns)*(cw+16) {
					t.Fatalf("incorrect alignment: %+v", c)
				}
				if c.X+cw > tc.width {
					t.Fatal("cell outside panel")
				}
			}
		})
	}
	t.Run("truncate name without losing quantity", func(t *testing.T) {
		cells, _, _ := inventoryGrid([]string{"超长中文库存名称\t12袋", "五花肉\t4袋", "名字带 空格\t1盒"}, 756, 9)
		if cells[0].Name != "超长中文…" || cells[0].Value != "12袋" || cells[1].Name != "五花肉" || cells[2].Value != "1盒" {
			t.Fatalf("bad cells: %+v", cells)
		}
	})
	t.Run("reserve overflow row", func(t *testing.T) {
		cells, _, hidden := inventoryGrid(items, 756, 3)
		if len(cells) != 8 || hidden != 17 {
			t.Fatalf("got %d cells, %d hidden", len(cells), hidden)
		}
	})
	t.Run("empty and no room", func(t *testing.T) {
		cells, _, hidden := inventoryGrid(nil, 756, 9)
		if len(cells) != 0 || hidden != 0 {
			t.Fatal("nonempty result")
		}
		cells, _, hidden = inventoryGrid(items, 756, 0)
		if len(cells) != 0 || hidden != 25 {
			t.Fatal("bad zero capacity")
		}
	})
}
