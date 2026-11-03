package kindle

import (
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestInventoryRows(t *testing.T) {
	measure := utf8.RuneCountInString
	t.Run("horizontal and wrap", func(t *testing.T) {
		got := inventoryRows([]string{"排骨 15袋", "牛奶 1件", "牛肉 8袋"}, 16, 4, measure)
		if len(got) != 2 || got[0] != "排骨 15袋   牛奶 1件" || got[1] != "牛肉 8袋" {
			t.Fatalf("unexpected rows: %q", got)
		}
	})
	t.Run("25 items fit without a hard item cap", func(t *testing.T) {
		items := []string{}
		for i := 0; i < 25; i++ {
			items = append(items, fmt.Sprintf("食材%d 2袋", i))
		}
		got := inventoryRows(items, 38, 8, measure)
		if strings.Contains(strings.Join(got, ""), "手机") || !strings.Contains(strings.Join(got, ""), "食材24 2袋") {
			t.Fatalf("lost inventory: %q", got)
		}
	})
	t.Run("overflow summary", func(t *testing.T) {
		got := inventoryRows([]string{"鸡蛋 12个", "牛奶 3瓶", "饺子 2袋"}, 8, 2, measure)
		if len(got) != 2 || !strings.Contains(got[1], "共 3 种") {
			t.Fatalf("missing summary: %q", got)
		}
	})
	t.Run("long names and empty", func(t *testing.T) {
		got := inventoryRows([]string{"很长很长的食材名称 12袋"}, 8, 2, measure)
		if len(got) != 1 || measure(got[0]) > 8 || !strings.HasSuffix(got[0], "…") {
			t.Fatalf("bad truncation: %q", got)
		}
		if len(inventoryRows(nil, 8, 2, measure)) != 0 {
			t.Fatal("expected empty")
		}
	})
}
