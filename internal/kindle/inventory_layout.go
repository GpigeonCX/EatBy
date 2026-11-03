package kindle

import (
	"fmt"
	"strings"
)

// inventoryRows packs whole inventory entries horizontally using actual font
// measurements. Oversized entries are shortened; overflow reserves a summary row.
func inventoryRows(items []string, width, capacity int, measure func(string) int) []string {
	if capacity <= 0 {
		return nil
	}
	rows := []string{}
	for _, item := range items {
		item = strings.Join(strings.Fields(item), " ")
		if measure(item) > width {
			runes := []rune(item)
			for len(runes) > 0 && measure(string(runes)+"…") > width {
				runes = runes[:len(runes)-1]
			}
			item = string(runes) + "…"
		}
		if len(rows) > 0 && measure(rows[len(rows)-1]+"   "+item) <= width {
			rows[len(rows)-1] += "   " + item
		} else {
			rows = append(rows, item)
		}
	}
	if len(rows) > capacity {
		rows = rows[:capacity]
		rows[capacity-1] = fmt.Sprintf("…… 共 %d 种，请在手机查看", len(items))
	}
	return rows
}
