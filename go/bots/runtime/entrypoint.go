// Copyright 2026 Sneat.app

package runtime

import (
	"fmt"

	"go.starlark.net/syntax"
)

// EntrypointError reports a static script-entrypoint contract failure. It is
// deliberately separate from Compile errors: ValidateEntrypoint never executes
// module top level, so it is safe to use before untrusted source reaches an
// executor.
type EntrypointError struct {
	Name   string
	Want   int
	Got    int
	Reason string
}

func (e *EntrypointError) Error() string {
	if e.Reason != "" {
		return fmt.Sprintf("starlarkbot: entrypoint %q: %s", e.Name, e.Reason)
	}
	return fmt.Sprintf("starlarkbot: entrypoint %q has %d parameters, want exactly %d", e.Name, e.Got, e.Want)
}

// ValidateEntrypoint parses source using the runtime's one dialect and proves
// that it declares exactly one non-variadic function named name with arity
// named parameters. A trailing default is allowed: it remains an explicitly
// declared argument when an official caller supplies all arity arguments. It
// does not compile or execute the module: a manifest
// validator must never run an untrusted module's top-level statements merely to
// learn whether its entrypoint is shaped correctly.
func ValidateEntrypoint(source, name string, arity int) error {
	if name == "" {
		return &EntrypointError{Reason: "name is empty"}
	}
	if arity < 0 {
		return &EntrypointError{Name: name, Want: arity, Reason: "arity is negative"}
	}
	file, err := DialectOptions.Parse("script.star", source, 0)
	if err != nil {
		return fmt.Errorf("starlarkbot: parse entrypoint %q: %w", name, err)
	}

	var found *syntax.DefStmt
	for _, statement := range file.Stmts {
		def, ok := statement.(*syntax.DefStmt)
		if !ok || def.Name.Name != name {
			continue
		}
		if found != nil {
			return &EntrypointError{Name: name, Want: arity, Reason: "is declared more than once"}
		}
		found = def
	}
	if found == nil {
		return &EntrypointError{Name: name, Want: arity, Reason: "is not declared"}
	}
	if len(found.Params) != arity {
		return &EntrypointError{Name: name, Want: arity, Got: len(found.Params)}
	}
	for _, parameter := range found.Params {
		switch parameter := parameter.(type) {
		case *syntax.Ident:
			continue
		case *syntax.BinaryExpr:
			if parameter.Op == syntax.EQ {
				if _, ok := parameter.X.(*syntax.Ident); ok {
					continue
				}
			}
		}
		return &EntrypointError{Name: name, Want: arity, Reason: "must use only named parameters; variadic parameters are not supported"}
	}
	return nil
}
