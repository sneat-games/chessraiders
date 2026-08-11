// Copyright 2026 Sneat.app

// Package manifest defines the portable, game-owned source artifact contract
// for a Chess Raiders bot. It intentionally contains no GitHub, Competios,
// account, token, Cup, or provider-profile concepts.
package manifest

import "encoding/json"

const (
	// SchemaVersion identifies this JSON document shape, not a game release.
	SchemaVersion = "chess-raiders-bot-manifest/v1"

	// ManifestPath is the mandatory root-relative file that carries a bot
	// manifest in an executable source closure.
	ManifestPath = "chess-raiders-bot.json"

	// ClosureDigestDomain separates a source-closure digest from hashes used
	// for any other Chess Raiders object.
	ClosureDigestDomain = "chess-raiders-bot-closure/v1"
)

type StateMode string

const (
	StateModeStateless      StateMode = "stateless"
	StateModeBattleStateful StateMode = "battle-stateful"
)

// Manifest is deliberately small. The host decides current resource limits
// and official provider policy; this document declares only what one bot
// source artifact needs in order to be validated and executed.
type Manifest struct {
	Schema               string        `json:"schema"`
	Display              Display       `json:"display"`
	Author               Attribution   `json:"author"`
	Source               Source        `json:"source"`
	License              License       `json:"license"`
	Compatibility        Compatibility `json:"compatibility"`
	Runtime              Runtime       `json:"runtime"`
	StateMode            StateMode     `json:"stateMode"`
	Parameters           Parameters    `json:"parameters"`
	RequiredCapabilities []string      `json:"requiredCapabilities"`
	Sources              []string      `json:"sources"`
}

type Display struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// Attribution identifies the human or organisation to credit. URL is
// optional because an offline source artifact must remain valid.
type Attribution struct {
	Name string `json:"name"`
	URL  string `json:"url,omitempty"`
}

// Source is provenance useful to public display and validation, not a live
// repository fetch instruction. The retained closure is always authoritative.
type Source struct {
	Repository  string `json:"repository"`
	Attribution string `json:"attribution"`
}

type License struct {
	SPDX string `json:"spdx"`
}

// Compatibility gives a game-owned, qualified range for the rules and
// runtime protocol. The validator treats these as opaque non-empty protocol
// identifiers; negotiating them belongs to the game provider.
type Compatibility struct {
	Game    string `json:"game"`
	Rules   Range  `json:"rules"`
	Runtime Range  `json:"runtime"`
}

type Range struct {
	Minimum string `json:"minimum"`
	Maximum string `json:"maximum"`
}

// Runtime names the only executable entry point. Protocol is deliberately
// qualified so a future incompatible bot wire contract cannot look like an
// ordinary source update.
type Runtime struct {
	Kind       string `json:"kind"`
	Protocol   string `json:"protocol"`
	Entrypoint string `json:"entrypoint"`
}

// Parameters describes the inert configuration file handed to a script. The
// host never executes parameter values as source.
type Parameters struct {
	Path         string                 `json:"path"`
	Declarations []ParameterDeclaration `json:"declarations"`
}

type ParameterDeclaration struct {
	Name    string          `json:"name"`
	Type    ParameterType   `json:"type"`
	Default json.RawMessage `json:"default"`
	Range   *NumericRange   `json:"range,omitempty"`
}

type ParameterType string

const (
	ParameterTypeNumber  ParameterType = "number"
	ParameterTypeInteger ParameterType = "integer"
	ParameterTypeBoolean ParameterType = "boolean"
	ParameterTypeString  ParameterType = "string"
	ParameterTypeJSON    ParameterType = "json"
)

type NumericRange struct {
	Minimum json.Number `json:"minimum"`
	Maximum json.Number `json:"maximum"`
}

// ValidationLimits are supplied by the caller. This package deliberately
// owns no Season 1 resource-limit numbers.
type ValidationLimits struct {
	MaxFiles         int
	MaxFileBytes     int64
	MaxTotalBytes    int64
	MaxManifestBytes int64
	MaxJSONDepth     int
}

// CompatibilityProfile is caller-supplied execution policy. The portable
// manifest package never embeds a Cup, Season, provider, or resource policy.
type CompatibilityProfile struct {
	Game                string
	Rules               Range
	Runtime             Range
	ScriptProtocol      string
	AllowedStateModes   []StateMode
	AllowedCapabilities []string
}

type EntryKind string

const (
	EntryKindRegular   EntryKind = "regular"
	EntryKindSymlink   EntryKind = "symlink"
	EntryKindSubmodule EntryKind = "submodule"
)

// TreeEntry is a materialised source-tree entry. A resolver must preserve
// kind rather than flatten a symlink/submodule into bytes, otherwise a
// validator could not reject it.
type TreeEntry struct {
	Path    string
	Kind    EntryKind
	Content []byte
}

type Artifact struct {
	Manifest Manifest
	Digest   Digest
}

type Digest [32]byte

func (d Digest) String() string { return "sha256:" + encodeHex(d[:]) }
