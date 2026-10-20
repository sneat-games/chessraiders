// Copyright 2026 Sneat.app

// Command gen materializes go/bots/standardbot/params.resolved.json. It
// derives every row solely by calling standardbot.MarshalResolvedParams,
// which itself derives every row solely by calling standardbot.ResolveParams
// — this module's existing JSON-Schema resolver — once per named set in
// params.json. This command contains no resolution or default-filling logic
// of its own; it only writes the bytes ResolveParams (by way of
// MarshalResolvedParams) already produced.
//
// Run it via `go generate ./bots/standardbot/...` from the go/ module root
// (see the //go:generate directive in ../resolved_params.go), which sets the
// working directory to go/bots/standardbot so the relative output path below
// lands in the right place. Running `go run ./gen` directly from
// go/bots/standardbot works the same way.
package main

import (
	"fmt"
	"io"
	"os"

	"github.com/sneat-games/chessraiders/go/bots/standardbot"
)

var (
	exitFunc                  = os.Exit
	stderrWriter    io.Writer = os.Stderr
	marshalParams             = standardbot.MarshalResolvedParams
	writeOutputFile           = os.WriteFile
)

func main() {
	if err := run(); err != nil {
		_, _ = fmt.Fprintln(stderrWriter, "gen:", err)
		exitFunc(1)
	}
}

func run() error {
	data, err := marshalParams()
	if err != nil {
		return err
	}
	return writeOutputFile("params.resolved.json", data, 0o644)
}
