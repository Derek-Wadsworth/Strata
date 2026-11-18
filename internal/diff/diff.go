package diff

import (
	"bytes"
	"strings"

	"github.com/sergi/go-diff/diffmatchpatch"
)

const binaryProbeSize = 8 * 1024

// Op is the kind of change for a single line.
type Op int

const (
	OpEqual Op = iota
	OpAdd
	OpRemove
)

// Line is one line in a structured diff.
type Line struct {
	Op    Op
	Text  string
	OldNo int // 1-based; 0 when OpAdd
	NewNo int // 1-based; 0 when OpRemove
}

// Hunk is a contiguous group of diff lines.
type Hunk struct {
	Lines []Line
}

// Result is a structured diff suitable for CLI or GUI rendering.
type Result struct {
	Binary       bool
	LinesAdded   int
	LinesRemoved int
	Hunks        []Hunk
}

// Compare returns a line-oriented diff of a (old) vs b (new).
// If either side looks binary, Result.Binary is true and no line hunks are produced.
func Compare(a, b []byte) Result {
	if IsBinary(a) || IsBinary(b) {
		return Result{Binary: true}
	}

	oldText := string(a)
	newText := string(b)

	dmp := diffmatchpatch.New()
	chars1, chars2, lineArray := dmp.DiffLinesToChars(oldText, newText)
	diffs := dmp.DiffMain(chars1, chars2, false)
	diffs = dmp.DiffCharsToLines(diffs, lineArray)

	var (
		lines   []Line
		oldNo   int
		newNo   int
		added   int
		removed int
	)

	for _, d := range diffs {
		parts := splitLines(d.Text)
		for _, part := range parts {
			switch d.Type {
			case diffmatchpatch.DiffEqual:
				oldNo++
				newNo++
				lines = append(lines, Line{Op: OpEqual, Text: part, OldNo: oldNo, NewNo: newNo})
			case diffmatchpatch.DiffInsert:
				newNo++
				added++
				lines = append(lines, Line{Op: OpAdd, Text: part, NewNo: newNo})
			case diffmatchpatch.DiffDelete:
				oldNo++
				removed++
				lines = append(lines, Line{Op: OpRemove, Text: part, OldNo: oldNo})
			}
		}
	}

	return Result{
		LinesAdded:   added,
		LinesRemoved: removed,
		Hunks:        []Hunk{{Lines: lines}},
	}
}

// IsBinary reports whether data looks binary (NUL in the first 8KiB).
func IsBinary(data []byte) bool {
	n := len(data)
	if n > binaryProbeSize {
		n = binaryProbeSize
	}
	return bytes.IndexByte(data[:n], 0) >= 0
}

// splitLines splits text into lines without trailing "\n", preserving empty lines.
// A trailing newline does not create an extra empty line at the end.
func splitLines(text string) []string {
	if text == "" {
		return nil
	}
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	parts := strings.Split(text, "\n")
	// DiffLinesToChars often keeps a trailing newline on chunks; drop empty tail.
	if len(parts) > 0 && parts[len(parts)-1] == "" {
		parts = parts[:len(parts)-1]
	}
	return parts
}
