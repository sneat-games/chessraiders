// Copyright 2026 Sneat.app

package botvalue

// THE CANONICAL RELATION VOCABULARY.
//
// This table is the record of what the four piece relations are called in the
// v2 script-facing protocol and what each one was called on the v1 JSON wire.
// It is code rather than a doc comment deliberately: every note, test name,
// spec reference and REQ: in both repositories still says `guardedBy` and
// `threatens`, so a maintainer reading Go needs somewhere authoritative to
// look, and a comment can drift from the schema it describes while a table the
// schema is BUILT FROM cannot.
//
// THE DEFECT THIS FIXES. The v1 names leaned on a `By` suffix to carry
// direction — `guardedBy` versus `guards` — and the script's own shorthand
// then dropped it entirely, leaving `guarded_count` with no way to tell which
// way it pointed. A reader could not answer "is this how many pieces defend
// this one, or how many it defends" without going to the source. The v2 names
// distinguish direction by WORD FORM instead: a noun for the pieces acting on
// this one, a participle for what this one is doing.
//
//	wire field (v1)   script name (v2)   meaning
//	guardedBy         defenders          friendlies defending this piece
//	guards            defending          friendlies this piece defends
//	threatenedBy      attackers          enemies attacking this piece
//	threatens         attacking          enemies this piece attacks
//
// Each is a UnitMask, so each carries `.count` and composes with the ordinary
// set operators. Both Cell and CandidateFact carry all four.
//
// NAMING RULE FOR EVERYTHING DOWNSTREAM: no identifier, doc comment or error
// message may say "guarded" or "threatened" without a direction. That is the
// exact ambiguity this vocabulary removes, and error text matters most —
// it is what a competitor author reads when their bot misbehaves.
type Relation uint8

const (
	// Defenders is the friendlies defending this piece (v1 wire: guardedBy).
	Defenders Relation = iota
	// Defending is the friendlies this piece defends (v1 wire: guards).
	Defending
	// Attackers is the enemies attacking this piece (v1 wire: threatenedBy).
	Attackers
	// Attacking is the enemies this piece attacks (v1 wire: threatens).
	Attacking
)

// RelationBinding is one row of the canonical table.
type RelationBinding struct {
	Relation Relation
	// Name is the attribute a script reads.
	Name string
	// WireV1 is the JSON field this replaces, kept so a maintainer reading an
	// older note, test or REQ: can find the current name without guessing.
	WireV1 string
	// Incoming reports whether the relation names pieces acting ON this one
	// (true) or pieces this one acts on (false). It is the property the v1
	// names expressed with a `By` suffix and the v2 names express with word
	// form.
	Incoming bool
	// Meaning is one sentence, used verbatim in generated protocol docs.
	Meaning string
}

// Relations is the complete, ordered table. Index by Relation.
var Relations = [4]RelationBinding{
	Defenders: {Defenders, "defenders", "guardedBy", true,
		"friendlies defending this piece"},
	Defending: {Defending, "defending", "guards", false,
		"friendlies this piece defends"},
	Attackers: {Attackers, "attackers", "threatenedBy", true,
		"enemies attacking this piece"},
	Attacking: {Attacking, "attacking", "threatens", false,
		"enemies this piece attacks"},
}

// Name returns the script-facing attribute name for a relation.
func (r Relation) Name() string {
	if int(r) >= len(Relations) {
		return ""
	}
	return Relations[r].Name
}

// String makes a Relation print as its script-facing name, so an error message
// cannot accidentally quote an ordinal or a stale wire field.
func (r Relation) String() string { return r.Name() }

// RelationByWireV1 resolves a v1 JSON field name to its v2 relation. It exists
// for migration tooling and for a maintainer reading an old spec reference,
// not for any runtime path.
func RelationByWireV1(field string) (Relation, bool) {
	for _, binding := range Relations {
		if binding.WireV1 == field {
			return binding.Relation, true
		}
	}
	return 0, false
}

// RelationFields builds the four FieldSpecs for one row kind, in canonical
// order, so a schema cannot declare them under ad-hoc names, leave one out, or
// order them differently between two row kinds. A caller appends these at a
// known offset and recovers the Relation in its Source with
// Relation(field - offset).
func RelationFields() []FieldSpec {
	fields := make([]FieldSpec, 0, len(Relations))
	for _, binding := range Relations {
		fields = append(fields, FieldSpec{Name: binding.Name, Relation: true})
	}
	return fields
}
