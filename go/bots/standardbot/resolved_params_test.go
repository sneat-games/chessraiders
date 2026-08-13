// Copyright 2026 Sneat.app

package standardbot_test

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/sneat-games/chessraiders/go/bots/standardbot"
)

// TestResolvedParamsFileMatchesTheResolver regenerates params.resolved.json's
// rows in memory via the same MarshalResolvedParams/ResolveParams call
// gen/main.go makes to write the committed file, and fails if the committed
// bytes have drifted from what the resolver actually returns today. This is
// what makes shipping a stale params.resolved.json impossible: CI already
// runs `go test ./...`, so any params.json or params.schema.json edit that
// is not followed by `go generate ./bots/standardbot` fails right here.
func TestResolvedParamsFileMatchesTheResolver(t *testing.T) {
	want, err := standardbot.MarshalResolvedParams()
	if err != nil {
		t.Fatalf("MarshalResolvedParams() = %v", err)
	}
	if !bytes.Equal(want, standardbot.ResolvedParams) {
		t.Fatalf("params.resolved.json is stale: run `go generate ./bots/standardbot` from the go/ module root and commit the regenerated file")
	}
}

// TestResolvedParamsRowsCarryExpectedTierValues is a sanity check on top of
// the byte-for-byte drift test above: it names a few values a reviewer can
// eyeball directly, rather than trusting only an opaque byte comparison.
// Lieutenant's overrides come from params.json; Commander is params.json's
// empty override object, so its row is pure params.schema.json defaults.
func TestResolvedParamsRowsCarryExpectedTierValues(t *testing.T) {
	var resolved struct {
		Schema string                     `json:"schema"`
		Rows   map[string]json.RawMessage `json:"rows"`
	}
	if err := json.Unmarshal(standardbot.ResolvedParams, &resolved); err != nil {
		t.Fatalf("params.resolved.json does not parse as JSON: %v", err)
	}
	if want := "chess-raiders-bot-resolved-parameters/v1"; resolved.Schema != want {
		t.Fatalf("schema = %q, want %q", resolved.Schema, want)
	}

	var lieutenant map[string]json.RawMessage
	if err := json.Unmarshal(resolved.Rows["lieutenant"], &lieutenant); err != nil {
		t.Fatalf("rows.lieutenant does not parse as an object: %v", err)
	}
	for key, want := range map[string]string{
		"advance":          "0.1",
		"beaconAggression": "0",
		"prisoner":         "0.5",
		"system":           "1",
	} {
		if got := string(lieutenant[key]); got != want {
			t.Fatalf("rows.lieutenant[%q] = %s, want %s", key, got, want)
		}
	}

	commander, err := standardbot.ResolveParams("commander")
	if err != nil {
		t.Fatalf("ResolveParams(commander) = %v", err)
	}
	var gotCommander, wantCommander bytes.Buffer
	if err := json.Compact(&gotCommander, resolved.Rows["commander"]); err != nil {
		t.Fatalf("compact rows.commander: %v", err)
	}
	if err := json.Compact(&wantCommander, commander); err != nil {
		t.Fatalf("compact ResolveParams(commander): %v", err)
	}
	if got, want := gotCommander.String(), wantCommander.String(); got != want {
		t.Fatalf("rows.commander = %s, want %s (params.schema.json's pure defaults)", got, want)
	}
}
