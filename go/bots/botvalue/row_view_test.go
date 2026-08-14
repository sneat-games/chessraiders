// Copyright 2026 Sneat.app

package botvalue

import "testing"

type rowTestSource struct {
	len    int
	nested Value
}

func (s rowTestSource) Len() int               { return s.len }
func (rowTestSource) Field(int, int) Scalar    { return NoneScalar() }
func (rowTestSource) Squares(int, int) []uint8 { return nil }
func (rowTestSource) Ints(int, int) []int32    { return nil }
func (rowTestSource) Mask(int, int) uint32     { return 0 }
func (s rowTestSource) Value(int, int) Value   { return s.nested }

func TestRowNestedValueAndFixedOrderView(t *testing.T) {
	sess := &Session{}
	childSchema := NewSchema("unit")
	children := NewRowArray(childSchema, rowTestSource{len: 3}, sess, 3)
	view := NewRowView(children)

	rootSchema := NewSchema("observation", FieldSpec{Name: "units", Nested: true})
	roots := NewRowArray(rootSchema, rowTestSource{len: 1, nested: view}, sess, 1)

	gen := sess.Begin()
	children.Rebind(gen)
	roots.Rebind(gen)
	defer sess.End()

	order := [64]uint16{2, 0}
	view.SetOrder(order, 2)
	if got := view.Len(); got != 2 {
		t.Fatalf("view.Len() = %d, want 2", got)
	}
	if got := view.Index(0); got != children.ValueAt(2) {
		t.Fatalf("view.Index(0) does not preserve source-row identity")
	}
	if got := view.Index(1); got != children.ValueAt(0) {
		t.Fatalf("view.Index(1) does not preserve source-row identity")
	}

	units, err := roots.rows[0].Attr("units")
	if err != nil {
		t.Fatalf("root.Attr(units) = %v", err)
	}
	if units != view {
		t.Fatalf("root.Attr(units) = %T, want exact native row view", units)
	}
}

func TestNestedFieldRequiresNestedSource(t *testing.T) {
	sess := &Session{}
	// scalarOnlySource intentionally does not implement NestedSource: a
	// malformed schema must fail rather than quietly yield None.
	schema := NewSchema("observation", FieldSpec{Name: "units", Nested: true})
	rows := NewRowArray(schema, scalarOnlySource{}, sess, 1)
	gen := sess.Begin()
	rows.Rebind(gen)
	defer sess.End()
	if _, err := rows.rows[0].Attr("units"); err == nil {
		t.Fatal("nested field without NestedSource succeeded, want loud schema error")
	}
}

func TestRelationsResolveByUnitIdentityNotFoggedSquareOrder(t *testing.T) {
	sess := &Session{}
	schema := NewSchema("cell", FieldSpec{Name: "defenders", Relation: true})
	rows := NewRowArray(schema, relationRowSource{}, sess, 64)
	identities := &UnitIdentityView{}
	// Cell 0 is a fog-memory cell with no UnitID. UnitID 8 is visible at cell
	// 40, so bit 7 must resolve to that row regardless of square ordering.
	identities.Bind(7, rows.ValueAt(40))
	rows.BindUnitIdentityView(identities)
	view := NewRowView(rows)
	order := [64]uint16{0, 40}
	view.SetOrder(order, 2)

	gen := sess.Begin()
	rows.Rebind(gen)
	defer sess.End()

	value, err := rows.rows[1].Attr("defenders")
	if err != nil {
		t.Fatalf("row.Attr(defenders): %v", err)
	}
	list, ok := value.(*UnitList)
	if !ok {
		t.Fatalf("relation = %T, want *UnitList", value)
	}
	if got := list.Index(0); got != rows.ValueAt(40) {
		t.Fatalf("relation resolved %v, want visible UnitID 8 row", got)
	}
	if ok, err := list.Has(rows.ValueAt(40)); err != nil || !ok {
		t.Fatalf("visible relation membership = %v, %v; want true, nil", ok, err)
	}
	if ok, err := list.Has(rows.ValueAt(0)); err != nil || ok {
		t.Fatalf("fog cell relation membership = %v, %v; want false, nil", ok, err)
	}
	if got := view.Index(1); got != rows.ValueAt(40) {
		t.Fatalf("square-order view lost cell identity: %v", got)
	}
}

func TestRowViewRebindStampsOnlyRowsItCanExpose(t *testing.T) {
	sess := &Session{}
	schema := NewSchema("cell", FieldSpec{Name: "square", Kind: KindSquare})
	rows := NewRowArray(schema, rowTestSource{len: 2}, sess, 2)
	view := NewRowView(rows)
	order := [64]uint16{1}
	view.SetOrder(order, 1)
	gen := sess.Begin()
	view.Rebind(gen)
	defer sess.End()

	if _, err := rows.rows[1].Attr("square"); err != nil {
		t.Fatalf("visible row Attr(square): %v", err)
	}
	if _, err := rows.rows[0].Attr("square"); err == nil {
		t.Fatal("unexposed row read succeeded without a lifetime stamp")
	}
}

func TestRelationMasksHideUnboundIdentities(t *testing.T) {
	sess := &Session{}
	schema := NewSchema("cell", FieldSpec{Name: "defenders", Relation: true})
	rows := NewRowArray(schema, relationRowSource{}, sess, 2)
	rows.BindUnitIdentityView(&UnitIdentityView{})
	gen := sess.Begin()
	rows.Rebind(gen)
	defer sess.End()

	value, err := rows.rows[1].Attr("defenders")
	if err != nil {
		t.Fatalf("row.Attr(defenders): %v", err)
	}
	list := value.(*UnitList)
	if got := list.Len(); got != 0 {
		t.Fatalf("hidden relation Len() = %d, want 0", got)
	}
	if got := list.Index(0); got != None {
		t.Fatalf("hidden relation Index(0) = %v, want None", got)
	}
}

type relationRowSource struct{}

func (relationRowSource) Len() int                 { return 64 }
func (relationRowSource) Field(int, int) Scalar    { return NoneScalar() }
func (relationRowSource) Squares(int, int) []uint8 { return nil }
func (relationRowSource) Ints(int, int) []int32    { return nil }
func (relationRowSource) Mask(row, field int) uint32 {
	if row == 1 && field == 0 {
		return 1 << 7
	}
	return 0
}

type scalarOnlySource struct{}

func (scalarOnlySource) Len() int                 { return 1 }
func (scalarOnlySource) Field(int, int) Scalar    { return NoneScalar() }
func (scalarOnlySource) Squares(int, int) []uint8 { return nil }
func (scalarOnlySource) Ints(int, int) []int32    { return nil }
func (scalarOnlySource) Mask(int, int) uint32     { return 0 }
