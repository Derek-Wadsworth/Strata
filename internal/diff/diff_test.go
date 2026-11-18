package diff

import (
	"testing"
)

func TestCompareAddedLines(t *testing.T) {
	res := Compare([]byte("a\n"), []byte("a\nb\n"))
	if res.Binary {
		t.Fatal("unexpected binary")
	}
	if res.LinesAdded != 1 || res.LinesRemoved != 0 {
		t.Fatalf("stats = +%d -%d", res.LinesAdded, res.LinesRemoved)
	}
	if !hasOp(res, OpAdd, "b") {
		t.Fatalf("missing added line: %+v", res.Hunks)
	}
}

func TestCompareRemovedLines(t *testing.T) {
	res := Compare([]byte("a\nb\n"), []byte("a\n"))
	if res.LinesAdded != 0 || res.LinesRemoved != 1 {
		t.Fatalf("stats = +%d -%d", res.LinesAdded, res.LinesRemoved)
	}
	if !hasOp(res, OpRemove, "b") {
		t.Fatalf("missing removed line: %+v", res.Hunks)
	}
}

func TestCompareChangedLines(t *testing.T) {
	res := Compare([]byte("hello\n"), []byte("world\n"))
	if res.LinesAdded != 1 || res.LinesRemoved != 1 {
		t.Fatalf("stats = +%d -%d, want +1 -1", res.LinesAdded, res.LinesRemoved)
	}
	if !hasOp(res, OpRemove, "hello") || !hasOp(res, OpAdd, "world") {
		t.Fatalf("change hunks = %+v", res.Hunks)
	}
}

func TestCompareEmptyFile(t *testing.T) {
	res := Compare(nil, []byte("x\n"))
	if res.LinesAdded != 1 || res.LinesRemoved != 0 {
		t.Fatalf("stats = +%d -%d", res.LinesAdded, res.LinesRemoved)
	}
	res = Compare([]byte("x\n"), nil)
	if res.LinesAdded != 0 || res.LinesRemoved != 1 {
		t.Fatalf("stats = +%d -%d", res.LinesAdded, res.LinesRemoved)
	}
}

func TestCompareIdentical(t *testing.T) {
	res := Compare([]byte("same\n"), []byte("same\n"))
	if res.LinesAdded != 0 || res.LinesRemoved != 0 {
		t.Fatalf("stats = +%d -%d", res.LinesAdded, res.LinesRemoved)
	}
	if len(allLines(res)) == 0 {
		t.Fatal("expected equal lines present")
	}
}

func TestCompareBinary(t *testing.T) {
	bin := []byte{0x00, 0x01, 0x02}
	res := Compare(bin, []byte("text"))
	if !res.Binary {
		t.Fatal("want binary result")
	}
	if len(res.Hunks) != 0 {
		t.Fatalf("want no hunks for binary, got %+v", res.Hunks)
	}

	res = Compare([]byte("text"), append([]byte("a"), 0))
	if !res.Binary {
		t.Fatal("want binary when new side has NUL")
	}
}

func TestIsBinary(t *testing.T) {
	if IsBinary([]byte("hello")) {
		t.Fatal("text marked binary")
	}
	if !IsBinary([]byte{1, 2, 0, 3}) {
		t.Fatal("NUL not detected")
	}
}

func hasOp(res Result, op Op, text string) bool {
	for _, line := range allLines(res) {
		if line.Op == op && line.Text == text {
			return true
		}
	}
	return false
}

func allLines(res Result) []Line {
	var out []Line
	for _, h := range res.Hunks {
		out = append(out, h.Lines...)
	}
	return out
}
