package kindle

import "strings"

type inventoryCell struct {
	Name, Value string
	X, Row      int
}

// Each row uses identical column positions, independent of name lengths.
// 800px landscape fits four columns; 600px portrait fits three.
func inventoryGrid(items []string, width, rows int) (cells []inventoryCell, cellWidth, hidden int) {
	const gap = 16
	columns := (width + gap) / 180
	if columns < 1 {
		columns = 1
	}
	if columns > 5 {
		columns = 5
	}
	cellWidth = (width - (columns-1)*gap) / columns
	if rows <= 0 {
		return nil, cellWidth, len(items)
	}
	capacity := rows * columns
	if len(items) > capacity {
		capacity = (rows - 1) * columns
	}
	shown := len(items)
	if shown > capacity {
		shown = capacity
	}
	for i, item := range items[:shown] {
		name, value, _ := strings.Cut(item, "\t")
		name = strings.Join(strings.Fields(name), " ")
		runes := []rune(name)
		if len(runes) > 4 {
			name = string(runes[:4]) + "…"
		}
		cells = append(cells, inventoryCell{Name: name, Value: value, X: (i % columns) * (cellWidth + gap), Row: i / columns})
	}
	return cells, cellWidth, len(items) - shown
}
