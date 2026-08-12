// Copyright 2026 Sneat.app

package manifest

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"reflect"
	"sort"
	"strings"

	"github.com/sneat-games/chessraiders/go/bots/runtime"
)

const gitLFSPointerPrefix = "version https://git-lfs.github.com/spec/v1"

// ValidateArtifact parses manifestBytes, verifies the supplied materialised
// tree is exactly its declared executable closure, and returns the stable
// content digest plus the selected complete parameter object. Callers retain
// only the returned values plus their own immutable storage reference; they
// never execute a live repository ref or raw partial parameter set.
func ValidateArtifact(manifestBytes []byte, entries []TreeEntry, limits ValidationLimits, profile CompatibilityProfile) (Artifact, error) {
	plan, err := ValidateManifest(manifestBytes, limits, profile)
	if err != nil {
		return Artifact{}, err
	}
	validated, err := validateClosure(plan.Manifest, manifestBytes, entries, limits)
	if err != nil {
		return Artifact{}, err
	}
	return Artifact{
		Manifest:           plan.Manifest,
		Digest:             validated.Digest,
		ResolvedParameters: append([]byte(nil), validated.ResolvedParameters...),
	}, nil
}

// ValidateManifest performs the bounded first stage of artifact admission. It
// reads no script, schema or configuration bytes and grants no execution by
// itself; it returns only the exact canonical closure plan a host may fetch for
// a later ValidateArtifact call.
func ValidateManifest(manifestBytes []byte, limits ValidationLimits, profile CompatibilityProfile) (ClosurePlan, error) {
	if err := validateLimits(limits); err != nil {
		return ClosurePlan{}, err
	}
	if int64(len(manifestBytes)) > limits.MaxManifestBytes || int64(len(manifestBytes)) > limits.MaxFileBytes {
		return ClosurePlan{}, &Error{Code: ErrByteLimit, Path: ManifestPath, Detail: "exceeds the caller's raw manifest/file byte limit"}
	}
	parsed, err := parseWithDepth(manifestBytes, limits.MaxJSONDepth)
	if err != nil {
		return ClosurePlan{}, err
	}
	if err := ValidateCompatibility(parsed, profile); err != nil {
		return ClosurePlan{}, err
	}
	if len(parsed.Sources) > limits.MaxFiles {
		return ClosurePlan{}, &Error{Code: ErrFileLimit, Detail: "manifest declares more files than the caller permits"}
	}
	paths := append([]string(nil), parsed.Sources...)
	sort.Strings(paths)
	return ClosurePlan{Manifest: parsed, Paths: paths, Limits: limits}, nil
}

// ValidateCompatibility applies the game/provider's caller-owned exact
// support profile. Structural Parse alone never establishes execution
// eligibility.
func ValidateCompatibility(manifest Manifest, profile CompatibilityProfile) error {
	if profile.Game == "" || profile.Rules.Minimum == "" || profile.Rules.Maximum == "" || profile.Runtime.Minimum == "" || profile.Runtime.Maximum == "" || profile.ScriptProtocol == "" || len(profile.AllowedStateModes) == 0 {
		return &Error{Code: ErrUnsupportedProfile, Field: "profile", Detail: "must supply exact game, rules, runtime, script protocol and allowed state modes"}
	}
	if manifest.Compatibility.Game != profile.Game || manifest.Compatibility.Rules != profile.Rules || manifest.Compatibility.Runtime != profile.Runtime || manifest.Runtime.Protocol != profile.ScriptProtocol {
		return &Error{Code: ErrUnsupportedProfile, Field: "compatibility", Detail: "manifest game/rules/runtime/protocol does not exactly match the caller profile"}
	}
	if !containsStateMode(profile.AllowedStateModes, manifest.StateMode) {
		return &Error{Code: ErrUnsupportedProfile, Field: "stateMode", Detail: "is not allowed by the caller profile"}
	}
	allowed := make(map[string]struct{}, len(profile.AllowedCapabilities))
	for _, capability := range profile.AllowedCapabilities {
		allowed[capability] = struct{}{}
	}
	for _, capability := range manifest.RequiredCapabilities {
		if _, ok := allowed[capability]; !ok {
			return &Error{Code: ErrUnsupportedProfile, Field: "requiredCapabilities", Detail: "requires unsupported capability " + capability}
		}
	}
	return nil
}

func containsStateMode(modes []StateMode, wanted StateMode) bool {
	for _, mode := range modes {
		if mode == wanted {
			return true
		}
	}
	return false
}

// ValidateClosure verifies that entries contains exactly the manifest's
// declared regular files. It deliberately accepts resolver-provided tree
// metadata so symlinks and submodules cannot be made to look like ordinary
// source bytes before validation. It does not apply a CompatibilityProfile or
// return the selected runtime configuration; untrusted eligibility callers use
// ValidateArtifact.
func ValidateClosure(manifest Manifest, manifestBytes []byte, entries []TreeEntry, limits ValidationLimits) (Digest, error) {
	validated, err := validateClosure(manifest, manifestBytes, entries, limits)
	return validated.Digest, err
}

type closureValidation struct {
	Digest             Digest
	ResolvedParameters json.RawMessage
}

func validateClosure(manifest Manifest, manifestBytes []byte, entries []TreeEntry, limits ValidationLimits) (closureValidation, error) {
	if err := validateLimits(limits); err != nil {
		return closureValidation{}, err
	}
	if int64(len(manifestBytes)) > limits.MaxManifestBytes || int64(len(manifestBytes)) > limits.MaxFileBytes {
		return closureValidation{}, &Error{Code: ErrByteLimit, Path: ManifestPath, Detail: "exceeds the caller's raw manifest/file byte limit"}
	}
	parsed, err := parseWithDepth(manifestBytes, limits.MaxJSONDepth)
	if err != nil {
		return closureValidation{}, err
	}
	if !reflect.DeepEqual(parsed, manifest) {
		return closureValidation{}, &Error{Code: ErrInvalidManifest, Path: ManifestPath, Detail: "parsed manifest does not match the manifest argument"}
	}
	if err := Validate(manifest); err != nil {
		return closureValidation{}, err
	}
	if len(entries) > limits.MaxFiles {
		return closureValidation{}, &Error{Code: ErrFileLimit, Detail: "tree has more entries than the caller permits"}
	}

	declared := make(map[string]struct{}, len(manifest.Sources))
	for _, source := range manifest.Sources {
		declared[source] = struct{}{}
	}
	files := make(map[string][]byte, len(entries))
	var total int64
	for _, entry := range entries {
		canonical, err := canonicalPath(entry.Path)
		if err != nil {
			return closureValidation{}, pathError(entry.Path, err)
		}
		if canonical != entry.Path {
			return closureValidation{}, &Error{Code: ErrInvalidPath, Path: entry.Path, Detail: "is not canonical"}
		}
		if _, exists := files[canonical]; exists {
			return closureValidation{}, &Error{Code: ErrDuplicatePath, Path: canonical, Detail: "appears more than once in the tree"}
		}
		if entry.Kind != EntryKindRegular {
			return closureValidation{}, &Error{Code: ErrNonRegularFile, Path: canonical, Detail: "must be a regular file"}
		}
		if int64(len(entry.Content)) > limits.MaxFileBytes {
			return closureValidation{}, &Error{Code: ErrByteLimit, Path: canonical, Detail: "exceeds the caller's per-file byte limit"}
		}
		total += int64(len(entry.Content))
		if total > limits.MaxTotalBytes {
			return closureValidation{}, &Error{Code: ErrByteLimit, Detail: "exceeds the caller's total byte limit"}
		}
		if isLFSPointer(entry.Content) {
			return closureValidation{}, &Error{Code: ErrLFSPointer, Path: canonical, Detail: "Git LFS pointer files are not executable source"}
		}
		if _, ok := declared[canonical]; !ok {
			return closureValidation{}, &Error{Code: ErrUndeclaredFile, Path: canonical, Detail: "is not declared by sources"}
		}
		files[canonical] = append([]byte(nil), entry.Content...)
	}
	for declaredPath := range declared {
		content, ok := files[declaredPath]
		if !ok {
			return closureValidation{}, &Error{Code: ErrMissingFile, Path: declaredPath, Detail: "is declared but absent from the tree"}
		}
		if declaredPath == ManifestPath && string(content) != string(manifestBytes) {
			return closureValidation{}, &Error{Code: ErrInvalidManifest, Path: ManifestPath, Detail: "tree bytes differ from the parsed manifest bytes"}
		}
	}
	parameterLimits := ParameterLimits{
		MaxDocumentBytes: limits.MaxFileBytes,
		MaxJSONDepth:     limits.MaxJSONDepth,
		MaxProperties:    limits.MaxParameterProperties,
		MaxSets:          limits.MaxParameterSets,
		MaxResolvedBytes: limits.MaxResolvedParameterBytes,
	}
	parameterSchema, err := ParseParameterSchema(files[manifest.Parameters.SchemaPath], parameterLimits)
	if err != nil {
		return closureValidation{}, &Error{Code: ErrInvalidManifest, Path: manifest.Parameters.SchemaPath, Detail: "declared parameter schema is invalid: " + err.Error()}
	}
	resolved, err := resolveParameterSet(
		parameterSchema,
		files[manifest.Parameters.SetsPath],
		manifest.Parameters.DefaultSet,
		parameterLimits,
	)
	if err != nil {
		return closureValidation{}, &Error{Code: ErrInvalidManifest, Path: manifest.Parameters.SetsPath, Detail: "declared parameter configurations are invalid: " + err.Error()}
	}
	if err := runtime.ValidateEntrypoint(string(files[manifest.Runtime.Entrypoint]), "decide", 5); err != nil {
		return closureValidation{}, &Error{Code: ErrEntrypoint, Path: manifest.Runtime.Entrypoint, Detail: err.Error()}
	}
	return closureValidation{Digest: DigestClosure(files), ResolvedParameters: resolved}, nil
}

// DigestClosure deterministically hashes an already-validated regular-file
// closure. Its domain tag and explicit length prefixes remove ambiguity between
// adjacent path/content pairs; paths are sorted as raw canonical POSIX text.
func DigestClosure(files map[string][]byte) Digest {
	paths := make([]string, 0, len(files))
	for name := range files {
		paths = append(paths, name)
	}
	sort.Strings(paths)
	hash := sha256.New()
	writeLengthPrefixed(hash, []byte(ClosureDigestDomain))
	for _, name := range paths {
		writeLengthPrefixed(hash, []byte(name))
		writeLengthPrefixed(hash, files[name])
	}
	var digest Digest
	copy(digest[:], hash.Sum(nil))
	return digest
}

func writeLengthPrefixed(writer interface{ Write([]byte) (int, error) }, value []byte) {
	var length [8]byte
	binary.BigEndian.PutUint64(length[:], uint64(len(value)))
	_, _ = writer.Write(length[:])
	_, _ = writer.Write(value)
}

func isLFSPointer(content []byte) bool {
	return strings.HasPrefix(string(content), gitLFSPointerPrefix)
}

func validateLimits(limits ValidationLimits) error {
	if limits.MaxFiles <= 0 || limits.MaxFileBytes <= 0 || limits.MaxTotalBytes <= 0 || limits.MaxManifestBytes <= 0 || limits.MaxJSONDepth <= 0 || limits.MaxParameterProperties <= 0 || limits.MaxParameterSets <= 0 || limits.MaxResolvedParameterBytes <= 0 {
		return &Error{Code: ErrInvalidLimit, Detail: "caller must supply positive file, byte, raw-manifest, JSON-depth and parameter resolution limits"}
	}
	if limits.MaxFileBytes > limits.MaxTotalBytes {
		return &Error{Code: ErrInvalidLimit, Detail: "maximum file bytes cannot exceed maximum total bytes"}
	}
	return nil
}

func encodeHex(value []byte) string { return hex.EncodeToString(value) }
