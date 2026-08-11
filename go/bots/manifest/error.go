// Copyright 2026 Sneat.app

package manifest

import "fmt"

type ErrorCode string

const (
	ErrInvalidJSON        ErrorCode = "invalid-json"
	ErrUnknownField       ErrorCode = "unknown-field"
	ErrInvalidManifest    ErrorCode = "invalid-manifest"
	ErrInvalidPath        ErrorCode = "invalid-path"
	ErrInvalidLimit       ErrorCode = "invalid-limit"
	ErrDuplicatePath      ErrorCode = "duplicate-path"
	ErrMissingFile        ErrorCode = "missing-file"
	ErrUndeclaredFile     ErrorCode = "undeclared-file"
	ErrNonRegularFile     ErrorCode = "non-regular-file"
	ErrLFSPointer         ErrorCode = "git-lfs-pointer"
	ErrFileLimit          ErrorCode = "file-limit"
	ErrByteLimit          ErrorCode = "byte-limit"
	ErrEntrypoint         ErrorCode = "invalid-entrypoint"
	ErrJSONDepth          ErrorCode = "json-depth"
	ErrUnsupportedProfile ErrorCode = "unsupported-profile"
)

// Error is a stable, caller-safe validation failure. Detail is suitable for a
// bot author, but never includes source bytes or credentials.
type Error struct {
	Code   ErrorCode
	Path   string
	Field  string
	Detail string
}

func (e *Error) Error() string {
	where := e.Path
	if where == "" {
		where = e.Field
	}
	if where == "" {
		return fmt.Sprintf("chess bot manifest: %s: %s", e.Code, e.Detail)
	}
	return fmt.Sprintf("chess bot manifest: %s at %s: %s", e.Code, where, e.Detail)
}
