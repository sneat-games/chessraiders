// Copyright 2026 Sneat.app

package botvalue_test

// shapes_bench_test.go is the decision-grade experiment behind the native-
// value design. It runs THE SAME evaluation, over THE SAME data, at THE SAME
// production cardinality, through three representations:
//
//	A  dict — today's wire: json.decode + square-keyed dicts + the script's
//	   own build_board copies + .get() idioms + square[0]/square[1] parsing.
//	B  native — Go-backed rows and flat arrays instead of dicts, but the
//	   script still derives the same facts itself (len(u.guarded_by), helper
//	   functions kept).
//	C  native+precomputed — Go-backed rows where the host answers the derived
//	   questions directly (u.guarded_count), so the script makes no helper
//	   call at all.
//
// A->B isolates "stop shipping dictionaries through JSON".
// B->C isolates "let the host pre-answer what the script recomputes".
//
// Every variant asserts the identical score, so the comparison cannot be
// won by doing less work.

import (
	"encoding/json"
	"fmt"
	"math"
	"testing"

	"github.com/sneat-games/chessraiders/go/bots/botvalue"
	"github.com/sneat-games/chessraiders/go/bots/runtime"
	"go.starlark.net/starlark"
)

// ---------------------------------------------------------------------------
// Fixture: 32 units on a board, 12 destinations each — the Lieutenant shape
// (Breadth 8, CandidateSpread 12) the production benchmark measures.
// ---------------------------------------------------------------------------

const (
	unitCount    = 32
	ownUnits     = 16
	destPerUnit  = 12
	relationSize = 3
)

var rankNames = []string{"", "pawn", "knight", "bishop", "rook", "queen", "king"}
var rankVocab = botvalue.NewVocabulary(rankNames...)
var sideVocab = botvalue.NewVocabulary("", "white", "black")

var rankMaterial = map[string]float64{
	"pawn": 1.0, "knight": 3.0, "bishop": 3.25, "rook": 5.0, "queen": 9.0, "king": 60.0,
}

type unitFixture struct {
	square       int
	sideCode     int
	rankCode     int
	cargoCount   int
	guardedBy    []uint8
	threatenedBy []uint8
	destinations []uint8
	patrolGain   []int
}

func newFixture() []unitFixture {
	units := make([]unitFixture, unitCount)
	for i := range units {
		u := &units[i]
		u.square = (i * 2) % 64
		u.sideCode = 1 + i/ownUnits
		u.rankCode = 1 + i%6
		u.cargoCount = i % 3
		u.guardedBy = make([]uint8, relationSize)
		u.threatenedBy = make([]uint8, (i%relationSize)+1)
		for j := range u.guardedBy {
			u.guardedBy[j] = uint8((u.square + j*7) % 64)
		}
		for j := range u.threatenedBy {
			u.threatenedBy[j] = uint8((u.square + j*11) % 64)
		}
		u.destinations = make([]uint8, destPerUnit)
		u.patrolGain = make([]int, destPerUnit)
		for j := range u.destinations {
			u.destinations[j] = uint8((u.square + 3 + j*5) % 64)
			u.patrolGain[j] = (i + j) % 4
		}
	}
	return units
}

// ---------------------------------------------------------------------------
// Variant A: today's JSON + dict wire.
// ---------------------------------------------------------------------------

func fixtureJSON(units []unitFixture) string {
	pieces := map[string]any{}
	legal := map[string][]string{}
	candidates := map[string]map[string]any{}
	for i := range units {
		u := &units[i]
		sq := botvalue.SquareName(u.square)
		guarded := make([]string, len(u.guardedBy))
		for j, s := range u.guardedBy {
			guarded[j] = botvalue.SquareName(int(s))
		}
		threatened := make([]string, len(u.threatenedBy))
		for j, s := range u.threatenedBy {
			threatened[j] = botvalue.SquareName(int(s))
		}
		pieces[sq] = map[string]any{
			"unitId": i + 1, "side": sideVocab.Name(u.sideCode), "rank": rankNames[u.rankCode],
			"convoy": false, "cargoCount": u.cargoCount, "kingCargo": false,
			"ghost": false, "refitting": false,
			"guardedBy": guarded, "threatenedBy": threatened,
		}
		if u.sideCode != 1 {
			continue
		}
		dests := make([]string, len(u.destinations))
		byDest := map[string]any{}
		for j, d := range u.destinations {
			dests[j] = botvalue.SquareName(int(d))
			byDest[dests[j]] = map[string]any{
				"destinationVisible": true, "patrolGain": u.patrolGain[j],
			}
		}
		legal[sq] = dests
		candidates[sq] = byDest
	}
	payload := map[string]any{
		"side": "white", "pieces": pieces, "legal": legal, "candidates": candidates,
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		panic(err)
	}
	return string(raw)
}

// scriptA reproduces the production script's own idioms verbatim in shape:
// piece_squares() sorting decoded keys, projected_cell()'s dict(raw) copy,
// build_board()'s square-keyed dicts, relation_squares()/guarded_count()
// helper calls, .get() with a fresh {} default, and square[0]/square[1]
// parsing of algebraic names.
const scriptA = `
FILES = "abcdefgh"

def square_file(square):
    return FILES.index(square[0])

def square_rank_number(square):
    return int(square[1])

def square_index(square):
    return square_file(square) * 8 + (square_rank_number(square) - 1)

def relation_squares(subject, relation):
    return subject.get(relation, []) or []

def guarded_count(subject):
    return len(relation_squares(subject, "guardedBy"))

def threatened_count(subject):
    return len(relation_squares(subject, "threatenedBy"))

def projected_cell(observation, square):
    raw = observation["pieces"].get(square)
    if raw == None:
        return None
    cell = dict(raw)
    cell["square"] = square
    return cell

def piece_squares(observation):
    return sorted(observation["pieces"].keys(), key = square_index)

def build_board(observation):
    own_cells = []
    enemy_cells = []
    own_by_square = {}
    enemy_by_square = {}
    all_by_square = {}
    for square in piece_squares(observation):
        cell = projected_cell(observation, square)
        all_by_square[square] = cell
        if cell["side"] == observation["side"]:
            own_cells.append(cell)
            own_by_square[square] = cell
        else:
            enemy_cells.append(cell)
            enemy_by_square[square] = cell
    return {
        "own": own_cells,
        "enemy": enemy_cells,
        "own_by_square": own_by_square,
        "enemy_by_square": enemy_by_square,
        "all_by_square": all_by_square,
    }

def candidate_at(observation, source_square, destination):
    return observation.get("candidates", {}).get(source_square, {}).get(destination, {})

MATERIAL = {"pawn": 1.0, "knight": 3.0, "bishop": 3.25, "rook": 5.0, "queen": 9.0, "king": 60.0}

def decide(observation, memory, params, host_random_draw, options):
    board = build_board(observation)
    total = 0.0
    for cell in board["own"]:
        square = cell["square"]
        for destination in observation["legal"].get(square, []):
            candidate = candidate_at(observation, square, destination)
            total += MATERIAL.get(cell["rank"], 0.0)
            total += 0.1 * guarded_count(cell)
            total -= 0.2 * threatened_count(cell)
            total += 0.01 * (square_file(destination) - square_file(square))
            total += 0.5 * candidate.get("patrolGain", 0)
    return total, {}, []
`

// ---------------------------------------------------------------------------
// Variants B and C: Go-backed rows.
// ---------------------------------------------------------------------------

// Field indices, shared by the schema declaration and the host switch.
const (
	fSquare = iota
	fSquareFile
	fSide
	fRank
	fCargoCount
	fDefenders
	fDefending
	fAttackers
	fAttacking
	fDefendersCount
	fAttackersCount
	fDestinationFiles
	fPatrolGains
	fIsOwn
)

func unitSchema() *botvalue.Schema {
	return botvalue.NewSchema("unit",
		botvalue.FieldSpec{Name: "square", Kind: botvalue.KindSquare},
		botvalue.FieldSpec{Name: "square_file", Kind: botvalue.KindInt},
		botvalue.FieldSpec{Name: "side", Kind: botvalue.KindEnum, Vocab: sideVocab},
		botvalue.FieldSpec{Name: "rank", Kind: botvalue.KindEnum, Vocab: rankVocab},
		botvalue.FieldSpec{Name: "cargo_count", Kind: botvalue.KindInt},
		botvalue.FieldSpec{Name: "defenders", Relation: true},
		botvalue.FieldSpec{Name: "defending", Relation: true},
		botvalue.FieldSpec{Name: "attackers", Relation: true},
		botvalue.FieldSpec{Name: "attacking", Relation: true},
		botvalue.FieldSpec{Name: "defenders_count", Kind: botvalue.KindInt},
		botvalue.FieldSpec{Name: "attackers_count", Kind: botvalue.KindInt},
		botvalue.FieldSpec{Name: "destination_files", RepeatedInts: true},
		botvalue.FieldSpec{Name: "patrol_gains", RepeatedInts: true},
		botvalue.FieldSpec{Name: "is_own", Kind: botvalue.KindBool},
	)
}

// unitSource is the ENGINE side: it names no Starlark type and answers one
// field at a time, straight out of host memory, on demand.
type unitSource struct {
	units []unitFixture
	files [][]int32
	gains [][]int32
}

func newUnitSource(units []unitFixture) *unitSource {
	s := &unitSource{units: units, files: make([][]int32, len(units)), gains: make([][]int32, len(units))}
	for i := range units {
		u := &units[i]
		files := make([]int32, len(u.destinations))
		gains := make([]int32, len(u.patrolGain))
		for j, d := range u.destinations {
			files[j] = int32(int(d) / 8)
			gains[j] = int32(u.patrolGain[j])
		}
		s.files[i], s.gains[i] = files, gains
	}
	return s
}

func (s *unitSource) Len() int { return len(s.units) }

func (s *unitSource) Field(row, field int) botvalue.Scalar {
	u := &s.units[row]
	switch field {
	case fSquare:
		return botvalue.SquareScalar(u.square)
	case fSquareFile:
		return botvalue.IntScalar(int64(u.square / 8))
	case fSide:
		return botvalue.EnumScalar(u.sideCode)
	case fRank:
		return botvalue.EnumScalar(u.rankCode)
	case fCargoCount:
		return botvalue.IntScalar(int64(u.cargoCount))
	case fDefendersCount:
		return botvalue.IntScalar(int64(len(u.guardedBy)))
	case fAttackersCount:
		return botvalue.IntScalar(int64(len(u.threatenedBy)))
	case fIsOwn:
		return botvalue.BoolScalar(u.sideCode == 1)
	}
	return botvalue.NoneScalar()
}

func (s *unitSource) Squares(row, field int) []uint8 { return nil }

// Mask answers the relation fields as 32-bit sets over UnitIndex.
func (s *unitSource) Mask(row, field int) uint32 {
	u := &s.units[row]
	var m uint32
	switch botvalue.Relation(field - fDefenders) {
	case botvalue.Defenders:
		for _, sq := range u.guardedBy {
			m |= 1 << uint(int(sq)%32)
		}
	case botvalue.Attackers:
		for _, sq := range u.threatenedBy {
			m |= 1 << uint(int(sq)%32)
		}
	case botvalue.Defending, botvalue.Attacking:
		// The fixture models only the incoming pair; the outgoing pair is
		// declared so the schema carries all four directions.
	}
	return m
}

func (s *unitSource) Ints(row, field int) []int32 {
	if s.units[row].sideCode != 1 {
		return nil
	}
	switch field {
	case fDestinationFiles:
		return s.files[row]
	case fPatrolGains:
		return s.gains[row]
	}
	return nil
}

const materialConst = `MATERIAL = {"pawn": 1.0, "knight": 3.0, "bishop": 3.25, "rook": 5.0, "queen": 9.0, "king": 60.0}`

// scriptB: native values, but the script still derives the counts itself
// through a helper call, exactly as chess-raiders-bot.star does today.
const scriptB = `
def relation_count(unit, which):
    if which == "defenders":
        return unit.defenders.count
    return unit.attackers.count

` + materialConst + `

def decide(observation, memory, params, host_random_draw, options):
    total = 0.0
    for unit in observation.units:
        if not unit.is_own:
            continue
        index = 0
        for destination_file in unit.destination_files:
            total += MATERIAL.get(unit.rank, 0.0)
            total += 0.1 * relation_count(unit, "defenders")
            total -= 0.2 * relation_count(unit, "attackers")
            total += 0.01 * (destination_file - unit.square_file)
            total += 0.5 * unit.patrol_gains[index]
            index += 1
    return total, {}, []
`

// scriptC: the host pre-answers the derived questions, so the script makes
// no helper call at all.
const scriptC = `
` + materialConst + `

def decide(observation, memory, params, host_random_draw, options):
    total = 0.0
    for unit in observation.units:
        if not unit.is_own:
            continue
        base = MATERIAL.get(unit.rank, 0.0) + 0.1 * unit.defenders_count - 0.2 * unit.attackers_count
        index = 0
        for destination_file in unit.destination_files:
            total += base + 0.01 * (destination_file - unit.square_file)
            total += 0.5 * unit.patrol_gains[index]
            index += 1
    return total, {}, []
`

// expectedScore computes the answer in Go so every variant is checked
// against an independent third implementation, not against each other.
func expectedScore(units []unitFixture) float64 {
	total := 0.0
	for i := range units {
		u := &units[i]
		if u.sideCode != 1 {
			continue
		}
		for j, d := range u.destinations {
			total += rankMaterial[rankNames[u.rankCode]]
			total += 0.1 * float64(len(u.guardedBy))
			total -= 0.2 * float64(len(u.threatenedBy))
			total += 0.01 * (float64(int(d)/8) - float64(u.square/8))
			total += 0.5 * float64(u.patrolGain[j])
		}
	}
	return total
}

func TestVariantsAgree(t *testing.T) {
	units := newFixture()
	want := expectedScore(units)
	if got := runVariantA(t, units); math.Abs(got-want) > 1e-9 {
		t.Fatalf("variant A score = %v, want %v", got, want)
	}
	h := newNativeHost(units)
	for name, source := range map[string]string{"B": scriptB, "C": scriptC} {
		got, err := h.call(source)
		if err != nil {
			t.Fatalf("variant %s: %v", name, err)
		}
		if math.Abs(got-want) > 1e-9 {
			t.Fatalf("variant %s score = %v, want %v", name, got, want)
		}
	}
	t.Logf("variants A, B and C all agree on %v", want)
}

func runVariantA(t testing.TB, units []unitFixture) float64 {
	p, err := runtime.Compile(scriptA)
	if err != nil {
		t.Fatalf("compile A: %v", err)
	}
	out, err := p.Call("decide", fixtureJSON(units), "{}", "{}", "0", "0")
	if err != nil {
		t.Fatalf("call A: %v", err)
	}
	var triple []json.RawMessage
	if err := json.Unmarshal([]byte(out), &triple); err != nil {
		t.Fatalf("decode A: %v", err)
	}
	var score float64
	if err := json.Unmarshal(triple[0], &score); err != nil {
		t.Fatalf("decode A score: %v", err)
	}
	return score
}

func BenchmarkShapeADictWire(b *testing.B) {
	units := newFixture()
	payload := fixtureJSON(units)
	p, err := runtime.Compile(scriptA)
	if err != nil {
		b.Fatal(err)
	}
	if _, err := p.Call("decide", payload, "{}", "{}", "0", "0"); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := p.Call("decide", payload, "{}", "{}", "0", "0"); err != nil {
			b.Fatal(err)
		}
	}
}

func benchNative(b *testing.B, source string) {
	units := newFixture()
	h := newNativeHost(units)
	if _, err := h.call(source); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := h.call(source); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkShapeBNativeDerived(b *testing.B)     { benchNative(b, scriptB) }
func BenchmarkShapeCNativePrecomputed(b *testing.B) { benchNative(b, scriptC) }

// ---------------------------------------------------------------------------
// The native host: one Session, pre-allocated rows, rebound per decision.
// ---------------------------------------------------------------------------

type nativeHost struct {
	sess     *botvalue.Session
	array    *botvalue.RowArray
	obs      *observationValue
	programs map[string]*runtime.Program
}

func newNativeHost(units []unitFixture) *nativeHost {
	sess := &botvalue.Session{}
	array := botvalue.NewRowArray(unitSchema(), newUnitSource(units), sess, len(units))
	h := &nativeHost{sess: sess, array: array, programs: map[string]*runtime.Program{}}
	h.obs = &observationValue{units: array}
	return h
}

func (h *nativeHost) call(source string) (float64, error) {
	p, ok := h.programs[source]
	if !ok {
		var err error
		if p, err = runtime.Compile(source); err != nil {
			return 0, err
		}
		h.programs[source] = p
	}
	gen := h.sess.Begin()
	h.array.Rebind(gen)
	defer h.sess.End()
	result, err := p.CallValues("decide", h.obs, botvalue.None, botvalue.None, botvalue.None, botvalue.None)
	if err != nil {
		return 0, err
	}
	first, ok := botvalue.TupleAt(result, 0)
	if !ok {
		return 0, fmt.Errorf("unexpected result %v", result)
	}
	score, ok := botvalue.AsFloat(first)
	if !ok {
		return 0, fmt.Errorf("first return is not numeric")
	}
	return score, nil
}

// observationValue is the top-level Go-backed observation: attributes only,
// no dictionary, no mutation interface.
type observationValue struct {
	units *botvalue.RowArray
}

func (o *observationValue) String() string       { return "<observation>" }
func (o *observationValue) Type() string         { return "observation" }
func (o *observationValue) Freeze()              {}
func (o *observationValue) Truth() starlark.Bool { return starlark.True }
func (o *observationValue) Hash() (uint32, error) {
	return 0, fmt.Errorf("unhashable type: observation")
}
func (o *observationValue) AttrNames() []string { return []string{"units"} }
func (o *observationValue) Attr(name string) (starlark.Value, error) {
	if name == "units" {
		return o.units, nil
	}
	return nil, nil
}
