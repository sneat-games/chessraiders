// Copyright 2026 Sneat.app

package standardbot_test

import (
	"encoding/json"
	"strconv"
	"testing"

	"github.com/sneat-games/chessraiders/go/bots/runtime"
	"github.com/sneat-games/chessraiders/go/bots/standardbot"
)

func BenchmarkStandardBotDecide(b *testing.B) {
	entries := corpusEntries(&testing.T{})
	if len(entries) == 0 {
		b.Fatal("no corpus entries")
	}
	c := readCorpusCase(&testing.T{}, entries[0])
	draw, _ := strconv.ParseInt(c.RandomDraw, 10, 64)

	var obs standardbot.Observation
	if err := json.Unmarshal(c.Observation, &obs); err != nil {
		b.Fatal(err)
	}
	var params standardbot.BotParams
	if err := json.Unmarshal(c.Parameters, &params); err != nil {
		b.Fatal(err)
	}
	var memory map[string]int64
	if len(c.Memory) > 0 {
		_ = json.Unmarshal(c.Memory, &memory)
	}
	if memory == nil {
		memory = make(map[string]int64)
	}

	b.Run("Go_NativeStructs", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			memCopy := make(map[string]int64, len(memory))
			for k, v := range memory {
				memCopy[k] = v
			}
			_, _, _ = standardbot.Decide(&obs, memCopy, &params, draw, c.Options)
		}
	})

	b.Run("Go_JSONWire", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _, _, err := standardbot.DecideJSON(c.Observation, c.Memory, c.Parameters, draw, c.Options)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	program, err := runtime.Compile(standardbot.Script)
	if err != nil {
		b.Fatal(err)
	}
	drawJSON, _ := json.Marshal(draw)
	optionsJSON, _ := json.Marshal(c.Options)
	memBytes := c.Memory
	if len(memBytes) == 0 {
		memBytes = json.RawMessage("{}")
	}

	b.Run("Starlark_Script", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := program.Call("decide",
				string(c.Observation), string(memBytes), string(c.Parameters), string(drawJSON), string(optionsJSON))
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}
