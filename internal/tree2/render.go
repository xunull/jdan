package tree2

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/mattn/go-runewidth"
)

const (
	columnGap    = 4
	minColumnGap = 2
)

var widthCondition = &runewidth.Condition{
	EastAsianWidth:     true,
	StrictEmojiNeutral: true,
}

type Block struct {
	Lines []string
	Width int
	Index int
}

type layout struct {
	rows      []layoutRow
	columns   int
	slotWidth int
}

type layoutCell struct {
	block Block
	start int
	span  int
}

type layoutRow struct {
	cells  []layoutCell
	width  int
	height int
	slots  int
}

func Render(nodes []Node, opts Options) string {
	if len(nodes) == 0 {
		return ""
	}

	blocks := makeBlocks(nodes)
	width := opts.Width
	if width <= 0 {
		width = DefaultWidth
	}

	lay := layoutBlocks(blocks, width, opts.Columns)

	var out strings.Builder
	for i, row := range lay.rows {
		writeLayoutRow(&out, row, lay.slotWidth)
		if i < len(lay.rows)-1 {
			out.WriteByte('\n')
		}
	}

	return strings.TrimRight(out.String(), "\n")
}

func InferColumns(blocks []Block, width int) int {
	if len(blocks) == 0 {
		return 1
	}
	if width <= 0 {
		width = DefaultWidth
	}

	best := 1
	for cols := 1; cols <= len(blocks); cols++ {
		if widestRow(blocks, cols) <= width {
			best = cols
		}
	}
	return best
}

func makeBlocks(nodes []Node) []Block {
	blocks := make([]Block, 0, len(nodes))
	for _, node := range nodes {
		lines := []string{displayName(node)}
		if node.Err != nil {
			lines = append(lines, "  [error: "+node.Err.Error()+"]")
		} else {
			for _, child := range node.Children {
				lines = append(lines, "  "+displayName(child))
			}
			if node.MoreCount > 0 {
				lines = append(lines, fmt.Sprintf("  ... %d more", node.MoreCount))
			}
		}

		width := 0
		for _, line := range lines {
			if w := cellWidth(line); w > width {
				width = w
			}
		}
		blocks = append(blocks, Block{Lines: lines, Width: width, Index: len(blocks)})
	}
	return blocks
}

func layoutBlocks(blocks []Block, width, forcedCols int) layout {
	if len(blocks) == 0 {
		return layout{}
	}
	if width <= 0 {
		width = DefaultWidth
	}
	cols := forcedCols
	if cols <= 0 {
		cols = InferColumns(blocks, width)
	}
	if cols < 1 {
		cols = 1
	}
	if cols > len(blocks) {
		cols = len(blocks)
	}
	slotWidth := columnSlotWidth(width, cols)

	if forcedCols > 0 {
		rows := make([]layoutRow, 0, (len(blocks)+cols-1)/cols)
		var row layoutRow
		for _, block := range blocks {
			span := blockSpan(block, slotWidth, cols)
			if len(row.cells) > 0 && row.slots+span > cols {
				rows = append(rows, row)
				row = layoutRow{}
			}
			addCell(&row, block, span, slotWidth)
		}
		if len(row.cells) > 0 {
			rows = append(rows, row)
		}
		return layout{rows: rows, columns: cols, slotWidth: slotWidth}
	}

	pending := append([]Block(nil), blocks...)
	sort.SliceStable(pending, func(i, j int) bool {
		if blockHeight(pending[i]) != blockHeight(pending[j]) {
			return blockHeight(pending[i]) > blockHeight(pending[j])
		}
		return pending[i].Index < pending[j].Index
	})

	var states []layoutRow
	for _, block := range pending {
		span := blockSpan(block, slotWidth, cols)
		best := -1
		bestCost := math.MaxInt
		for i := range states {
			state := states[i]
			if state.slots+span > cols {
				continue
			}

			cost := rowPlacementCost(state, block)
			if cost < bestCost {
				best = i
				bestCost = cost
			}
		}

		if best == -1 {
			var row layoutRow
			addCell(&row, block, span, slotWidth)
			states = append(states, row)
			continue
		}

		addCell(&states[best], block, span, slotWidth)
	}

	rows := make([]layoutRow, 0, len(states))
	for _, state := range states {
		sort.Slice(state.cells, func(i, j int) bool {
			return state.cells[i].block.Index < state.cells[j].block.Index
		})
		reflowRow(&state, slotWidth)
		rows = append(rows, state)
	}
	sort.Slice(rows, func(i, j int) bool {
		return rows[i].cells[0].block.Index < rows[j].cells[0].block.Index
	})
	return layout{rows: rows, columns: cols, slotWidth: slotWidth}
}

func rowPlacementCost(state layoutRow, block Block) int {
	height := blockHeight(block)
	if height > state.height {
		return (height - state.height) * (len(state.cells) + 1)
	}
	return state.height - height
}

func blockHeight(block Block) int {
	return len(block.Lines)
}

func writeLayoutRow(out *strings.Builder, row layoutRow, slotWidth int) {
	for lineIdx := 0; lineIdx < row.height; lineIdx++ {
		cursor := 0
		for _, cell := range row.cells {
			start := cell.start * (slotWidth + columnGap)
			if start > cursor {
				out.WriteString(strings.Repeat(" ", start-cursor))
				cursor = start
			}
			line := ""
			if lineIdx < len(cell.block.Lines) {
				line = cell.block.Lines[lineIdx]
			}
			out.WriteString(line)
			cursor += cellWidth(line)
		}
		out.WriteByte('\n')
	}
}

func cellWidth(s string) int {
	return widthCondition.StringWidth(s)
}

func widestRow(blocks []Block, cols int) int {
	widest := 0
	for rowStart := 0; rowStart < len(blocks); rowStart += cols {
		rowEnd := rowStart + cols
		if rowEnd > len(blocks) {
			rowEnd = len(blocks)
		}

		width := 0
		for i, block := range blocks[rowStart:rowEnd] {
			width += block.Width
			if i < rowEnd-rowStart-1 {
				width += columnGap
			}
		}
		if width > widest {
			widest = width
		}
	}
	return widest
}

func columnSlotWidth(width, cols int) int {
	if cols <= 1 {
		return width
	}
	slotWidth := (width - (cols-1)*columnGap) / cols
	if slotWidth < 1 {
		return 1
	}
	return slotWidth
}

func blockSpan(block Block, slotWidth, cols int) int {
	for span := 1; span <= cols; span++ {
		if block.Width <= spanCapacity(span, slotWidth, cols) {
			return span
		}
	}
	return cols
}

func spanCapacity(span, slotWidth, cols int) int {
	if span >= cols {
		return span*slotWidth + (span-1)*columnGap
	}
	return span*(slotWidth+columnGap) - minColumnGap
}

func addCell(row *layoutRow, block Block, span, slotWidth int) {
	cell := layoutCell{
		block: block,
		start: row.slots,
		span:  span,
	}
	row.cells = append(row.cells, cell)
	row.slots += span
	row.width = row.slots*slotWidth + max(0, row.slots-1)*columnGap
	if h := blockHeight(block); h > row.height {
		row.height = h
	}
}

func reflowRow(row *layoutRow, slotWidth int) {
	row.slots = 0
	row.width = 0
	row.height = 0
	for i := range row.cells {
		row.cells[i].start = row.slots
		row.slots += row.cells[i].span
		row.width = row.slots*slotWidth + max(0, row.slots-1)*columnGap
		if h := blockHeight(row.cells[i].block); h > row.height {
			row.height = h
		}
	}
}

func displayName(node Node) string {
	if node.IsDir {
		return node.Name + "/"
	}
	return node.Name
}
