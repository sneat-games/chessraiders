// Copyright 2026 Sneat.app

package botvalue

import (
	"fmt"
	"math/bits"

	"go.starlark.net/starlark"
)

// SquareMaskList is an immutable Starlark sequence over a fixed-board uint64
// set. It is the native form for legal destinations: index and iteration walk
// set bits directly, so no []Square, []string, map, or conversion buffer is
// constructed on a decision's hot path.
type SquareMaskList struct {
	bits uint64
}

var (
	_ starlark.Value     = (*SquareMaskList)(nil)
	_ starlark.Indexable = (*SquareMaskList)(nil)
	_ starlark.Sequence  = (*SquareMaskList)(nil)
	_ starlark.Container = (*SquareMaskList)(nil)
)

func (l *SquareMaskList) String() string        { return fmt.Sprintf("<squares %d>", l.Len()) }
func (l *SquareMaskList) Type() string          { return "squares" }
func (l *SquareMaskList) Freeze()               {}
func (l *SquareMaskList) Truth() starlark.Bool  { return starlark.Bool(l.bits != 0) }
func (l *SquareMaskList) Hash() (uint32, error) { return 0, fmt.Errorf("unhashable type: squares") }
func (l *SquareMaskList) Len() int              { return bits.OnesCount64(l.bits) }

func (l *SquareMaskList) Index(i int) starlark.Value {
	remaining := l.bits
	for ; i > 0; i-- {
		remaining &= remaining - 1
	}
	if remaining == 0 {
		return starlark.None
	}
	return Square(bits.TrailingZeros64(remaining))
}

func (l *SquareMaskList) Iterate() starlark.Iterator {
	return &squareMaskIter{remaining: l.bits}
}

func (l *SquareMaskList) Has(y starlark.Value) (bool, error) {
	name, ok := y.(starlark.String)
	if !ok {
		return false, nil
	}
	index, ok := SquareIndex(string(name))
	if !ok {
		return false, nil
	}
	return l.bits&(uint64(1)<<uint(index)) != 0, nil
}

type squareMaskIter struct{ remaining uint64 }

func (it *squareMaskIter) Next(p *starlark.Value) bool {
	if it.remaining == 0 {
		return false
	}
	index := bits.TrailingZeros64(it.remaining)
	it.remaining &= it.remaining - 1
	*p = Square(index)
	return true
}

func (*squareMaskIter) Done() {}
