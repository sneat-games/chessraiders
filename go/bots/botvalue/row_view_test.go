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

	order := [32]uint8{2, 0}
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

type scalarOnlySource struct{}

func (scalarOnlySource) Len() int                 { return 1 }
func (scalarOnlySource) Field(int, int) Scalar    { return NoneScalar() }
func (scalarOnlySource) Squares(int, int) []uint8 { return nil }
func (scalarOnlySource) Ints(int, int) []int32    { return nil }
func (scalarOnlySource) Mask(int, int) uint32     { return 0 }
