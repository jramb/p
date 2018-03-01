package table

/**
*
* 2017 by J Ramb
*
**/

import (
	"fmt"
	"strconv"
	"strings"
)

type Alignment int

const (
	Left Alignment = iota
	Right
	Center
)

type Cell struct {
	Value string
	Align Alignment
}

type Row []Cell

type Table []Row

func NewTable() Table {
	return make(Table, 0)
}

func NewRow() Row {
	return make(Row, 0)
}
func (tab Table) Add(row Row) Table {
	return append(tab, row)
}

func (tab Table) AddDivider() Table {
	return append(tab, make(Row, 0))
}

func (row Row) Add(cell Cell) Row {
	return append(row, cell)
}

func (tab Table) colSizes() []int {
	w := 0
	for _, r := range tab {
		if len(r) > w {
			w = len(r)
		}
	}
	sizes := make([]int, w)
	for _, r := range tab {
		for x, c := range r {
			s := len(c.Value)
			if s > sizes[x] {
				sizes[x] = s
			}
		}
	}
	return sizes
}

func (c Cell) printCell(size int, ext string) {
	var fmtstr string
	str := c.Value
	cLength := len(str)
	if cLength > size {
		cLength = size
		str = string(str[:size-3]) + "..."
	}
	switch c.Align {
	case Left:
		fmtstr = " %-" + strconv.Itoa(size) + "s "
	case Right:
		fmtstr = " %" + strconv.Itoa(size) + "s "
	case Center:
		spc := (size - cLength) / 2
		fmtstr = " " + strings.Repeat(" ", spc) + "%-s" +
			strings.Repeat(" ", size-cLength-spc) + " "
	}
	fmt.Printf(fmtstr+ext, str)
}

func printDivider(sizes []int) {
	l := len(sizes) - 1
	for n, s := range sizes {
		fmt.Print(strings.Repeat("-", s+2))
		if n < l {
			fmt.Print("+")
		}
	}
}

func (tab Table) Print(orgmode bool) {
	sizes := tab.colSizes()
	for _, r := range tab {
		if orgmode {
			fmt.Print("|")
		}
		if len(r) == 0 {
			printDivider(sizes)
		} else {
			for x, c := range r {
				if x < len(r)-1 {
					c.printCell(sizes[x], "|")
				} else {
					c.printCell(sizes[x], "")

				}
				// fmt.Printf(formats[x], c.Value)
			}
		}
		if orgmode {
			fmt.Print("|")
		}
		fmt.Println()
	}
}
