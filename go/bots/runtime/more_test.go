// Copyright 2026 Sneat.app

package runtime

import (
	"strings"
	"testing"

	"github.com/sneat-games/chessraiders/go/bots/botvalue"
	"go.starlark.net/starlark"
	"go.starlark.net/syntax"
)

func TestNativeValueHelpers(t *testing.T) {
	if s, ok := AsString(starlark.String("hello")); !ok || s != "hello" {
		t.Errorf("AsString failed: %q, %v", s, ok)
	}

	b := NewBuiltin("my_builtin", func(thread *Thread, fn *Builtin, args Tuple, kwargs []Tuple) (Value, error) {
		return None, nil
	})
	if b.Name() != "my_builtin" {
		t.Errorf("NewBuiltin name = %q, want my_builtin", b.Name())
	}

	if err := UnpackPositionalArgs("my_builtin", Tuple{}, nil, 0); err != nil {
		t.Errorf("UnpackPositionalArgs failed: %v", err)
	}
}

func TestProgramValuesAndStepLimits(t *testing.T) {
	src := `
global_var = 42

def add(x, y):
    return x + y

def loop(n):
    total = 0
    for i in range(n):
        total += i
    return total

def return_func(x):
    return lambda: x
`
	prog, err := Compile(src)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	// CallValues success
	val, err := prog.CallValues("add", starlark.MakeInt(10), starlark.MakeInt(20))
	if err != nil {
		t.Fatalf("CallValues failed: %v", err)
	}
	intVal, ok := botvalue.AsInt(val)
	if !ok || intVal != 30 {
		t.Errorf("CallValues result = %v, want 30", val)
	}

	// CallValues with nil argument
	_, err = prog.CallValues("add", nil, starlark.MakeInt(5))
	if err == nil {
		t.Error("CallValues with nil arg did not produce error when adding None")
	}

	// CallValues missing global
	if _, err := prog.CallValues("missing_func"); err == nil {
		t.Error("CallValues with missing function succeeded, want error")
	}

	// CallValues not callable
	if _, err := prog.CallValues("global_var"); err == nil {
		t.Error("CallValues with non-callable succeeded, want error")
	}

	// CallValuesWithStepLimit success
	_, steps, err := prog.CallValuesWithStepLimit(10_000, "add", starlark.MakeInt(1), starlark.MakeInt(2))
	if err != nil || steps == 0 {
		t.Errorf("CallValuesWithStepLimit failed: %v, steps=%d", err, steps)
	}

	// CallValuesWithStepLimit exceeded
	_, _, err = prog.CallValuesWithStepLimit(5, "loop", starlark.MakeInt(100))
	if err == nil {
		t.Error("CallValuesWithStepLimit exceeded step limit succeeded, want error")
	}

	// CallValueWithJSONArgsWithStepLimit success
	res, steps, err := prog.CallValueWithJSONArgsWithStepLimit(10_000, "add", starlark.MakeInt(3), "4")
	if err != nil || res != "7" || steps == 0 {
		t.Errorf("CallValueWithJSONArgsWithStepLimit failed: %v, res=%s steps=%d", err, res, steps)
	}

	// CallValueWithJSONArgsWithStepLimit step limit exceeded
	_, _, err = prog.CallValueWithJSONArgsWithStepLimit(5, "loop", starlark.MakeInt(100))
	if err == nil {
		t.Error("CallValueWithJSONArgsWithStepLimit exceeded limit succeeded, want error")
	}

	// CallValueWithJSONArgs missing function
	if _, err := prog.CallValueWithJSONArgs("missing", nil); err == nil {
		t.Error("CallValueWithJSONArgs missing function succeeded, want error")
	}

	// CallValueWithJSONArgs non-callable
	if _, err := prog.CallValueWithJSONArgs("global_var", nil); err == nil {
		t.Error("CallValueWithJSONArgs non-callable succeeded, want error")
	}

	// CallValueWithJSONArgs invalid JSON arg
	if _, err := prog.CallValueWithJSONArgs("add", nil, "{invalid"); err == nil {
		t.Error("CallValueWithJSONArgs invalid JSON succeeded, want error")
	}

	// CallValueWithJSONArgs encode failure (unencodable lambda return)
	if _, err := prog.CallValueWithJSONArgs("return_func", nil); err == nil {
		t.Error("CallValueWithJSONArgs returning function succeeded encoding, want error")
	}

	// Call encode failure (unencodable lambda return)
	if _, err := prog.Call("return_func", "1"); err == nil {
		t.Error("Call returning function succeeded encoding, want error")
	}

	// Call with decode failure
	if _, err := prog.Call("add", "{bad", "1"); err == nil {
		t.Error("Call with decode failure succeeded, want error")
	}

	// CompileWithStepLimit error
	if _, err := CompileWithStepLimit("def syntax_error( :::", 100); err == nil {
		t.Error("CompileWithStepLimit with syntax error succeeded, want error")
	}
}

func TestEntrypointValidationEdges(t *testing.T) {
	// Error formatting with no Reason
	errNoReason := &EntrypointError{Name: "decide", Got: 3, Want: 5}
	if !strings.Contains(errNoReason.Error(), "has 3 parameters, want exactly 5") {
		t.Errorf("EntrypointError format = %q", errNoReason.Error())
	}

	// Empty name
	if err := ValidateEntrypoint("def decide(): pass", "", 0); err == nil {
		t.Error("ValidateEntrypoint with empty name succeeded, want error")
	}

	// Negative arity
	if err := ValidateEntrypoint("def decide(): pass", "decide", -1); err == nil {
		t.Error("ValidateEntrypoint with negative arity succeeded, want error")
	}

	// Duplicate definition
	dupSrc := `
def decide():
    pass

def decide():
    pass
`
	if err := ValidateEntrypoint(dupSrc, "decide", 0); err == nil {
		t.Error("ValidateEntrypoint with duplicate definition succeeded, want error")
	}

	// Default parameter (BinaryExpr with Op == syntax.EQ)
	defaultParamSrc := `
def decide(a, b, c=1):
    pass
`
	if err := ValidateEntrypoint(defaultParamSrc, "decide", 3); err != nil {
		t.Errorf("ValidateEntrypoint with default param failed: %v", err)
	}

	// Alias cycle in file
	aliasCycleSrc := `
a = b
b = a

def decide():
    a()
`
	if err := ValidateEntrypoint(aliasCycleSrc, "decide", 0); err == nil {
		t.Error("ValidateEntrypoint with alias cycle succeeded, want error")
	}

	// Indirect callable expression (e.g. (f)())
	indirectCallableSrc := `
def helper():
    return 1

def decide():
    (helper)()
`
	if err := ValidateEntrypoint(indirectCallableSrc, "decide", 0); err == nil {
		t.Error("ValidateEntrypoint with indirect callable succeeded, want error")
	}

	// Positional callback in sorted(items, key_fn)
	sortedPositionalSrc := `
def key_fn(x):
    return x

def decide():
    sorted([1, 2], key_fn)
`
	if err := ValidateEntrypoint(sortedPositionalSrc, "decide", 0); err != nil {
		t.Errorf("ValidateEntrypoint with positional sorted callback failed: %v", err)
	}

	// Tuple args expansion in sorted(*(...))
	sortedTupleArgsSrc := `
def key_fn(x):
    return x

def decide():
    sorted(*( [1, 2], key_fn ))
`
	if err := ValidateEntrypoint(sortedTupleArgsSrc, "decide", 0); err != nil {
		t.Errorf("ValidateEntrypoint with tuple args expansion failed: %v", err)
	}

	// Non-string key in **kwargs
	badKwargsSrc := `
def decide():
    sorted([1, 2], **{1: None})
`
	if err := ValidateEntrypoint(badKwargsSrc, "decide", 0); err == nil {
		t.Error("ValidateEntrypoint with non-string key in kwargs succeeded, want error")
	}

	// Other builtin (like len) in function body
	otherBuiltinSrc := `
def decide(state):
    return len([state])
`
	if err := ValidateEntrypoint(otherBuiltinSrc, "decide", 1); err != nil {
		t.Errorf("ValidateEntrypoint with len failed: %v", err)
	}

	// Nested def inside function body
	nestedDefSrc := `
def decide(state):
    def helper():
        return 1
    return helper()
`
	if err := ValidateEntrypoint(nestedDefSrc, "decide", 1); err != nil {
		t.Errorf("ValidateEntrypoint with nested def failed: %v", err)
	}

	// Lambda as key callback in sorted
	badSortedCallbackSrc := `
def decide(state):
    return sorted([1], key=lambda x: x)
`
	if err := ValidateEntrypoint(badSortedCallbackSrc, "decide", 1); err == nil {
		t.Error("ValidateEntrypoint with lambda key callback succeeded, want error")
	}

	// List args expansion in sorted(*[...])
	sortedListArgsSrc := `
def key_fn(x):
    return x

def decide():
    sorted(*[ [1, 2], key_fn ])
`
	if err := ValidateEntrypoint(sortedListArgsSrc, "decide", 0); err != nil {
		t.Errorf("ValidateEntrypoint with list args expansion failed: %v", err)
	}
}

func TestStaticRecursionCycleUnboundOrUnresolved(t *testing.T) {
	// from == nil
	file := &syntax.File{
		Stmts: []syntax.Stmt{
			&syntax.DefStmt{Name: &syntax.Ident{Name: "unbound"}},
		},
	}
	staticRecursionCycle(file)

	// unresolved callable in body
	file2 := &syntax.File{
		Stmts: []syntax.Stmt{
			&syntax.DefStmt{
				Name: &syntax.Ident{Name: "caller", Binding: "binding"},
				Body: []syntax.Stmt{
					&syntax.ExprStmt{
						X: &syntax.CallExpr{
							Fn: &syntax.Ident{Name: "unresolved"},
						},
					},
				},
			},
		},
	}
	staticRecursionCycle(file2)
}

func TestCanaryAndNewRuntimeFailures(t *testing.T) {
	// Mutate DialectOptions to make RunCanary fail
	origSet := DialectOptions.Set
	defer func() { DialectOptions.Set = origSet }()

	DialectOptions.Set = false
	if err := RunCanary(); err == nil {
		t.Error("RunCanary with Set=false succeeded, want error")
	}

	if _, err := New(); err == nil {
		t.Error("New() runtime with failed canary succeeded, want error")
	}

	// runCanary with nil/unsupported options
	if err := runCanary(&syntax.FileOptions{}); err == nil {
		t.Error("runCanary with empty options succeeded, want error")
	}

	// canary_passed not true
	origSource := canarySource
	defer func() { canarySource = origSource }()
	canarySource = "canary_passed = False\n"
	if err := RunCanary(); err == nil {
		t.Error("RunCanary with canary_passed=False succeeded, want error")
	}
}
