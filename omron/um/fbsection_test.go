package um

import (
	"encoding/json"
	"os"
	"slices"
	"testing"
)

func TestCPU21FBSectionNativeInstructions(t *testing.T) {
	data, err := os.ReadFile("testdata/cpu21-fb-sections.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Kind    string
		Number  int
		Words   []uint32
		Text    []string
		Offsets []int
	}
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	for _, c := range cases {
		read := func(words []uint32) (sectionReader, bool) {
			r := sectionReader{remaining: words}
			var ok bool
			if c.Kind == "instance" {
				ok = r.instance(c.Number)
			} else {
				ok = r.definition(c.Number)
			}
			return r, ok && len(r.remaining) == 0
		}
		r, ok := read(c.Words)
		if !ok || len(r.rows) != len(c.Text) {
			t.Fatal(c.Kind, c.Number, r.offset)
		}
		for i, row := range r.rows {
			end := len(c.Words)
			if i+1 < len(c.Offsets) {
				end = c.Offsets[i+1]
			}
			if row.Text != c.Text[i] || row.OffsetWords != c.Offsets[i] || !slices.Equal(row.Words, c.Words[c.Offsets[i]:end]) {
				t.Fatal(row)
			}
		}
		for end := 0; end < len(c.Words); end++ {
			if _, ok := read(c.Words[:end]); ok {
				t.Fatalf("accepted truncation at %d", end)
			}
		}
		for _, at := range c.Offsets {
			damaged := slices.Clone(c.Words)
			damaged[at] ^= 0x80000000
			if _, ok := read(damaged); ok {
				t.Fatalf("accepted unknown opcode at %d", at)
			}
		}
	}
}
