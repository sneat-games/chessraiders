// Copyright 2026 Sneat.app

package runtime

import (
	"errors"
	"strings"
	"testing"
)

func TestValidateEntrypointAcceptsTheExactStaticShape(t *testing.T) {
	const source = `
x = 1
def decide(observation, memory, params, random_draw, options):
    return None
`
	if err := ValidateEntrypoint(source, "decide", 5); err != nil {
		t.Fatalf("ValidateEntrypoint() = %v, want nil", err)
	}
}

func TestValidateEntrypointNeverExecutesTopLevel(t *testing.T) {
	const source = `
x = fail("top level must not execute during validation")
def decide(observation, memory, params, random_draw, options):
    return None
`
	if err := ValidateEntrypoint(source, "decide", 5); err != nil {
		t.Fatalf("ValidateEntrypoint() = %v, want static shape validation without top-level execution", err)
	}
}

func TestValidateEntrypointUsesTheExactProgramPredeclaredNames(t *testing.T) {
	accepted := `
def decide(observation, memory, params, random_draw, options):
    return json.encode(params)
`
	if err := ValidateEntrypoint(accepted, "decide", 5); err != nil {
		t.Fatalf("ValidateEntrypoint(json predeclared) = %v, want nil", err)
	}
	for _, name := range []string{"input", "rand", "invented_host_name"} {
		source := `
def decide(observation, memory, params, random_draw, options):
    return ` + name + `
`
		if err := ValidateEntrypoint(source, "decide", 5); err == nil {
			t.Fatalf("ValidateEntrypoint(%s) = nil, want unresolved-identifier failure", name)
		}
	}
}

func TestValidateEntrypointRejectsLoadAndRecursionWithoutExecuting(t *testing.T) {
	tests := []struct {
		name   string
		source string
	}{
		{
			name: "load",
			source: `
load("helper.star", "helper")
def decide(observation, memory, params, random_draw, options):
    return helper()
`,
		},
		{
			name: "recursion",
			source: `
def recurse():
    return recurse()
def decide(observation, memory, params, random_draw, options):
    return recurse()
`,
		},
		{
			name: "mutual recursion",
			source: `
def first():
    return second()
def second():
    return first()
def decide(observation, memory, params, random_draw, options):
    return first()
`,
		},
		{
			name: "recursion through top-level alias",
			source: `
def recurse():
    return alias()
alias = recurse
def decide(observation, memory, params, random_draw, options):
    return alias()
`,
		},
		{
			name: "recursion through local callable alias",
			source: `
def recurse():
    alias = recurse
    return alias()
def decide(observation, memory, params, random_draw, options):
    return recurse()
`,
		},
		{
			name: "recursion through higher-order call",
			source: `
def invoke(fn):
    return fn()
def decide(observation, memory, params, random_draw, options):
    return invoke(decide)
`,
		},
		{
			name: "recursion through higher-order builtin callback",
			source: `
def decide(observation, memory, params, random_draw, options):
    return sorted([observation], key=decide)
`,
		},
		{
			name: "recursion through sorted literal positional expansion",
			source: `
def decide(observation, memory, params, random_draw, options):
    return sorted(*([observation], decide))
`,
		},
		{
			name: "recursion through sorted literal keyword expansion",
			source: `
def decide(observation, memory, params, random_draw, options):
    return sorted(**{"iterable": [observation], "key": decide})
`,
		},
		{
			name: "recursion through min literal keyword expansion",
			source: `
def decide(observation, memory, params, random_draw, options):
    return min([observation], **{"key": decide})
`,
		},
		{
			name: "recursion through max literal keyword expansion",
			source: `
def decide(observation, memory, params, random_draw, options):
    return max([observation], **{"key": decide})
`,
		},
		{
			name: "every explicit callback is graphed",
			source: `
def helper(value):
    return value
def decide(observation, memory, params, random_draw, options):
    return min([observation], key=helper, **{"key": decide})
`,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := ValidateEntrypoint(test.source, "decide", 5); err == nil {
				t.Fatal("ValidateEntrypoint() = nil, want static rejection")
			}
		})
	}
}

func TestValidateEntrypointRejectsDynamicHigherOrderBuiltinExpansions(t *testing.T) {
	for _, test := range []struct {
		name   string
		source string
	}{
		{
			name: "sorted args",
			source: `
def decide(observation, memory, params, random_draw, options):
    args = ([observation], decide)
    return sorted(*args)
`,
		},
		{
			name: "sorted kwargs",
			source: `
def decide(observation, memory, params, random_draw, options):
    kwargs = {"iterable": [observation], "key": decide}
    return sorted(**kwargs)
`,
		},
		{
			name: "min kwargs",
			source: `
def decide(observation, memory, params, random_draw, options):
    kwargs = {"key": decide}
    return min([observation], **kwargs)
`,
		},
		{
			name: "max kwargs",
			source: `
def decide(observation, memory, params, random_draw, options):
    kwargs = {"key": decide}
    return max([observation], **kwargs)
`,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := ValidateEntrypoint(test.source, "decide", 5)
			if err == nil {
				t.Fatal("ValidateEntrypoint() = nil, want dynamic spread rejection")
			}
			if !strings.Contains(err.Error(), "dynamic") {
				t.Fatalf("ValidateEntrypoint() error = %v, want dynamic-spread reason", err)
			}
		})
	}
}

func TestValidateEntrypointResolvesBenignTopLevelFunctionAliases(t *testing.T) {
	const source = `
def helper():
    return None
alias = helper
def decide(observation, memory, params, random_draw, options):
    return alias()
`
	if err := ValidateEntrypoint(source, "decide", 5); err != nil {
		t.Fatalf("ValidateEntrypoint(benign function alias) = %v, want nil", err)
	}
}

func TestValidateEntrypointRejectsAnUnresolvedCallableParameter(t *testing.T) {
	const source = `
def helper(helper):
    return helper()
def decide(observation, memory, params, random_draw, options):
    return helper(random_draw)
`
	if err := ValidateEntrypoint(source, "decide", 5); err == nil {
		t.Fatal("ValidateEntrypoint(callable parameter) = nil, want conservative indirect-call rejection")
	}
}

func TestValidateEntrypointRejectsALaterGlobalRebinding(t *testing.T) {
	for _, source := range []string{
		`def decide(observation, memory, params, random_draw, options):
    return None
decide = 1
`,
		`def decide(observation, memory, params, random_draw, options):
    return None
(decide,) = (1,)
`,
	} {
		if err := ValidateEntrypoint(source, "decide", 5); err == nil {
			t.Fatalf("ValidateEntrypoint(%q) = nil, want final-global-binding rejection", source)
		}
	}
}

func TestValidateEntrypointRejectsMissingWrongAndVariadicDefinitions(t *testing.T) {
	tests := []struct {
		name   string
		source string
	}{
		{name: "missing", source: "def other(a):\n    return None\n"},
		{name: "wrong count", source: "def decide(a, b):\n    return None\n"},
		{name: "variadic", source: "def decide(a, b, c, d, *e):\n    return None\n"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := ValidateEntrypoint(test.source, "decide", 5)
			if err == nil {
				t.Fatal("ValidateEntrypoint() = nil, want an entrypoint error")
			}
			var entrypointError *EntrypointError
			if !errors.As(err, &entrypointError) && !strings.Contains(err.Error(), "statically compile entrypoint") {
				t.Fatalf("ValidateEntrypoint() error = %T %v, want a typed entrypoint or parse error", err, err)
			}
		})
	}
}
