// Copyright 2026 Sneat.app

package botvalue

import (
	"math/big"
	"testing"

	"go.starlark.net/starlark"
)

func TestInternedHelpers(t *testing.T) {
	if got := Square(-1); got != starlark.None {
		t.Errorf("Square(-1) = %v, want None", got)
	}
	if got := Square(64); got != starlark.None {
		t.Errorf("Square(64) = %v, want None", got)
	}
	if got := Square(0); got == starlark.None {
		t.Errorf("Square(0) = None, want valid square")
	}

	if got := SquareName(-1); got != "" {
		t.Errorf("SquareName(-1) = %q, want empty", got)
	}
	if got := SquareName(64); got != "" {
		t.Errorf("SquareName(64) = %q, want empty", got)
	}
	if got := SquareName(0); got != "a1" {
		t.Errorf("SquareName(0) = %q, want a1", got)
	}

	if got := Int(-1); got == nil {
		t.Errorf("Int(-1) = nil")
	}
	if got := Int(5); got == nil {
		t.Errorf("Int(5) = nil")
	}
	if got := Int(1000); got == nil {
		t.Errorf("Int(1000) = nil")
	}

	if got := Bool(true); got != starlark.True {
		t.Errorf("Bool(true) != True")
	}
	if got := Bool(false); got != starlark.False {
		t.Errorf("Bool(false) != False")
	}

	v := NewVocabulary("first", "", "third")
	if v.Len() != 3 {
		t.Errorf("v.Len() = %d, want 3", v.Len())
	}
	if v.Value(-1) != starlark.None || v.Value(10) != starlark.None {
		t.Errorf("v.Value out of range should be None")
	}
	if v.Value(0) != starlark.String("first") {
		t.Errorf("v.Value(0) = %v, want 'first'", v.Value(0))
	}
	if v.Value(1) != starlark.None {
		t.Errorf("v.Value(1) for empty string should be None")
	}
	if v.Name(-1) != "" || v.Name(10) != "" {
		t.Errorf("v.Name out of range should be empty")
	}
	if v.Name(0) != "first" {
		t.Errorf("v.Name(0) = %q, want 'first'", v.Name(0))
	}

	var nilVocab *Vocabulary
	if nilVocab.Len() != 0 {
		t.Errorf("nilVocab.Len() = %d, want 0", nilVocab.Len())
	}
	if nilVocab.Value(0) != starlark.None {
		t.Errorf("nilVocab.Value(0) should be None")
	}
	if nilVocab.Name(0) != "" {
		t.Errorf("nilVocab.Name(0) should be empty")
	}

	if val, ok := AsInt(starlark.MakeInt64(42)); !ok || val != 42 {
		t.Errorf("AsInt failed: val=%d ok=%v", val, ok)
	}
	if _, ok := AsInt(starlark.String("abc")); ok {
		t.Errorf("AsInt(string) succeeded, want false")
	}

	if val, ok := AsString(starlark.String("hello")); !ok || val != "hello" {
		t.Errorf("AsString failed: val=%q ok=%v", val, ok)
	}
	if _, ok := AsString(starlark.MakeInt(1)); ok {
		t.Errorf("AsString(int) succeeded, want false")
	}

	tup := starlark.Tuple{starlark.MakeInt(10), starlark.String("x")}
	if n, ok := TupleLen(tup); !ok || n != 2 {
		t.Errorf("TupleLen failed: n=%d ok=%v", n, ok)
	}
	if _, ok := TupleLen(starlark.MakeInt(1)); ok {
		t.Errorf("TupleLen(int) succeeded, want false")
	}

	if item, ok := TupleAt(tup, 0); !ok || item != starlark.MakeInt(10) {
		t.Errorf("TupleAt(0) failed: item=%v ok=%v", item, ok)
	}
	if _, ok := TupleAt(tup, -1); ok {
		t.Errorf("TupleAt(-1) succeeded, want false")
	}
	if _, ok := TupleAt(tup, 10); ok {
		t.Errorf("TupleAt(10) succeeded, want false")
	}
	if _, ok := TupleAt(starlark.MakeInt(1), 0); ok {
		t.Errorf("TupleAt(int, 0) succeeded, want false")
	}

	if idx, ok := SquareIndex("a1"); !ok || idx != 0 {
		t.Errorf("SquareIndex(a1) = %d, %v, want 0, true", idx, ok)
	}
	if idx, ok := SquareIndex("h8"); !ok || idx != 63 {
		t.Errorf("SquareIndex(h8) = %d, %v, want 63, true", idx, ok)
	}
	if _, ok := SquareIndex("a"); ok {
		t.Errorf("SquareIndex(a) succeeded, want false")
	}
	if _, ok := SquareIndex("z1"); ok {
		t.Errorf("SquareIndex(z1) succeeded, want false")
	}
	if _, ok := SquareIndex("a9"); ok {
		t.Errorf("SquareIndex(a9) succeeded, want false")
	}
}

func TestRelations(t *testing.T) {
	if got := Relation(0).Name(); got != "defenders" {
		t.Errorf("Relation(0).Name() = %q, want 'defenders'", got)
	}
	if got := Relation(99).Name(); got != "" {
		t.Errorf("Relation(99).Name() = %q, want empty", got)
	}
	if got := Relation(0).String(); got != "defenders" {
		t.Errorf("Relation(0).String() = %q, want 'defenders'", got)
	}

	if rel, ok := RelationByWireV1("threatenedBy"); !ok || rel != Attackers {
		t.Errorf("RelationByWireV1(threatenedBy) = %v, %v", rel, ok)
	}
	if _, ok := RelationByWireV1("invalid"); ok {
		t.Errorf("RelationByWireV1(invalid) succeeded, want false")
	}

	fields := RelationFields()
	if len(fields) != len(Relations) {
		t.Errorf("RelationFields len = %d, want %d", len(fields), len(Relations))
	}
}

func TestSquareMaskList(t *testing.T) {
	mask := &SquareMaskList{bits: 0b101}
	if mask.String() != "<squares 2>" {
		t.Errorf("mask.String() = %q", mask.String())
	}
	if mask.Type() != "squares" {
		t.Errorf("mask.Type() = %q", mask.Type())
	}
	mask.Freeze()
	if mask.Truth() != starlark.True {
		t.Errorf("mask.Truth() != True")
	}
	emptyMask := &SquareMaskList{bits: 0}
	if emptyMask.Truth() != starlark.False {
		t.Errorf("emptyMask.Truth() != False")
	}
	if _, err := mask.Hash(); err == nil {
		t.Errorf("mask.Hash() succeeded, want error")
	}
	if mask.Len() != 2 {
		t.Errorf("mask.Len() = %d, want 2", mask.Len())
	}
	if got := mask.Index(0); got == starlark.None {
		t.Errorf("mask.Index(0) = None, want square")
	}
	if got := mask.Index(10); got != starlark.None {
		t.Errorf("mask.Index(10) = %v, want None", got)
	}

	iter := mask.Iterate()
	defer iter.Done()
	var val starlark.Value
	count := 0
	for iter.Next(&val) {
		count++
	}
	if count != 2 {
		t.Errorf("iter count = %d, want 2", count)
	}

	if has, err := mask.Has(starlark.String("a1")); err != nil || !has {
		t.Errorf("mask.Has(a1) = %v, %v, want true, nil", has, err)
	}
	if has, err := mask.Has(starlark.MakeInt(1)); err != nil || has {
		t.Errorf("mask.Has(non-string) = %v, %v, want false, nil", has, err)
	}
	if has, err := mask.Has(starlark.String("invalid")); err != nil || has {
		t.Errorf("mask.Has(invalid) = %v, %v, want false, nil", has, err)
	}
	if has, err := mask.Has(starlark.String("b1")); err != nil || has {
		t.Errorf("mask.Has(b1) = %v, %v, want false, nil", has, err)
	}
}

func TestUnitListMore(t *testing.T) {
	ul := &UnitList{}
	if ul.String() != "<units 0>" {
		t.Errorf("ul.String() = %q", ul.String())
	}
	if ul.Type() != "units" {
		t.Errorf("ul.Type() = %q", ul.Type())
	}
	ul.Freeze()
	if ul.Truth() != starlark.False {
		t.Errorf("ul.Truth() != False")
	}
	if _, err := ul.Hash(); err == nil {
		t.Errorf("ul.Hash() succeeded, want error")
	}
	if got := ul.Index(0); got != starlark.None {
		t.Errorf("ul.Index(0) = %v, want None", got)
	}
	if got := ul.Index(2); got != starlark.None {
		t.Errorf("ul.Index(2) = %v, want None", got)
	}

	if countVal, err := ul.Attr("count"); err != nil || countVal == nil {
		t.Errorf("ul.Attr(count) failed: %v", err)
	}
	if other, err := ul.Attr("other"); err != nil || other != nil {
		t.Errorf("ul.Attr(other) = %v, %v, want nil, nil", other, err)
	}

	iter := ul.Iterate()
	iter.Done()
	var p starlark.Value
	if iter.Next(&p) {
		t.Errorf("empty unit list iter returned true")
	}

	it := &unitListIter{list: &UnitList{rows: nil}, remaining: 1}
	if it.Next(&p) {
		t.Errorf("it.Next() should return false when rows is nil")
	}

	if has, err := ul.Has(starlark.MakeInt(1)); err != nil || has {
		t.Errorf("ul.Has(int) = %v, %v, want false, nil", has, err)
	}
	if has, err := ul.Has(&Row{}); err != nil || has {
		t.Errorf("ul.Has(Row with nil rows) = %v, %v, want false, nil", has, err)
	}
}

func TestRowAndListsMore(t *testing.T) {
	sess := &Session{}
	if sess.Generation() != 0 {
		t.Errorf("sess.Generation() = %d, want 0", sess.Generation())
	}

	schema := NewSchema("testrow",
		FieldSpec{Name: "num", RepeatedInts: true},
		FieldSpec{Name: "sqs", Repeated: true},
		FieldSpec{Name: "flt"},
	)
	if schema.TypeName() != "testrow" {
		t.Errorf("schema.TypeName() = %q", schema.TypeName())
	}
	if idx, ok := schema.FieldIndex("num"); !ok || idx != 0 {
		t.Errorf("schema.FieldIndex(num) = %d, %v", idx, ok)
	}
	if _, ok := schema.FieldIndex("unknown"); ok {
		t.Errorf("schema.FieldIndex(unknown) succeeded")
	}

	fltScalar := FloatScalar(3.14)
	if fltScalar.Kind != KindFloat || fltScalar.Float != 3.14 {
		t.Errorf("FloatScalar failed: %v", fltScalar)
	}

	il := &IntList{values: []int32{10, 20}}
	if il.String() != "<ints 2>" {
		t.Errorf("il.String() = %q", il.String())
	}
	if il.Type() != "ints" {
		t.Errorf("il.Type() = %q", il.Type())
	}
	il.Freeze()
	if il.Truth() != starlark.True {
		t.Errorf("il.Truth() != True")
	}
	emptyIl := &IntList{}
	if emptyIl.Truth() != starlark.False {
		t.Errorf("emptyIl.Truth() != False")
	}
	if _, err := il.Hash(); err == nil {
		t.Errorf("il.Hash() succeeded")
	}
	if il.Len() != 2 {
		t.Errorf("il.Len() = %d", il.Len())
	}
	if got := il.Index(0); got == nil {
		t.Errorf("il.Index(0) = nil")
	}
	iIter := il.Iterate()
	defer iIter.Done()
	var iVal starlark.Value
	c := 0
	for iIter.Next(&iVal) {
		c++
	}
	if c != 2 {
		t.Errorf("int iter count = %d, want 2", c)
	}
	if has, err := il.Has(starlark.MakeInt(10)); err != nil || !has {
		t.Errorf("il.Has(10) = %v, %v", has, err)
	}
	if has, err := il.Has(starlark.MakeInt(99)); err != nil || has {
		t.Errorf("il.Has(99) = %v, %v", has, err)
	}
	if has, err := il.Has(starlark.String("str")); err != nil || has {
		t.Errorf("il.Has(string) = %v, %v", has, err)
	}
	bigInt := starlark.MakeBigInt(new(big.Int).Lsh(big.NewInt(1), 70))
	if has, err := il.Has(bigInt); err != nil || has {
		t.Errorf("il.Has(bigInt) = %v, %v, want false, nil", has, err)
	}

	sl := &SquareList{squares: []uint8{0, 63}}
	if sl.String() != "<squares 2>" {
		t.Errorf("sl.String() = %q", sl.String())
	}
	if sl.Type() != "squares" {
		t.Errorf("sl.Type() = %q", sl.Type())
	}
	sl.Freeze()
	if sl.Truth() != starlark.True {
		t.Errorf("sl.Truth() != True")
	}
	if (&SquareList{}).Truth() != starlark.False {
		t.Errorf("empty sl.Truth() != False")
	}
	if _, err := sl.Hash(); err == nil {
		t.Errorf("sl.Hash() succeeded")
	}
	if sl.Len() != 2 {
		t.Errorf("sl.Len() = %d", sl.Len())
	}
	if got := sl.Index(0); got == starlark.None {
		t.Errorf("sl.Index(0) = None")
	}
	sIter := sl.Iterate()
	defer sIter.Done()
	var sVal starlark.Value
	sc := 0
	for sIter.Next(&sVal) {
		sc++
	}
	if sc != 2 {
		t.Errorf("square iter count = %d", sc)
	}
	if has, err := sl.Has(starlark.String("a1")); err != nil || !has {
		t.Errorf("sl.Has(a1) = %v, %v", has, err)
	}
	if has, err := sl.Has(starlark.String("b1")); err != nil || has {
		t.Errorf("sl.Has(b1) = %v, %v", has, err)
	}
	if has, err := sl.Has(starlark.MakeInt(1)); err != nil || has {
		t.Errorf("sl.Has(int) = %v, %v", has, err)
	}
	if has, err := sl.Has(starlark.String("bad")); err != nil || has {
		t.Errorf("sl.Has(bad) = %v, %v", has, err)
	}

	rowsArr := NewRowArray(schema, rowTestSource{len: 2}, sess, 2)
	if rowsArr.String() != "<rows 2>" {
		t.Errorf("rowsArr.String() = %q", rowsArr.String())
	}
	if rowsArr.Type() != "rows" {
		t.Errorf("rowsArr.Type() = %q", rowsArr.Type())
	}
	rowsArr.Freeze()
	if rowsArr.Truth() != starlark.True {
		t.Errorf("rowsArr.Truth() != True")
	}
	if _, err := rowsArr.Hash(); err == nil {
		t.Errorf("rowsArr.Hash() succeeded")
	}
	if rowsArr.Index(0) == nil {
		t.Errorf("rowsArr.Index(0) == nil")
	}
	rIter := rowsArr.Iterate()
	defer rIter.Done()
	var rVal starlark.Value
	rc := 0
	for rIter.Next(&rVal) {
		rc++
	}
	if rc != 2 {
		t.Errorf("row array iter count = %d", rc)
	}
	if has, err := rowsArr.Has(rowsArr.rows[0]); err != nil || !has {
		t.Errorf("rowsArr.Has(rows[0]) = %v, %v", has, err)
	}
	if has, err := rowsArr.Has(&Row{}); err != nil || has {
		t.Errorf("rowsArr.Has(unrelated) = %v, %v", has, err)
	}
	if has, err := rowsArr.Has(starlark.MakeInt(1)); err != nil || has {
		t.Errorf("rowsArr.Has(int) = %v, %v", has, err)
	}

	row := rowsArr.rows[0]
	if row.String() != "<testrow 0>" {
		t.Errorf("row.String() = %q", row.String())
	}
	if row.Type() != "testrow" {
		t.Errorf("row.Type() = %q", row.Type())
	}
	row.Freeze()
	if row.Truth() != starlark.True {
		t.Errorf("row.Truth() != True")
	}
	if _, err := row.Hash(); err == nil {
		t.Errorf("row.Hash() succeeded")
	}
	if _, err := row.Attr("nonexistent"); err != nil {
		t.Errorf("row.Attr(nonexistent) returned error: %v", err)
	}
}

func TestRowViewMore(t *testing.T) {
	sess := &Session{}
	schema := NewSchema("u")
	arr := NewRowArray(schema, rowTestSource{len: 4}, sess, 4)
	view := NewRowView(arr)

	view.SetOrder([64]uint16{0, 1}, -1)
	if view.Len() != 0 {
		t.Errorf("view.Len() = %d, want 0", view.Len())
	}
	view.SetOrder([64]uint16{0, 100}, 2)
	if view.Len() != 1 {
		t.Errorf("view.Len() = %d, want 1", view.Len())
	}
	view.SetOrder([64]uint16{}, 70)
	if view.Len() != 64 {
		t.Errorf("view.Len() = %d, want 64", view.Len())
	}
	emptyView := NewRowView(nil)
	emptyView.SetOrder([64]uint16{0}, 1)
	if emptyView.Len() != 0 {
		t.Errorf("emptyView.Len() = %d, want 0", emptyView.Len())
	}

	view.SetRange(-1, 2)
	if view.Len() != 0 {
		t.Errorf("view.Len() = %d, want 0", view.Len())
	}
	view.SetRange(0, -1)
	if view.Len() != 0 {
		t.Errorf("view.Len() = %d, want 0", view.Len())
	}
	view.SetRange(10, 2)
	if view.Len() != 0 {
		t.Errorf("view.Len() = %d, want 0", view.Len())
	}
	emptyView.SetRange(0, 2)
	if emptyView.Len() != 0 {
		t.Errorf("emptyView.SetRange with nil source did not produce 0")
	}
	view.SetRange(1, 10)
	if view.Len() != 3 {
		t.Errorf("view.Len() after clamped range = %d, want 3", view.Len())
	}

	gen := sess.Begin()
	view.Rebind(gen)
	emptyView.Rebind(gen)
	defer sess.End()

	if view.String() != "<rows 3>" {
		t.Errorf("view.String() = %q", view.String())
	}
	if view.Type() != "rows" {
		t.Errorf("view.Type() = %q", view.Type())
	}
	view.Freeze()
	if view.Truth() != starlark.True {
		t.Errorf("view.Truth() != True")
	}
	if emptyView.Truth() != starlark.False {
		t.Errorf("emptyView.Truth() != False")
	}
	if _, err := view.Hash(); err == nil {
		t.Errorf("view.Hash() succeeded")
	}
	if got := view.Index(-1); got != starlark.None {
		t.Errorf("view.Index(-1) = %v, want None", got)
	}
	if got := view.Index(10); got != starlark.None {
		t.Errorf("view.Index(10) = %v, want None", got)
	}
	if got := emptyView.Index(0); got != starlark.None {
		t.Errorf("emptyView.Index(0) = %v, want None", got)
	}

	vIter := view.Iterate()
	defer vIter.Done()
	var val starlark.Value
	vc := 0
	for vIter.Next(&val) {
		vc++
	}
	if vc != 3 {
		t.Errorf("row view iter count = %d, want 3", vc)
	}

	if has, err := view.Has(arr.rows[1]); err != nil || !has {
		t.Errorf("view.Has(rows[1]) = %v, %v", has, err)
	}
	if has, err := view.Has(arr.rows[0]); err != nil || has {
		t.Errorf("view.Has(rows[0] which is out of range) = %v, %v", has, err)
	}
	if has, err := view.Has(starlark.MakeInt(1)); err != nil || has {
		t.Errorf("view.Has(int) = %v, %v", has, err)
	}
	if has, err := emptyView.Has(arr.rows[0]); err != nil || has {
		t.Errorf("emptyView.Has() = %v, %v", has, err)
	}
}

func TestUnitIdentityViewAndBoard(t *testing.T) {
	uiv := &UnitIdentityView{}
	row := &Row{}
	uiv.Bind(-1, row)
	uiv.Bind(35, row)
	uiv.Bind(0, starlark.None)
	if uiv.presentMask() != 0 {
		t.Errorf("uiv.presentMask() after invalid binds = %d, want 0", uiv.presentMask())
	}
	uiv.Bind(0, row)
	if uiv.presentMask() != 1 {
		t.Errorf("uiv.presentMask() = %d, want 1", uiv.presentMask())
	}
	uiv.Clear()
	if uiv.presentMask() != 0 {
		t.Errorf("uiv.presentMask() after clear = %d, want 0", uiv.presentMask())
	}

	board := NewBoard()
	if board.String() != "<board>" {
		t.Errorf("board.String() = %q", board.String())
	}
	if board.Type() != "board" {
		t.Errorf("board.Type() = %q", board.Type())
	}
	board.Freeze()
	if board.Truth() != starlark.True {
		t.Errorf("board.Truth() != True")
	}
	if _, err := board.Hash(); err == nil {
		t.Errorf("board.Hash() succeeded")
	}
	if board.Len() != 8 {
		t.Errorf("board.Len() = %d, want 8", board.Len())
	}
	if board.Index(0) == nil {
		t.Errorf("board.Index(0) = nil")
	}

	bIter := board.Iterate()
	defer bIter.Done()
	var bVal starlark.Value
	bc := 0
	for bIter.Next(&bVal) {
		bc++
	}
	if bc != 8 {
		t.Errorf("board iter count = %d, want 8", bc)
	}

	rank := board.ranks[0]
	if rank.String() != "<rank>" {
		t.Errorf("rank.String() = %q", rank.String())
	}
	if rank.Type() != "rank" {
		t.Errorf("rank.Type() = %q", rank.Type())
	}
	rank.Freeze()
	if rank.Truth() != starlark.True {
		t.Errorf("rank.Truth() != True")
	}
	if _, err := rank.Hash(); err == nil {
		t.Errorf("rank.Hash() succeeded")
	}
	if rank.Len() != 8 {
		t.Errorf("rank.Len() = %d, want 8", rank.Len())
	}
	if rank.Index(0) != starlark.None {
		t.Errorf("rank.Index(0) = %v, want None", rank.Index(0))
	}

	rIter := rank.Iterate()
	defer rIter.Done()
	var rVal starlark.Value
	rc := 0
	for rIter.Next(&rVal) {
		rc++
	}
	if rc != 8 {
		t.Errorf("rank iter count = %d, want 8", rc)
	}

	board.Place(0, row)
	if board.ranks[0].cells[0] != row {
		t.Errorf("board.ranks[0].cells[0] != row")
	}
	board.Place(0, nil)
	if board.ranks[0].cells[0] != starlark.None {
		t.Errorf("board.ranks[0].cells[0] after nil Place != None")
	}
	board.Place(0, row)
	board.Clear()
	if board.ranks[0].cells[0] != starlark.None {
		t.Errorf("board.ranks[0].cells[0] after Clear != None")
	}
}

type dummyScalarSource struct {
	scalar Scalar
}

func (s dummyScalarSource) Len() int               { return 1 }
func (s dummyScalarSource) Field(int, int) Scalar  { return s.scalar }
func (dummyScalarSource) Squares(int, int) []uint8 { return []uint8{1, 2} }
func (dummyScalarSource) Ints(int, int) []int32    { return nil }
func (dummyScalarSource) Mask(int, int) uint32     { return 0 }

type multiSquareMaskSource struct {
	dummyScalarSource
}

func (multiSquareMaskSource) SquareMask(int, int) uint64 { return 0b11 }

func TestBoxScalarMore(t *testing.T) {
	sess := &Session{}
	schema := NewSchema("item", FieldSpec{Name: "val"})
	gen := sess.Begin()
	defer sess.End()

	s := dummyScalarSource{scalar: FloatScalar(2.718)}
	arr := NewRowArray(schema, s, sess, 1)
	arr.Rebind(gen)
	v, err := arr.rows[0].Attr("val")
	if err != nil {
		t.Fatalf("Attr(val) err = %v", err)
	}
	f, ok := v.(starlark.Float)
	if !ok || float64(f) != 2.718 {
		t.Errorf("Attr(val) = %v (%T), want Float(2.718)", v, v)
	}

	sUnknown := dummyScalarSource{scalar: Scalar{Kind: 99}}
	arr2 := NewRowArray(schema, sUnknown, sess, 1)
	arr2.Rebind(gen)
	v2, err := arr2.rows[0].Attr("val")
	if err != nil {
		t.Fatalf("Attr(val) err = %v", err)
	}
	if v2 != starlark.None {
		t.Errorf("Attr(val) with unknown scalar kind = %v, want None", v2)
	}

	schemaMask := NewSchema("item", FieldSpec{Name: "mask", SquareMask: true})
	arrMask := NewRowArray(schemaMask, s, sess, 1)
	arrMask.Rebind(gen)
	if _, err := arrMask.rows[0].Attr("mask"); err == nil {
		t.Errorf("Attr(mask) on non-SquareMaskSource succeeded, want error")
	}
}

func TestAdditionalCoverageEdges(t *testing.T) {
	sess := &Session{}
	gen := sess.Begin()
	defer sess.End()

	schema := NewSchema("multi",
		FieldSpec{Name: "sqs1", Repeated: true},
		FieldSpec{Name: "sqs2", Repeated: true},
		FieldSpec{Name: "mask1", SquareMask: true},
		FieldSpec{Name: "mask2", SquareMask: true},
	)
	s := multiSquareMaskSource{}
	arr := NewRowArray(schema, s, sess, 1)
	arr.Rebind(gen)

	if val := arr.ValueAt(-1); val != None {
		t.Errorf("ValueAt(-1) = %v, want None", val)
	}
	if val := arr.ValueAt(99); val != None {
		t.Errorf("ValueAt(99) = %v, want None", val)
	}

	if _, err := arr.rows[0].Attr("sqs2"); err != nil {
		t.Errorf("Attr(sqs2) failed: %v", err)
	}
	if _, err := arr.rows[0].Attr("mask2"); err != nil {
		t.Errorf("Attr(mask2) failed: %v", err)
	}
}

func TestPanicOnDuplicateSchema(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("NewSchema with duplicate field did not panic")
		}
	}()
	_ = NewSchema("dup", FieldSpec{Name: "x"}, FieldSpec{Name: "x"})
}
