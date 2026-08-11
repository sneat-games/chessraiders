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
while True:
    pass
def decide(observation, memory, params, random_draw, options):
    return None
`
	if err := ValidateEntrypoint(source, "decide", 5); err != nil {
		t.Fatalf("ValidateEntrypoint() = %v, want static shape validation without top-level execution", err)
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
			if !errors.As(err, &entrypointError) && !strings.Contains(err.Error(), "parse entrypoint") {
				t.Fatalf("ValidateEntrypoint() error = %T %v, want a typed entrypoint or parse error", err, err)
			}
		})
	}
}
