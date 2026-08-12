// Copyright 2026 Sneat.app

package runtime

import (
	"fmt"
	"sort"

	"go.starlark.net/resolve"
	"go.starlark.net/starlark"
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

// ValidateEntrypoint parses, resolves and compiles source using the runtime's
// one dialect and exact Program predeclared names, then proves that it declares
// exactly one non-variadic function named name with arity named parameters. A
// trailing default is allowed: it remains an explicitly declared argument when
// an official caller supplies all arity arguments. It rejects unresolved
// identifiers, recursion and load statements, but never calls Program.Init and
// therefore never executes the module's top-level statements.
func ValidateEntrypoint(source, name string, arity int) error {
	if name == "" {
		return &EntrypointError{Reason: "name is empty"}
	}
	if arity < 0 {
		return &EntrypointError{Name: name, Want: arity, Reason: "arity is negative"}
	}
	file, program, err := starlark.SourceProgramOptions(DialectOptions, "script.star", source, programPredeclared().Has)
	if err != nil {
		return fmt.Errorf("starlarkbot: statically compile entrypoint %q: %w", name, err)
	}
	if program.NumLoads() != 0 {
		module, position := program.Load(0)
		return &EntrypointError{Name: name, Want: arity, Reason: fmt.Sprintf("load is forbidden (%q at %s)", module, position)}
	}
	cycle, unsupportedCall := staticRecursionCycle(file)
	if unsupportedCall != "" {
		return &EntrypointError{Name: name, Want: arity, Reason: unsupportedCall}
	}
	if len(cycle) != 0 {
		return &EntrypointError{Name: name, Want: arity, Reason: "recursion is forbidden: " + joinCycle(cycle)}
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

func staticRecursionCycle(file *syntax.File) ([]string, string) {
	var functions []*syntax.DefStmt
	bindingNames := map[any]string{}
	bindingPositions := map[any]string{}
	syntax.Walk(file, func(node syntax.Node) bool {
		if definition, ok := node.(*syntax.DefStmt); ok {
			functions = append(functions, definition)
			if definition.Name.Binding != nil {
				bindingNames[definition.Name.Binding] = definition.Name.Name
				bindingPositions[definition.Name.Binding] = definition.Name.NamePos.String()
			}
		}
		return true
	})
	aliasTargets := map[any]any{}
	for _, statement := range file.Stmts {
		assignment, ok := statement.(*syntax.AssignStmt)
		if !ok || assignment.Op != syntax.EQ {
			continue
		}
		alias, aliasOK := assignment.LHS.(*syntax.Ident)
		target, targetOK := assignment.RHS.(*syntax.Ident)
		if aliasOK && targetOK && alias.Binding != nil && target.Binding != nil {
			aliasTargets[alias.Binding] = target.Binding
		}
	}
	resolveFunction := func(binding any) (any, bool) {
		seen := map[any]struct{}{}
		for binding != nil {
			if _, ok := bindingNames[binding]; ok {
				return binding, true
			}
			if _, duplicate := seen[binding]; duplicate {
				return nil, false
			}
			seen[binding] = struct{}{}
			next, ok := aliasTargets[binding]
			if !ok {
				return nil, false
			}
			binding = next
		}
		return nil, false
	}
	resolveCallable := func(expression syntax.Expr) (target any, userFunction bool, reason string) {
		identifier, ok := expression.(*syntax.Ident)
		if !ok {
			return nil, false, fmt.Sprintf("indirect callable expression at %s is forbidden", expressionStart(expression))
		}
		binding, ok := identifier.Binding.(*resolve.Binding)
		if !ok {
			return nil, false, fmt.Sprintf("unresolved callable %q at %s is forbidden", identifier.Name, identifier.NamePos)
		}
		if binding.Scope == resolve.Universal || binding.Scope == resolve.Predeclared {
			return nil, false, ""
		}
		if target, ok := resolveFunction(identifier.Binding); ok {
			return target, true, ""
		}
		return nil, false, fmt.Sprintf("indirect callable %q at %s is forbidden", identifier.Name, identifier.NamePos)
	}

	edges := make(map[any]map[any]struct{}, len(functions))
	unsupportedCall := ""
	for _, definition := range functions {
		from := definition.Name.Binding
		if from == nil {
			continue
		}
		edges[from] = map[any]struct{}{}
		for _, statement := range definition.Body {
			syntax.Walk(statement, func(node syntax.Node) bool {
				if unsupportedCall != "" {
					return false
				}
				if nested, ok := node.(*syntax.DefStmt); ok && nested != definition {
					return false
				}
				call, ok := node.(*syntax.CallExpr)
				if !ok {
					return true
				}
				identifier, directIdentifier := call.Fn.(*syntax.Ident)
				if !directIdentifier {
					if _, methodCall := call.Fn.(*syntax.DotExpr); !methodCall {
						unsupportedCall = fmt.Sprintf("indirect callable expression at %s is forbidden", call.Lparen)
						return false
					}
					return true
				}
				target, userFunction, reason := resolveCallable(identifier)
				if reason != "" {
					unsupportedCall = reason
					return false
				}
				if userFunction {
					edges[from][target] = struct{}{}
					return true
				}
				if callback := builtinKeyCallback(call, identifier.Name); callback != nil {
					target, userFunction, reason := resolveCallable(callback)
					if reason != "" {
						unsupportedCall = reason
						return false
					}
					if userFunction {
						edges[from][target] = struct{}{}
					}
				}
				return true
			})
			if unsupportedCall != "" {
				break
			}
		}
		if unsupportedCall != "" {
			break
		}
	}
	if unsupportedCall != "" {
		return nil, unsupportedCall
	}

	state := map[any]uint8{}
	stack := make([]any, 0, len(edges))
	var visit func(any) []string
	visit = func(binding any) []string {
		state[binding] = 1
		stack = append(stack, binding)
		targets := make([]any, 0, len(edges[binding]))
		for target := range edges[binding] {
			targets = append(targets, target)
		}
		sort.Slice(targets, func(left, right int) bool {
			return staticFunctionSortKey(targets[left], bindingNames, bindingPositions) < staticFunctionSortKey(targets[right], bindingNames, bindingPositions)
		})
		for _, target := range targets {
			switch state[target] {
			case 0:
				if cycle := visit(target); len(cycle) != 0 {
					return cycle
				}
			case 1:
				for index, stacked := range stack {
					if stacked == target {
						cycle := make([]string, 0, len(stack)-index+1)
						for _, member := range stack[index:] {
							cycle = append(cycle, bindingNames[member])
						}
						return append(cycle, bindingNames[target])
					}
				}
			}
		}
		stack = stack[:len(stack)-1]
		state[binding] = 2
		return nil
	}
	bindings := make([]any, 0, len(edges))
	for binding := range edges {
		bindings = append(bindings, binding)
	}
	sort.Slice(bindings, func(left, right int) bool {
		return staticFunctionSortKey(bindings[left], bindingNames, bindingPositions) < staticFunctionSortKey(bindings[right], bindingNames, bindingPositions)
	})
	for _, binding := range bindings {
		if state[binding] == 0 {
			if cycle := visit(binding); len(cycle) != 0 {
				return cycle, ""
			}
		}
	}
	return nil, ""
}

func builtinKeyCallback(call *syntax.CallExpr, callee string) syntax.Expr {
	if callee != "sorted" && callee != "min" && callee != "max" {
		return nil
	}
	for _, argument := range call.Args {
		keyword, ok := argument.(*syntax.BinaryExpr)
		if !ok || keyword.Op != syntax.EQ {
			continue
		}
		name, ok := keyword.X.(*syntax.Ident)
		if ok && name.Name == "key" {
			return keyword.Y
		}
	}
	if callee == "sorted" && len(call.Args) >= 2 {
		if _, keyword := call.Args[1].(*syntax.BinaryExpr); !keyword {
			return call.Args[1]
		}
	}
	return nil
}

func expressionStart(expression syntax.Expr) syntax.Position {
	start, _ := expression.Span()
	return start
}

func staticFunctionSortKey(binding any, names, positions map[any]string) string {
	return names[binding] + "\x00" + positions[binding]
}

func joinCycle(cycle []string) string {
	result := ""
	for index, name := range cycle {
		if index != 0 {
			result += " -> "
		}
		result += name
	}
	return result
}
