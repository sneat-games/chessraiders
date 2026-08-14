// Copyright 2026 Sneat.app

package botvalue

import (
	"fmt"
	"math/bits"

	"go.starlark.net/starlark"
)

// UnitList is a piece relation as a script sees it: an ordinary array of the
// units involved. `unit.defenders` is a list of pieces, iterated, indexed and
// tested with `in` exactly like any other sequence, and it needs no
// explanation beyond that sentence.
//
// THE ENGINE STILL CARRIES A uint32 BITSET. Only the script-facing shape
// changed. The host keeps cheap set operations, the small struct and the fog
// property below; what a third-party author meets is a list. That split is the
// whole design: a custom set type with operator overloads would have been
// protocol surface every one of thousands of competitors had to learn — that
// `-` means set difference and Go's `&^` does not exist — to buy an
// optimisation they did not ask for. Legibility wins on a public protocol.
//
// THE BIT MAPPING IS PART OF THE PROTOCOL even though scripts never see it,
// because the engine, the wire and any future non-Starlark runtime all share
// it:
//
//	bit i  <->  UnitID i+1        i is the UnitIndex, 0..31
//	UnitID 1..32 occupy bits 0..31
//	NoUnitID (0) has NO bit, so an absent unit is unrepresentable
//	width is uint32, welding the 32-unit cap into the public protocol
//
// That third line is why the form is safe rather than merely compact: a
// relation can never name a square whose occupant the viewer is not entitled
// to know about, because a mask names units and only units the projection
// already resolved.
//
// NOTHING IS EVER MATERIALIZED. The list is a VIEW over the mask, not a slice
// built from it: iteration walks set bits and yields rows, indexing walks to
// the i-th set bit, `.count` is one population count and `in` is one bit test.
// A relation no script reads costs nothing at all, and one it reads costs no
// backing array either — which is stronger than the lazy materialization this
// had to guarantee, and it falls out of keeping the mask as the backing store.
//
// ELEMENTS ARE UNIT ROWS, NOT IDS. An author reading `defenders` wants to ask
// about those pieces — `.rank`, `.square`, `.attackers.count` — so the list
// yields the rows themselves and no lookup table is exposed to scripts at all.
// The by-UnitIndex resolution table survives only as this type's private
// `rows` field.
//
// The four relations and their v1 wire names are in relations.go.
//
// Like every other type here it implements no mutation interface, so a
// relation cannot be edited by the script that reads it. It also deliberately
// implements NO operator overloads: there is no `|`, no `-`, and no `.bits`
// escape hatch. Set algebra in script is an ordinary comprehension, which
// costs more than an operator would have — see relation_cost_test.go for the
// measured price of that choice.
type UnitList struct {
	bits uint32
	// rows resolves a bit index back to a unit. It is a fixed by-UnitIndex
	// table, NOT the by-square iteration view: a relation is keyed by identity
	// while rows are ordered by square and may include ghosts. Keeping it
	// unexported retires that hazard from the protocol rather than documenting
	// it.
	rows *UnitIdentityView
	sess *Session
	gen  uint64
}

var (
	_ starlark.Value     = (*UnitList)(nil)
	_ starlark.HasAttrs  = (*UnitList)(nil)
	_ starlark.Indexable = (*UnitList)(nil)
	_ starlark.Sequence  = (*UnitList)(nil)
	_ starlark.Container = (*UnitList)(nil)
)

func (l *UnitList) String() string        { return fmt.Sprintf("<units %d>", l.Len()) }
func (l *UnitList) Type() string          { return "units" }
func (l *UnitList) Freeze()               {}
func (l *UnitList) Truth() starlark.Bool  { return starlark.Bool(l.visibleBits() != 0) }
func (l *UnitList) Hash() (uint32, error) { return 0, fmt.Errorf("unhashable type: units") }

// visibleBits is a belt-and-suspenders fog boundary. The host should only set
// relation bits for identities in the current projection, but this fixed
// 32-entry filter makes a malformed host answer safely invisible rather than
// yielding a stale or hidden row.
func (l *UnitList) visibleBits() uint32 {
	if l.rows == nil {
		return 0
	}
	return l.bits & l.rows.presentMask()
}

// Len is a population count over currently exposed identities, so
// len(unit.defenders) never materializes anything or leaks a fog-hidden unit.
func (l *UnitList) Len() int { return bits.OnesCount32(l.visibleBits()) }

func (l *UnitList) AttrNames() []string { return []string{"count"} }

// Attr answers `.count` without materializing the list. This is deliberate and
// load-bearing: a script asking only "how many defenders" must never build the
// array, and the measured cost of `.count` is 0.00 allocations on BOTH the
// server and the browser (relation_cost_test.go). It is kept as its own
// attribute rather than folded into len() so that intent reads clearly at the
// call site.
func (l *UnitList) Attr(name string) (starlark.Value, error) {
	if name == "count" {
		return Int(int64(l.Len())), nil
	}
	return nil, nil
}

// Index walks to the i-th set bit. No backing slice is built.
func (l *UnitList) Index(i int) starlark.Value {
	remaining := l.visibleBits()
	for ; i > 0; i-- {
		remaining &= remaining - 1
	}
	index := bits.TrailingZeros32(remaining)
	if l.rows == nil || index >= len(l.rows.rows) || l.rows.rows[index] == nil {
		return starlark.None
	}
	return l.rows.rows[index]
}

func (l *UnitList) Iterate() starlark.Iterator {
	return &unitListIter{list: l, remaining: l.visibleBits()}
}

type unitListIter struct {
	list      *UnitList
	remaining uint32
}

func (it *unitListIter) Next(p *starlark.Value) bool {
	if it.remaining == 0 {
		return false
	}
	index := bits.TrailingZeros32(it.remaining)
	it.remaining &= it.remaining - 1
	if it.list.rows == nil || index >= len(it.list.rows.rows) || it.list.rows.rows[index] == nil {
		return false
	}
	*p = it.list.rows.rows[index]
	return true
}

func (it *unitListIter) Done() {}

// Has implements `unit in cell.defenders`. It stays ONE BIT TEST despite the
// list shape, because the mask is still the backing store — so membership did
// not become linear when the script-facing type became an array. Measured at
// 0.00 allocations on both targets.
func (l *UnitList) Has(y starlark.Value) (bool, error) {
	row, ok := y.(*Row)
	if !ok {
		return false, nil
	}
	if l.rows == nil {
		return false, nil
	}
	for i, candidate := range l.rows.rows {
		if candidate == row {
			return l.bits&(1<<uint(i)) != 0, nil
		}
	}
	return false, nil
}

// MaskSource is the host side of a relation field: it answers one row's one
// relation as a raw uint32, naming no Starlark type. The engine's
// representation is unchanged by the script-facing shape.
type MaskSource interface {
	Mask(row, field int) uint32
}
