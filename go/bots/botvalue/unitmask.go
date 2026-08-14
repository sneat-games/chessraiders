// Copyright 2026 Sneat.app

package botvalue

import (
	"fmt"
	"math/bits"

	"go.starlark.net/starlark"
	"go.starlark.net/syntax"
)

// UnitMask is a piece relation carried as a 32-bit set over unit identity.
//
// THE BIT MAPPING IS PART OF THE PROTOCOL, not an implementation note:
//
//	bit i  <->  UnitID i+1        i is the UnitIndex, 0..31
//	UnitID 1..32 occupy bits 0..31
//	NoUnitID (0) has NO bit, so an absent unit is unrepresentable in a mask
//
// That last line is why the mask form is safe: a relation can never name a
// square whose occupant the viewer is not entitled to know about, because a
// mask names units and only units the projection already resolved.
//
// WHY THIS IS A VALUE TYPE AND NOT A BARE INT. The founder's decision is that
// the wire and the host both carry a uint32; the question this type answers is
// what a SCRIPT holds. Exposing the raw integer would have handed every author
// two traps, both measured rather than assumed:
//
//   - NO POPCOUNT. Starlark has &, |, ^ and << but no bit-count builtin, so
//     deriving a count from a raw mask means a 32-iteration loop — exactly the
//     interpreted overhead this whole protocol exists to remove.
//   - THE BIT-31 CLIFF. go.starlark.net represents int32-range values in a
//     pointer-sized union (int_posix64.go) and anything wider as a *big.Int, so
//     `1 << 31` — the last bit of a uint32 mask — falls out of the cheap
//     representation. A measured 32-iteration shift walk costs 154 allocs/op
//     before doing any work at all, which is worse than the string arrays the
//     masks replace.
//
// So the mask is a Go-backed value that answers all three questions natively:
// `.count` is one bits.OnesCount32, iteration walks the set bits host-side and
// yields unit rows directly (no shifts, no bit-index arithmetic in script), and
// &, | and &^ compose masks through HasBinary. `.bits` is still there for an
// author who genuinely wants the integer.
//
// Like every other type in this package it implements no mutation interface at
// all, so a relation cannot be edited by the script that reads it.
type UnitMask struct {
	bits uint32
	// units resolves a bit index back to the unit that owns it. It is the
	// by-UnitIndex array, NOT the by-square iteration array: a mask is keyed
	// by identity, and unit rows are ordered by square, so the two are
	// different views and conflating them would silently return the wrong
	// unit.
	units *RowArray
	sess  *Session
	gen   uint64
}

var (
	_ starlark.Value     = (*UnitMask)(nil)
	_ starlark.HasAttrs  = (*UnitMask)(nil)
	_ starlark.Sequence  = (*UnitMask)(nil)
	_ starlark.Container = (*UnitMask)(nil)
	_ starlark.HasBinary = (*UnitMask)(nil)
)

func (m *UnitMask) String() string        { return fmt.Sprintf("<units %08x>", m.bits) }
func (m *UnitMask) Type() string          { return "unitset" }
func (m *UnitMask) Freeze()               {}
func (m *UnitMask) Truth() starlark.Bool  { return starlark.Bool(m.bits != 0) }
func (m *UnitMask) Hash() (uint32, error) { return m.bits, nil }

// Len is the population count, so len(unit.guarded_by) agrees with
// unit.guarded_by.count and neither costs a loop.
func (m *UnitMask) Len() int { return bits.OnesCount32(m.bits) }

func (m *UnitMask) AttrNames() []string { return []string{"bits", "count"} }

func (m *UnitMask) Attr(name string) (starlark.Value, error) {
	switch name {
	case "count":
		return Int(int64(bits.OnesCount32(m.bits))), nil
	case "bits":
		return Int(int64(m.bits)), nil
	}
	return nil, nil
}

// Iterate walks the set bits in ascending UnitID order and yields the UNIT
// ROWS themselves, so a script never performs a shift or resolves a bit index
// by hand.
func (m *UnitMask) Iterate() starlark.Iterator {
	return &maskIter{mask: m, remaining: m.bits}
}

type maskIter struct {
	mask      *UnitMask
	remaining uint32
}

func (it *maskIter) Next(p *starlark.Value) bool {
	if it.remaining == 0 {
		return false
	}
	index := bits.TrailingZeros32(it.remaining)
	it.remaining &= it.remaining - 1
	if it.mask.units == nil || index >= len(it.mask.units.rows) {
		return false
	}
	*p = it.mask.units.rows[index]
	return true
}

func (it *maskIter) Done() {}

// Has implements `unit in cell.guarded_by` — one bit test.
func (m *UnitMask) Has(y starlark.Value) (bool, error) {
	row, ok := y.(*Row)
	if !ok {
		return false, nil
	}
	if row.index < 0 || row.index > 31 {
		return false, nil
	}
	return m.bits&(1<<uint(row.index)) != 0, nil
}

// Binary composes masks with the ordinary set operators, so the eight relation
// DELTAS a script might want are one operator each and no host field:
//
//	gained = candidate.guards &^ unit.guards
//	lost   = unit.guards &^ candidate.guards
//	both   = unit.guards & candidate.guards
//
// The result is a new mask, so `.count` works on it directly.
func (m *UnitMask) Binary(op syntax.Token, y starlark.Value, side starlark.Side) (starlark.Value, error) {
	other, ok := y.(*UnitMask)
	if !ok {
		return nil, nil // let the interpreter report the type error
	}
	var result uint32
	switch op {
	case syntax.AMP:
		result = m.bits & other.bits
	case syntax.PIPE:
		result = m.bits | other.bits
	case syntax.CIRCUMFLEX:
		result = m.bits ^ other.bits
	case syntax.MINUS:
		if side == starlark.Left {
			result = m.bits &^ other.bits
		} else {
			result = other.bits &^ m.bits
		}
	default:
		return nil, nil
	}
	return &UnitMask{bits: result, units: m.units, sess: m.sess, gen: m.gen}, nil
}

// MaskSource is the host side of a relation field: it answers one row's one
// relation as a raw uint32, naming no Starlark type.
type MaskSource interface {
	Mask(row, field int) uint32
}
