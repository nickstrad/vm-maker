package main

import "testing"

func TestDeliveredTextAndFenceBoundaries(t *testing.T) {
	input := "# Lesson\r\n<!-- draw-visual: diagrams/boot.mmd -->\r\n```text\r\n┌───┐\r\n│ A │\r\n└───┘\r\n```\r\n```sh\r\necho ignored\r\n```\r\n"
	b, err := textBlocks(input, false)
	if err != nil || len(b) != 1 {
		t.Fatalf("blocks: %v %v", b, err)
	}
	if b[0].Label != "diagrams/boot.mmd" || b[0].Line != 4 || len(b[0].Text) != 3 || b[0].Text[1] != "│ A │" {
		t.Fatalf("lost source identity or diagram text: %#v", b[0])
	}
	if _, err = textBlocks("```text\nunclosed", false); err == nil {
		t.Fatal("accepted truncated fence")
	}
	if _, err = textBlocks("# no diagrams", false); err == nil {
		t.Fatal("silently accepted no diagrams")
	}
}

func TestTerminalCellWrapping(t *testing.T) {
	for _, tc := range []struct {
		input string
		width int
		rows  []string
	}{
		{"12345", 5, []string{"12345"}},
		{"123456", 5, []string{"12345", "6"}},
		{"界界X", 4, []string{"界界", "X"}},
		{"e\u0301abc", 3, []string{"e\u0301ab", "c"}},
	} {
		got := wrap(tc.input, tc.width)
		if len(got) != len(tc.rows) {
			t.Fatalf("%q: %#v", tc.input, got)
		}
		for i := range got {
			if got[i] != tc.rows[i] {
				t.Fatalf("%q row %d: %q", tc.input, i, got[i])
			}
		}
	}
	if got := expandTabs("A\tB"); got != "A       B" {
		t.Fatalf("tab stops: %q", got)
	}
}
