// Copyright 2026 Sneat.app

package botvalue

import (
	"fmt"
	"sync/atomic"

	"go.starlark.net/starlark"
)

// ScalarKind tags one host-supplied field value. A Scalar is returned BY
// VALUE from the host, so answering a field question costs no allocation on
// the host side either — the bridge does all boxing, out of its interned
// tables.
type ScalarKind uint8

const (
	KindNone ScalarKind = iota
	KindBool
	KindInt
	// KindEnum indexes the field's own Vocabulary (declared in the Schema).
	KindEnum
	// KindSquare indexes the 64 interned square names.
	KindSquare
	KindFloat
)

// Scalar is one field answer. Deliberately a flat struct with no pointer and
// no interface: a host fills it from engine memory and returns it without
// touching the heap.
type Scalar struct {
	Kind  ScalarKind
	Int   int64
	Float float64
}

// NoneScalar and the constructors below keep host code readable.
func NoneScalar() Scalar { return Scalar{Kind: KindNone} }
func BoolScalar(v bool) Scalar {
	i := int64(0)
	if v {
		i = 1
	}
	return Scalar{Kind: KindBool, Int: i}
}
func IntScalar(v int64) Scalar     { return Scalar{Kind: KindInt, Int: v} }
func EnumScalar(code int) Scalar   { return Scalar{Kind: KindEnum, Int: int64(code)} }
func SquareScalar(i int) Scalar    { return Scalar{Kind: KindSquare, Int: int64(i)} }
func FloatScalar(v float64) Scalar { return Scalar{Kind: KindFloat, Float: v} }

// Source is what the ENGINE implements to back one row kind. It names no
// Starlark type, so the private engine module can implement it without
// importing go.starlark.net at all — which is exactly what
// server-go/importboundary requires of every package in that module.
//
// Field is called LAZILY: only for attributes a script actually reads, and
// only at the moment it reads them. A per-unit fact no script consults is
// never computed.
type Source interface {
	// Len is how many rows this source currently holds.
	Len() int
	// Field answers one row's one field. field is the Schema field index,
	// never a name, so the host switches on a small integer.
	Field(row, field int) Scalar
	// Squares answers a repeated-square field (guardedBy, threatens,
	// nextPossibleMoves...). Returning a slice the host already owns is
	// fine and expected: the bridge never mutates it and never lets a
	// script reach it.
	Squares(row, field int) []uint8
	// Ints answers a repeated-integer field the same way.
	Ints(row, field int) []int32
}

// FieldSpec declares one attribute of one row kind: its script-visible name
// and, for enum fields, the closed vocabulary its codes index.
//
// THE SCHEMA IS THE PRIVACY BOUNDARY. Under the JSON wire, adding a field to
// the host's observation struct exposed it to every script automatically.
// Here, a field is visible to a script if and only if it is listed here, by
// hand, in the PUBLIC module — which makes widening the bot-visible surface
// a reviewable diff in the repository scripts are written against, rather
// than a side effect of an engine refactor.
type FieldSpec struct {
	Name  string
	Kind  ScalarKind
	Vocab *Vocabulary
	// Repeated marks a square-list field, answered by Source.Squares.
	Repeated bool
	// RepeatedInts marks an integer-list field, answered by Source.Ints.
	RepeatedInts bool
}

// Schema is one row kind's complete, closed attribute surface.
type Schema struct {
	typeName string
	fields   []FieldSpec
	byName   map[string]int
	names    []string
}

// NewSchema builds a row kind. It panics on a duplicate name because a
// duplicate is a protocol authoring bug that must never reach a running
// tournament.
func NewSchema(typeName string, fields ...FieldSpec) *Schema {
	s := &Schema{typeName: typeName, fields: fields, byName: make(map[string]int, len(fields))}
	s.names = make([]string, 0, len(fields))
	for i, f := range fields {
		if _, dup := s.byName[f.Name]; dup {
			panic("botvalue: duplicate field " + f.Name + " on " + typeName)
		}
		s.byName[f.Name] = i
		s.names = append(s.names, f.Name)
	}
	return s
}

// FieldIndex resolves a script-visible name to the integer a Source switches
// on. Hosts use it to keep their own switch statements in step with the
// schema without hard-coding ordinals.
func (s *Schema) FieldIndex(name string) (int, bool) {
	i, ok := s.byName[name]
	return i, ok
}

// TypeName is the type string a script sees in an error message.
func (s *Schema) TypeName() string { return s.typeName }

// Session bounds the lifetime of every value minted against it — ONE
// decision.
//
// generation is odd while a decision is running and even between decisions,
// so a single atomic word answers both "which decision was this minted for"
// and "is a decision running at all". A value records the odd generation it
// was minted at; any later read compares against the live word and fails if
// it has moved. Atomic rather than plain because the read can legitimately
// come from a different goroutine than the writer: a tournament runs matches
// in parallel and a stale reference is exactly the case this must diagnose
// rather than race on.
type Session struct {
	generation atomic.Uint64
}

// Begin opens a decision and returns its generation.
func (s *Session) Begin() uint64 { return s.generation.Add(1) }

// End closes the decision, invalidating every value minted during it.
func (s *Session) End() { s.generation.Add(1) }

// Generation reports the live generation.
func (s *Session) Generation() uint64 { return s.generation.Load() }

// Row is one Go-backed record with attribute access and NOTHING else.
//
// It implements starlark.Value and starlark.HasAttrs. It deliberately
// implements NEITHER starlark.HasSetField NOR starlark.HasSetKey NOR
// starlark.HasSetIndex: the interpreter's setField/setIndex look for exactly
// those interfaces (go.starlark.net eval.go), so `unit.square = "e4"` and
// `unit["square"] = ...` are not "rejected", they have nothing to dispatch
// to. A hostile script cannot mutate what has no mutator.
//
// It also does not implement starlark.Mapping, so `unit["square"]` fails —
// which is the loud, immediate failure a script written for the dict-shaped
// v1 protocol must get on its very first observation read.
type Row struct {
	schema *Schema
	source Source
	index  int
	sess   *Session
	gen    uint64
	// lists is a per-row, pre-allocated square-list object per repeated
	// field, so reading `unit.guarded_by` twice costs zero allocations and
	// zero garbage. Nil until the kind declares repeated fields.
	lists []SquareList
	ints  []IntList
}

var (
	_ starlark.Value    = (*Row)(nil)
	_ starlark.HasAttrs = (*Row)(nil)
)

func (r *Row) String() string {
	return fmt.Sprintf("<%s %d>", r.schema.typeName, r.index)
}
func (r *Row) Type() string { return r.schema.typeName }

// Freeze is a no-op: a Row has no mutable state to freeze. Immutability here
// is structural, not a mode the value can be put into.
func (r *Row) Freeze() {}

func (r *Row) Truth() starlark.Bool { return starlark.True }

// Hash refuses, exactly as a dict does, so a Row cannot become a set member
// or a dict key and thereby outlive its decision inside a container.
func (r *Row) Hash() (uint32, error) {
	return 0, fmt.Errorf("unhashable type: %s", r.schema.typeName)
}

// AttrNames is the closed field list, used by the interpreter only to spell-
// check a failed lookup.
func (r *Row) AttrNames() []string { return r.schema.names }

// Attr resolves one attribute. On the hot path — a scalar field of a live
// decision — it performs one map lookup, one atomic load, one host call and
// one slice index into an interned table, and allocates nothing at all.
func (r *Row) Attr(name string) (starlark.Value, error) {
	field, ok := r.schema.byName[name]
	if !ok {
		return nil, nil // interpreter turns this into a spelling-hinted error
	}
	if r.sess.generation.Load() != r.gen {
		return nil, &ErrStaleObservation{
			Type: r.schema.typeName, Attr: name,
			MintedAt: r.gen, CurrentGen: r.sess.generation.Load(),
		}
	}
	spec := &r.schema.fields[field]
	if spec.Repeated {
		list := &r.lists[repeatedSlot(r.schema, field, false)]
		list.squares = r.source.Squares(r.index, field)
		return list, nil
	}
	if spec.RepeatedInts {
		list := &r.ints[repeatedSlot(r.schema, field, true)]
		list.values = r.source.Ints(r.index, field)
		return list, nil
	}
	return boxScalar(r.source.Field(r.index, field), spec)
}

func boxScalar(s Scalar, spec *FieldSpec) (starlark.Value, error) {
	switch s.Kind {
	case KindNone:
		return starlark.None, nil
	case KindBool:
		return starlark.Bool(s.Int != 0), nil
	case KindInt:
		return Int(s.Int), nil
	case KindEnum:
		return spec.Vocab.Value(int(s.Int)), nil
	case KindSquare:
		return Square(int(s.Int)), nil
	case KindFloat:
		return starlark.Float(s.Float), nil
	}
	return starlark.None, nil
}

// repeatedSlot maps a field index to its dense slot in Row.lists/Row.ints.
func repeatedSlot(s *Schema, field int, ints bool) int {
	slot := 0
	for i := 0; i < field; i++ {
		if ints && s.fields[i].RepeatedInts {
			slot++
		} else if !ints && s.fields[i].Repeated {
			slot++
		}
	}
	return slot
}

// IntList is SquareList's integer twin: immutable, indexable, iterable, and
// implementing no mutation interface.
type IntList struct {
	values []int32
}

var (
	_ starlark.Value     = (*IntList)(nil)
	_ starlark.Indexable = (*IntList)(nil)
	_ starlark.Sequence  = (*IntList)(nil)
)

func (l *IntList) String() string             { return fmt.Sprintf("<ints %d>", len(l.values)) }
func (l *IntList) Type() string               { return "ints" }
func (l *IntList) Freeze()                    {}
func (l *IntList) Truth() starlark.Bool       { return starlark.Bool(len(l.values) > 0) }
func (l *IntList) Hash() (uint32, error)      { return 0, fmt.Errorf("unhashable type: ints") }
func (l *IntList) Len() int                   { return len(l.values) }
func (l *IntList) Index(i int) starlark.Value { return Int(int64(l.values[i])) }
func (l *IntList) Iterate() starlark.Iterator { return &intIter{list: l} }

type intIter struct {
	list *IntList
	i    int
}

func (it *intIter) Next(p *starlark.Value) bool {
	if it.i >= len(it.list.values) {
		return false
	}
	*p = Int(int64(it.list.values[it.i]))
	it.i++
	return true
}
func (it *intIter) Done() {}

// SquareList is an immutable, indexable, iterable list of square names backed
// by host-owned bytes. Like Row it implements no mutation interface at all,
// so `unit.guarded_by.append("e4")` finds no method and `unit.guarded_by[0] =
// x` finds no HasSetIndex.
type SquareList struct {
	squares []uint8
}

var (
	_ starlark.Value     = (*SquareList)(nil)
	_ starlark.Indexable = (*SquareList)(nil)
	_ starlark.Sequence  = (*SquareList)(nil)
)

func (l *SquareList) String() string        { return fmt.Sprintf("<squares %d>", len(l.squares)) }
func (l *SquareList) Type() string          { return "squares" }
func (l *SquareList) Freeze()               {}
func (l *SquareList) Truth() starlark.Bool  { return starlark.Bool(len(l.squares) > 0) }
func (l *SquareList) Hash() (uint32, error) { return 0, fmt.Errorf("unhashable type: squares") }
func (l *SquareList) Len() int              { return len(l.squares) }
func (l *SquareList) Index(i int) starlark.Value {
	return Square(int(l.squares[i]))
}
func (l *SquareList) Iterate() starlark.Iterator { return &squareIter{list: l} }

type squareIter struct {
	list *SquareList
	i    int
}

func (it *squareIter) Next(p *starlark.Value) bool {
	if it.i >= len(it.list.squares) {
		return false
	}
	*p = Square(int(it.list.squares[it.i]))
	it.i++
	return true
}
func (it *squareIter) Done() {}

// RowArray is a flat, indexable, iterable array of rows: the shape the bulk
// collections take instead of a map keyed by square. Rows are PRE-ALLOCATED
// once per session and merely rebound, so `obs.units[i]` allocates nothing.
type RowArray struct {
	rows  []*Row
	count func() int
}

var (
	_ starlark.Value     = (*RowArray)(nil)
	_ starlark.Indexable = (*RowArray)(nil)
	_ starlark.Sequence  = (*RowArray)(nil)
)

func (a *RowArray) String() string        { return fmt.Sprintf("<rows %d>", a.Len()) }
func (a *RowArray) Type() string          { return "rows" }
func (a *RowArray) Freeze()               {}
func (a *RowArray) Truth() starlark.Bool  { return starlark.Bool(a.Len() > 0) }
func (a *RowArray) Hash() (uint32, error) { return 0, fmt.Errorf("unhashable type: rows") }
func (a *RowArray) Len() int              { return a.count() }
func (a *RowArray) Index(i int) starlark.Value {
	return a.rows[i]
}
func (a *RowArray) Iterate() starlark.Iterator { return &rowIter{array: a} }

type rowIter struct {
	array *RowArray
	i     int
}

func (it *rowIter) Next(p *starlark.Value) bool {
	if it.i >= it.array.Len() {
		return false
	}
	*p = it.array.rows[it.i]
	it.i++
	return true
}
func (it *rowIter) Done() {}

// NewRowArray pre-allocates every row object one kind will ever need for one
// session — bounded by the protocol (<=32 units, <=64 squares) — so a
// decision mints no row objects at all.
func NewRowArray(schema *Schema, source Source, sess *Session, capacity int) *RowArray {
	repeated, repeatedInts := 0, 0
	for _, f := range schema.fields {
		if f.Repeated {
			repeated++
		}
		if f.RepeatedInts {
			repeatedInts++
		}
	}
	rows := make([]*Row, capacity)
	backing := make([]Row, capacity)
	var lists []SquareList
	if repeated > 0 {
		lists = make([]SquareList, capacity*repeated)
	}
	var ints []IntList
	if repeatedInts > 0 {
		ints = make([]IntList, capacity*repeatedInts)
	}
	for i := range backing {
		backing[i].schema = schema
		backing[i].source = source
		backing[i].index = i
		backing[i].sess = sess
		if repeated > 0 {
			backing[i].lists = lists[i*repeated : (i+1)*repeated]
		}
		if repeatedInts > 0 {
			backing[i].ints = ints[i*repeatedInts : (i+1)*repeatedInts]
		}
		rows[i] = &backing[i]
	}
	return &RowArray{rows: rows, count: source.Len}
}

// Rebind stamps the current decision's generation onto every pre-allocated
// row. It is the one write the Go side makes between decisions, and it is
// what makes a reference from the previous decision fail loudly.
func (a *RowArray) Rebind(gen uint64) {
	for _, r := range a.rows {
		r.gen = gen
	}
}

// Board is the founder's `board[rank][file]` shape: a fixed 8x8 grid of row
// references (or None), indexable positionally with no keyed lookup anywhere.
type Board struct {
	ranks [8]*BoardRank
}

// BoardRank is one rank of the board.
type BoardRank struct {
	cells [8]starlark.Value
}

var (
	_ starlark.Value     = (*Board)(nil)
	_ starlark.Indexable = (*Board)(nil)
	_ starlark.Value     = (*BoardRank)(nil)
	_ starlark.Indexable = (*BoardRank)(nil)
)

func (b *Board) String() string             { return "<board>" }
func (b *Board) Type() string               { return "board" }
func (b *Board) Freeze()                    {}
func (b *Board) Truth() starlark.Bool       { return starlark.True }
func (b *Board) Hash() (uint32, error)      { return 0, fmt.Errorf("unhashable type: board") }
func (b *Board) Len() int                   { return 8 }
func (b *Board) Index(i int) starlark.Value { return b.ranks[i] }
func (b *Board) Iterate() starlark.Iterator { return &boardIter{board: b} }

type boardIter struct {
	board *Board
	i     int
}

func (it *boardIter) Next(p *starlark.Value) bool {
	if it.i >= 8 {
		return false
	}
	*p = it.board.ranks[it.i]
	it.i++
	return true
}
func (it *boardIter) Done() {}

func (r *BoardRank) String() string             { return "<rank>" }
func (r *BoardRank) Type() string               { return "rank" }
func (r *BoardRank) Freeze()                    {}
func (r *BoardRank) Truth() starlark.Bool       { return starlark.True }
func (r *BoardRank) Hash() (uint32, error)      { return 0, fmt.Errorf("unhashable type: rank") }
func (r *BoardRank) Len() int                   { return 8 }
func (r *BoardRank) Index(i int) starlark.Value { return r.cells[i] }
func (r *BoardRank) Iterate() starlark.Iterator { return &rankIter{rank: r} }

type rankIter struct {
	rank *BoardRank
	i    int
}

func (it *rankIter) Next(p *starlark.Value) bool {
	if it.i >= 8 {
		return false
	}
	*p = it.rank.cells[it.i]
	it.i++
	return true
}
func (it *rankIter) Done() {}

// NewBoard pre-allocates the whole grid once per session, filled with None.
func NewBoard() *Board {
	b := &Board{}
	for i := range b.ranks {
		rank := &BoardRank{}
		for j := range rank.cells {
			rank.cells[j] = starlark.None
		}
		b.ranks[i] = rank
	}
	return b
}

// Place binds one square (file-major 0..63) to a row, or clears it with nil.
func (b *Board) Place(square int, row starlark.Value) {
	file, rank := square/8, square%8
	if row == nil {
		row = starlark.None
	}
	b.ranks[rank].cells[file] = row
}

// Clear resets every square to None between decisions.
func (b *Board) Clear() {
	for _, rank := range b.ranks {
		for j := range rank.cells {
			rank.cells[j] = starlark.None
		}
	}
}
