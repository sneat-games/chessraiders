// Copyright 2026 Sneat.app

package botvalue

import (
	"testing"

	"go.starlark.net/starlark"
)

type squareMaskTestSource struct{}

func (squareMaskTestSource) Len() int                 { return 1 }
func (squareMaskTestSource) Field(int, int) Scalar    { return NoneScalar() }
func (squareMaskTestSource) Squares(int, int) []uint8 { return nil }
func (squareMaskTestSource) Ints(int, int) []int32    { return nil }
func (squareMaskTestSource) Mask(int, int) uint32     { return 0 }
func (squareMaskTestSource) SquareMask(int, int) uint64 {
	return (uint64(1) << 0) | (uint64(1) << 12) | (uint64(1) << 63)
}

func TestSquareMaskListIsOrderedImmutableSequence(t *testing.T) {
	sess := &Session{}
	schema := NewSchema("unit", FieldSpec{Name: "destinations", SquareMask: true})
	rows := NewRowArray(schema, squareMaskTestSource{}, sess, 1)
	gen := sess.Begin()
	rows.Rebind(gen)
	defer sess.End()

	value, err := rows.rows[0].Attr("destinations")
	if err != nil {
		t.Fatalf("Attr(destinations): %v", err)
	}
	list, ok := value.(*SquareMaskList)
	if !ok {
		t.Fatalf("destinations = %T, want *SquareMaskList", value)
	}
	if got := list.Len(); got != 3 {
		t.Fatalf("Len() = %d, want 3", got)
	}
	if got := list.Index(0); got != Square(0) {
		t.Fatalf("Index(0) = %v, want a1", got)
	}
	if got := list.Index(1); got != Square(12) {
		t.Fatalf("Index(1) = %v, want b5", got)
	}
	if got := list.Index(2); got != Square(63) {
		t.Fatalf("Index(2) = %v, want h8", got)
	}
	if ok, err := list.Has(starlark.String("b5")); err != nil || !ok {
		t.Fatalf("b5 membership = %v, %v; want true, nil", ok, err)
	}
	if ok, err := list.Has(starlark.String("c3")); err != nil || ok {
		t.Fatalf("c3 membership = %v, %v; want false, nil", ok, err)
	}
}

func TestSquareMaskFieldRequiresSquareMaskSource(t *testing.T) {
	sess := &Session{}
	schema := NewSchema("unit", FieldSpec{Name: "destinations", SquareMask: true})
	rows := NewRowArray(schema, scalarOnlySource{}, sess, 1)
	gen := sess.Begin()
	rows.Rebind(gen)
	defer sess.End()
	if _, err := rows.rows[0].Attr("destinations"); err == nil {
		t.Fatal("square-mask field without SquareMaskSource succeeded")
	}
}
