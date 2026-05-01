package tree2

import (
	"strings"
	"testing"
)

func TestRenderAppendsSlashForDirectories(t *testing.T) {
	out := Render([]Node{{Name: "internal", IsDir: true}}, Options{Width: 80})
	if out != "internal/" {
		t.Fatalf("out = %q, want internal/", out)
	}
}

func TestRenderForcedColumnsUseDeterministicChunks(t *testing.T) {
	nodes := []Node{
		{Name: "a", IsDir: true, Children: []Node{{Name: "aa", IsDir: true}}},
		{Name: "b", IsDir: true, Children: []Node{{Name: "bb", IsDir: true}}},
		{Name: "c", IsDir: true},
	}

	out := Render(nodes, Options{Columns: 2, Width: 80})
	lines := strings.Split(out, "\n")
	if !strings.Contains(lines[0], "a/") || !strings.Contains(lines[0], "b/") {
		t.Fatalf("first row should contain a and b blocks:\n%s", out)
	}
	if !strings.Contains(out, "\nc/") {
		t.Fatalf("second block row should contain c:\n%s", out)
	}
}

func TestLayoutRowsGroupsSimilarHeightBlocks(t *testing.T) {
	blocks := []Block{
		testBlock(0, "a", 8, 10),
		testBlock(1, "b", 2, 10),
		testBlock(2, "c", 7, 10),
		testBlock(3, "d", 3, 10),
		testBlock(4, "e", 6, 10),
	}

	lay := layoutBlocks(blocks, 40, 0)
	got := rowIndexes(lay.rows)
	want := [][]int{{0, 2, 4}, {1, 3}}
	if !equalRows(got, want) {
		t.Fatalf("rows = %v, want %v", got, want)
	}
}

func TestLayoutRowsAvoidsPairingTallWithShortWhenBetterMatchFits(t *testing.T) {
	blocks := []Block{
		testBlock(0, "a", 8, 10),
		testBlock(1, "b", 2, 10),
		testBlock(2, "c", 7, 10),
		testBlock(3, "d", 3, 10),
	}

	lay := layoutBlocks(blocks, 24, 0)
	got := rowIndexes(lay.rows)
	want := [][]int{{0, 2}, {1, 3}}
	if !equalRows(got, want) {
		t.Fatalf("rows = %v, want %v", got, want)
	}
}

func TestLayoutRowsKeepsRowsWithinWidth(t *testing.T) {
	blocks := []Block{
		testBlock(0, "a", 3, 10),
		testBlock(1, "b", 3, 10),
		testBlock(2, "c", 3, 10),
	}

	lay := layoutBlocks(blocks, 24, 0)
	for _, row := range lay.rows {
		if got := rowWidth(row); got > 24 {
			t.Fatalf("row width = %d, want <= 24 for row %v", got, rowIndexes([]layoutRow{row})[0])
		}
	}
}

func TestLayoutBlocksUsesGlobalColumnSlotsForLongNames(t *testing.T) {
	blocks := []Block{
		testBlock(0, "a", 8, 10),
		testBlock(1, "b", 7, 10),
		testBlock(2, "c", 6, 10),
		testBlock(3, "long", 5, 30),
		testBlock(4, "d", 4, 10),
		testBlock(5, "e", 3, 10),
	}

	lay := layoutBlocks(blocks, 70, 0)
	got := rowIndexes(lay.rows)
	want := [][]int{{0, 1, 2}, {3, 4}, {5}}
	if !equalRows(got, want) {
		t.Fatalf("rows = %v, want %v", got, want)
	}
	if lay.columns != 3 {
		t.Fatalf("columns = %d, want 3", lay.columns)
	}
	if lay.rows[1].cells[0].span != 2 {
		t.Fatalf("long block span = %d, want 2", lay.rows[1].cells[0].span)
	}
	if lay.rows[0].cells[2].start != lay.rows[1].cells[1].start {
		t.Fatalf("third column start mismatch: %d != %d", lay.rows[0].cells[2].start, lay.rows[1].cells[1].start)
	}
}

func TestBlockSpanCanUsePartOfColumnGap(t *testing.T) {
	block := testBlock(0, "slightly-wide", 1, 18)
	if got := blockSpan(block, 16, 6); got != 1 {
		t.Fatalf("span = %d, want 1", got)
	}
}

func TestLayoutRowsHandlesEmptyAndSingleBlock(t *testing.T) {
	if lay := layoutBlocks(nil, 80, 0); lay.rows != nil {
		t.Fatalf("empty rows = %v, want nil", lay.rows)
	}

	lay := layoutBlocks([]Block{testBlock(0, "a", 1, 10)}, 80, 0)
	got := rowIndexes(lay.rows)
	want := [][]int{{0}}
	if !equalRows(got, want) {
		t.Fatalf("rows = %v, want %v", got, want)
	}
}

func TestInferColumnsGrowsWithWidth(t *testing.T) {
	blocks := []Block{{Width: 10}, {Width: 10}, {Width: 10}}
	if got := InferColumns(blocks, 20); got != 1 {
		t.Fatalf("cols at width 20 = %d, want 1", got)
	}
	if got := InferColumns(blocks, 24); got != 2 {
		t.Fatalf("cols at width 24 = %d, want 2", got)
	}
	if got := InferColumns(blocks, 40); got != 3 {
		t.Fatalf("cols at width 40 = %d, want 3", got)
	}
}

func TestRenderColumnsOverrideAutoInference(t *testing.T) {
	nodes := []Node{
		{Name: "a", IsDir: true},
		{Name: "b", IsDir: true},
	}
	out := Render(nodes, Options{Columns: 2, Width: 80})
	if !strings.Contains(strings.Split(out, "\n")[0], "b/") {
		t.Fatalf("forced columns should place b on first row:\n%s", out)
	}
}

func TestRenderLongNameReducesColumnsWithoutTruncation(t *testing.T) {
	long := "this-is-a-very-long-directory-name"
	nodes := []Node{
		{Name: long, IsDir: true},
		{Name: "b", IsDir: true},
	}
	out := Render(nodes, Options{Width: 20})
	if !strings.Contains(out, long+"/") {
		t.Fatalf("long name should not be truncated:\n%s", out)
	}
	if strings.Contains(strings.Split(out, "\n")[0], "b/") {
		t.Fatalf("long name should force single-column row:\n%s", out)
	}
}

func TestRenderEmptyTreeIsStable(t *testing.T) {
	if got := Render(nil, Options{Width: 80}); got != "" {
		t.Fatalf("empty render = %q, want empty", got)
	}
}

func TestRenderUsesDisplayWidthForChineseNames(t *testing.T) {
	nodes := []Node{
		{Name: "源码", IsDir: true, Children: []Node{{Name: "子目录", IsDir: true}}},
		{Name: "docs", IsDir: true, Children: []Node{{Name: "plans", IsDir: true}}},
	}
	out := Render(nodes, Options{Columns: 2, Width: 80})
	lines := strings.Split(out, "\n")
	firstDocs := displayIndex(lines[0], "docs/")
	secondPlans := displayIndex(lines[1], "  plans/")
	if firstDocs != secondPlans {
		t.Fatalf("columns are not aligned using display width:\n%s", out)
	}
}

func TestRenderShowsChildErrorAndMoreCount(t *testing.T) {
	out := Render([]Node{{
		Name:      "parent",
		IsDir:     true,
		MoreCount: 2,
		Children:  []Node{{Name: "a", IsDir: true}},
	}}, Options{Width: 80})
	if !strings.Contains(out, "... 2 more") {
		t.Fatalf("missing more count:\n%s", out)
	}

	out = Render([]Node{{
		Name:  "locked",
		IsDir: true,
		Err:   errForTest("permission denied"),
	}}, Options{Width: 80})
	if !strings.Contains(out, "[error: permission denied]") {
		t.Fatalf("missing error marker:\n%s", out)
	}
}

type errForTest string

func (e errForTest) Error() string { return string(e) }

func displayIndex(s, substr string) int {
	idx := strings.Index(s, substr)
	if idx < 0 {
		return -1
	}
	return cellWidth(s[:idx])
}

func testBlock(index int, name string, height, width int) Block {
	lines := make([]string, height)
	for i := range lines {
		lines[i] = name
	}
	return Block{Lines: lines, Width: width, Index: index}
}

func rowIndexes(rows []layoutRow) [][]int {
	out := make([][]int, 0, len(rows))
	for _, row := range rows {
		indexes := make([]int, 0, len(row.cells))
		for _, cell := range row.cells {
			indexes = append(indexes, cell.block.Index)
		}
		out = append(out, indexes)
	}
	return out
}

func equalRows(a, b [][]int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if len(a[i]) != len(b[i]) {
			return false
		}
		for j := range a[i] {
			if a[i][j] != b[i][j] {
				return false
			}
		}
	}
	return true
}

func rowWidth(row layoutRow) int {
	return row.width
}
