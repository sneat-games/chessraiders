// Copyright 2026 Sneat.app

// Package botvalue is the native, immutable, Go-backed Starlark value bridge
// for the Chess Raiders bot protocol: the layer that lets a host hand an
// observation to a script WITHOUT a JSON hop and WITHOUT a single dictionary.
//
// WHY THIS EXISTS (measured, not asserted). One production decide round trip
// allocates ~18,170 objects. Attributed by allocation site through the
// interpreter (go.starlark.net@v0.0.0-20260708150628 interp.go):
//
//	interp.go:351  slices.Clone(positional)   5,683/op  32.5%  builtin-call args
//	interp.go:62   make([]Value, nspace)      3,213/op  18.4%  Starlark call frame
//	interp.go:442  getAttr -> BindReceiver    1,918/op  11.0%  bound method (.get/.keys)
//	json.decode                               1,514/op   8.7%  the JSON hop
//	interp.go:431  getIndex -> String.Index   1,333/op   7.6%  square[0] slicing
//	interp.go:460  new(Dict)                    547/op   3.1%  dict literals
//	interp.go:547  NewList                      596/op   3.4%  list literals
//
// The JSON hop is 8.7%. INTERPRETED CALL OVERHEAD IS 61.9%: every call to a
// builtin clones its argument slice, every call to a Starlark function
// allocates a frame, and every bound method (`d.get(k, default)`) allocates
// a *Builtin on top. A script that must call `guarded_count(cell)` to learn
// a fact pays two allocations before the fact is even looked up.
//
// So the win is NOT "skip the marshal". The win is that a Go-backed value
// with attribute access lets the host PRE-ANSWER the questions the script
// currently computes, and hand each answer back as a pre-boxed, interned
// starlark.Value: `cell.guarded_count` costs zero allocations where
// `guarded_count(cell)` costs four (frame + clone + BindReceiver + clone).
//
// THREE PROPERTIES ARE STRUCTURAL, NOT POLICY, because bot scripts will be
// user-contributed and must be assumed hostile:
//
//   - IMMUTABLE BY CONSTRUCTION. No type in this package implements
//     starlark.HasSetField, starlark.HasSetKey or starlark.HasSetIndex.
//     Mutation is not rejected at runtime, it is unrepresentable: the
//     interpreter's setField/setIndex (eval.go) look for exactly those
//     interfaces and find nothing to call. Freeze() is a no-op because
//     there is no mutable state to freeze. This is strictly stronger than
//     freezing a *starlark.Dict, which still HAS mutation methods and
//     relies on somebody having remembered to freeze it.
//
//   - NO DICTIONARIES. Structured rows expose fields as ATTRIBUTES
//     (`unit.square`), resolved by an integer index into a fixed Schema —
//     no hash table, no per-decision key strings. Bulk collections are flat
//     indexable arrays (`board[rank][file]`, `obs.units[i]`). A dictionary
//     appears nowhere in the observation.
//
//   - LAZY AND INVALIDATED. A row reads through to live host memory on
//     demand, so a field no script touches costs nothing. Because that
//     memory is only valid for the decision that produced it, every value
//     carries the Session generation it was minted at and re-checks it on
//     every access: a reference squirrelled away and read after the
//     decision ended fails loudly (ErrStaleObservation) instead of quietly
//     reporting whatever the engine now holds.
//
// The interning tables below are what make attribute access free. A
// starlark.String is a two-word value, so boxing one into a starlark.Value
// interface ALWAYS heap-allocates; likewise starlark.Int on GOARCH=wasm,
// whose intImpl is a two-word union rather than the single unsafe.Pointer
// the darwin/arm64 build gets (go.starlark.net int_generic.go vs
// int_posix64.go). Pre-boxing every value in the protocol's closed
// vocabulary once, at init, turns every one of those allocations into a
// slice index.
package botvalue

import (
	"fmt"

	"go.starlark.net/starlark"
)

// squareNames is the protocol's 64 square names in file-major engine order
// (a1..a8, b1..b8, ... h1..h8) — the SAME order the host's own square index
// uses, so a host converts an index to a name with one slice index and a
// script never parses "e4" into a file and a rank at all.
var squareNames = buildSquareNames()

func buildSquareNames() [64]string {
	var names [64]string
	for file := 0; file < 8; file++ {
		for rank := 0; rank < 8; rank++ {
			names[file*8+rank] = string(rune('a'+file)) + string(rune('1'+rank))
		}
	}
	return names
}

// squareValues is squareNames pre-boxed. Every one of these is immutable and
// shared by every match on the process, which is safe for the reason
// go.starlark.net's own String.Freeze() is a no-op: a Starlark string has no
// mutable state at all.
var squareValues = func() [64]starlark.Value {
	var values [64]starlark.Value
	for i, name := range squareNames {
		values[i] = starlark.String(name)
	}
	return values
}()

// smallInts pre-boxes 0..1023, which covers every count, index, unit id,
// morale value, integrity value and millisecond-free scalar in the
// protocol's hot path. Out-of-range values fall through to starlark.MakeInt64
// (free on darwin/arm64, one allocation on wasm).
const smallIntCeiling = 1024

var smallInts = func() [smallIntCeiling]starlark.Value {
	var values [smallIntCeiling]starlark.Value
	for i := range values {
		values[i] = starlark.MakeInt(i)
	}
	return values
}()

// Square returns the interned Starlark value for a 0..63 square index. It
// never allocates.
func Square(index int) starlark.Value {
	if index < 0 || index >= 64 {
		return starlark.None
	}
	return squareValues[index]
}

// SquareName is Square's Go-side twin, for a host building an error message
// or a returned intent.
func SquareName(index int) string {
	if index < 0 || index >= 64 {
		return ""
	}
	return squareNames[index]
}

// Int returns an interned Starlark int for small non-negative values and
// falls back to MakeInt64 otherwise.
func Int(v int64) starlark.Value {
	if v >= 0 && v < smallIntCeiling {
		return smallInts[v]
	}
	return starlark.MakeInt64(v)
}

// Bool returns one of the two package-level boolean values. starlark.Bool is
// a one-word type so this never allocated anyway; it exists so a Vocabulary
// accessor can stay uniform.
func Bool(v bool) starlark.Value { return starlark.Bool(v) }

// Vocabulary is one closed set of enum strings — ranks, professions, grades,
// lifecycles, sides — pre-boxed once. A host hands the bridge an integer
// CODE, never a string, so an enum-valued attribute is a slice index rather
// than a string box.
//
// A Vocabulary is built once at package or match init and never mutated, so
// it is safe to share across every concurrently-running match.
type Vocabulary struct {
	names  []string
	values []starlark.Value
}

// NewVocabulary builds a closed enum table. The zero code is conventionally
// the empty/absent member, which a host uses for "no profession".
func NewVocabulary(names ...string) *Vocabulary {
	v := &Vocabulary{names: append([]string(nil), names...)}
	v.values = make([]starlark.Value, len(v.names))
	for i, name := range v.names {
		if name == "" {
			v.values[i] = starlark.None
			continue
		}
		v.values[i] = starlark.String(name)
	}
	return v
}

// Value returns the pre-boxed member for one code, or None when the code is
// out of range. It never allocates.
func (v *Vocabulary) Value(code int) starlark.Value {
	if v == nil || code < 0 || code >= len(v.values) {
		return starlark.None
	}
	return v.values[code]
}

// Name returns the Go string for one code.
func (v *Vocabulary) Name(code int) string {
	if v == nil || code < 0 || code >= len(v.names) {
		return ""
	}
	return v.names[code]
}

// Len reports how many members the vocabulary declares.
func (v *Vocabulary) Len() int {
	if v == nil {
		return 0
	}
	return len(v.names)
}

// ErrStaleObservation is what every accessor on a value whose Session has
// already ended returns. It is deliberately a distinct, matchable error
// rather than a generic attribute failure: a script that stashes a
// reference and reads it on a later decision must produce an unmistakable
// diagnosis, not "None".
type ErrStaleObservation struct {
	Type       string
	Attr       string
	MintedAt   uint64
	CurrentGen uint64
}

func (e *ErrStaleObservation) Error() string {
	return fmt.Sprintf(
		"chess-raiders bot protocol: %s.%s was read after its decision ended "+
			"(value belongs to decision %d, the host is now at %d). "+
			"An observation value is valid only for the decide() call that "+
			"produced it and must never be carried across decisions.",
		e.Type, e.Attr, e.MintedAt, e.CurrentGen)
}

// Value is an ALIAS for starlark.Value. It exists so a host module that must
// not import go.starlark.net (server-go/importboundary sweeps import paths
// across the private engine module) can still name, hold and pass the values
// this package mints. Nothing about the alias weakens the boundary: the host
// can name the interface but cannot reach any go.starlark.net constructor,
// so every value it passes came out of this package's own closed schema.
type Value = starlark.Value

// None is the shared Starlark none value, re-exported for the same reason.
var None = starlark.None

// AsFloat reads a numeric result (int or float) back on the host side.
func AsFloat(v Value) (float64, bool) { return starlark.AsFloat(v) }

// AsInt reads an integer result back on the host side.
func AsInt(v Value) (int64, bool) {
	i, ok := v.(starlark.Int)
	if !ok {
		return 0, false
	}
	return i.Int64()
}

// AsString reads a string result back on the host side.
func AsString(v Value) (string, bool) {
	s, ok := v.(starlark.String)
	return string(s), ok
}

// TupleLen and TupleAt read a multi-value return (Starlark's `return a, b, c`
// is a tuple) without the host naming a go.starlark.net type.
func TupleLen(v Value) (int, bool) {
	t, ok := v.(starlark.Tuple)
	return len(t), ok
}

func TupleAt(v Value, i int) (Value, bool) {
	t, ok := v.(starlark.Tuple)
	if !ok || i < 0 || i >= len(t) {
		return None, false
	}
	return t[i], true
}
