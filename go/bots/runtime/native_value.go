// Copyright 2026 Sneat.app

package runtime

import "go.starlark.net/starlark"

// Native-value aliases are the deliberately narrow interpreter boundary for
// engine-owned immutable values. A game host may implement these interfaces
// without importing go.starlark.net directly; runtime remains the one public
// package that owns the concrete interpreter dependency.
//
// They are aliases, rather than a wrapper hierarchy, because Starlark's
// evaluator discovers Mapping, IterableMapping, HasAttrs and Sequence by
// exact method signatures. This keeps native values allocation-free while
// preserving the established import boundary.
type (
	Value           = starlark.Value
	Bool            = starlark.Bool
	Builtin         = starlark.Builtin
	Container       = starlark.Container
	HasAttrs        = starlark.HasAttrs
	Indexable       = starlark.Indexable
	IterableMapping = starlark.IterableMapping
	Iterator        = starlark.Iterator
	Sequence        = starlark.Sequence
	String          = starlark.String
	Thread          = starlark.Thread
	Tuple           = starlark.Tuple
)

var (
	None = starlark.None
	True = starlark.True
)

func AsString(v Value) (string, bool) { return starlark.AsString(v) }

func NewBuiltin(name string, fn func(*Thread, *Builtin, Tuple, []Tuple) (Value, error)) *Builtin {
	return starlark.NewBuiltin(name, fn)
}

func UnpackPositionalArgs(fnname string, args Tuple, kwargs []Tuple, min int, vars ...any) error {
	return starlark.UnpackPositionalArgs(fnname, args, kwargs, min, vars...)
}
