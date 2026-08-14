// Copyright 2026 Sneat.app

package botvalue_test

// arch_cost_test.go measures the per-operation allocation cost of a relation
// set on BOTH targets, because they do not agree and the browser is the one
// that matters most.
//
// go.starlark.net represents an int32-range Int as a single unsafe.Pointer on
// darwin/arm64 (int_posix64.go, which reserves a 4GB address range so an Int
// fits in one word), and as a two-word {int64, *big.Int} union everywhere the
// optimisation is unavailable — including GOARCH=wasm (int_generic.go). So
// boxing an integer into a starlark.Value is free on the server and an
// allocation in the browser, and reasoning about browser cost from server
// numbers is exactly the mistake this file exists to avoid.
//
// Run both:
//
//	go test ./bots/botvalue -run TestRelationSetCostByOperation -v
//	GOOS=js GOARCH=wasm go test -exec="$(go env GOROOT)/lib/wasm/go_js_wasm_exec" \
//	    ./bots/botvalue -run TestRelationSetCostByOperation -v

import (
	"testing"

	"github.com/sneat-games/chessraiders/go/bots/botvalue"
	"github.com/sneat-games/chessraiders/go/bots/runtime"
)

func measureLoop(t *testing.T, h *nativeHost, body string) float64 {
	t.Helper()
	source := "def decide(observation, memory, params, host_random_draw, options):\n" +
		"    total = 0\n" + body + "    return total, {}, []\n"
	p, err := runtime.Compile(source)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	return testing.AllocsPerRun(100, func() {
		gen := h.sess.Begin()
		h.array.Rebind(gen)
		if _, err := p.CallValues("decide", h.obs, botvalue.None, botvalue.None, botvalue.None, botvalue.None); err != nil {
			h.sess.End()
			t.Fatal(err)
		}
		h.sess.End()
	})
}

// TestRelationSetCostByOperation isolates each thing a script does with a
// relation and reports its cost NET of the loop that carries it, so the
// interned-count claim and the raw-integer trap are separated from the
// iteration overhead they share.
func TestRelationSetCostByOperation(t *testing.T) {
	h := newNativeHost(newFixture())
	h.array.BindIdentityView(h.array)

	// Two baselines, because they differ by an amount that is itself a
	// finding: a bare walk, and the same walk doing ONE integer addition per
	// unit. Every case below performs exactly one addition per unit too, so
	// subtracting the arithmetic baseline isolates the relation operation
	// rather than the accumulator carrying it.
	walk := measureLoop(t, h, "    for unit in observation.units:\n        pass\n")
	arith := measureLoop(t, h, "    for unit in observation.units:\n        total += 1\n")

	cases := []struct {
		name string
		body string
	}{
		{"count (interned small int)", "    for unit in observation.units:\n        total += unit.guarded_by.count\n"},
		{"membership (unit in set)", "    for unit in observation.units:\n        total += (1 if unit in unit.guarded_by else 0)\n"},
		{"set union (guards | threatens)", "    for unit in observation.units:\n        total += (unit.guarded_by | unit.threatened_by).count\n"},
		{"set difference (a - b)", "    for unit in observation.units:\n        total += (unit.guarded_by - unit.threatened_by).count\n"},
		{"RAW .bits integer (the trap)", "    for unit in observation.units:\n        total += unit.guarded_by.bits\n"},
	}

	t.Logf("bare walk over %d units          %8.1f allocs/op", unitCount, walk)
	t.Logf("walk + one int addition each    %8.1f allocs/op  (%.2f per addition)",
		arith, (arith-walk)/float64(unitCount))
	for _, c := range cases {
		got := measureLoop(t, h, c.body)
		net := got - arith
		t.Logf("%-32s %8.1f allocs/op total  %8.1f net  (%.2f per unit)",
			c.name, got, net, net/float64(unitCount))
	}

	// Iteration is reported separately: its body runs once per MEMBER, not
	// once per unit, so it shares no baseline with the rows above.
	iter := measureLoop(t, h, "    for unit in observation.units:\n        for other in unit.guarded_by:\n            total += 1\n")
	members := 0
	for i := range newFixture() {
		_ = i
		members += relationSize
	}
	t.Logf("%-32s %8.1f allocs/op total  (%d loops + %d member additions)",
		"iterate members", iter, unitCount, members)
}
