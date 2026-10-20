// Copyright 2026 Sneat.app

package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestRunSuccess(t *testing.T) {
	tmpDir := t.TempDir()
	origWrite := writeOutputFile
	defer func() { writeOutputFile = origWrite }()

	outPath := filepath.Join(tmpDir, "params.resolved.json")
	writeOutputFile = func(filename string, data []byte, perm os.FileMode) error {
		return origWrite(outPath, data, perm)
	}

	if err := run(); err != nil {
		t.Fatalf("run() failed: %v", err)
	}

	if _, err := os.Stat(outPath); err != nil {
		t.Fatalf("expected output file %s: %v", outPath, err)
	}
}

func TestRunMarshalError(t *testing.T) {
	origMarshal := marshalParams
	defer func() { marshalParams = origMarshal }()

	marshalParams = func() ([]byte, error) {
		return nil, errors.New("marshal error")
	}

	if err := run(); err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestMainSuccess(t *testing.T) {
	origWrite := writeOutputFile
	origExit := exitFunc
	defer func() {
		writeOutputFile = origWrite
		exitFunc = origExit
	}()

	writeOutputFile = func(string, []byte, os.FileMode) error {
		return nil
	}
	exitFunc = func(int) {
		t.Fatal("exitFunc called on success")
	}

	main()
}

func TestMainError(t *testing.T) {
	origMarshal := marshalParams
	origExit := exitFunc
	origStderr := stderrWriter
	defer func() {
		marshalParams = origMarshal
		exitFunc = origExit
		stderrWriter = origStderr
	}()

	var buf bytes.Buffer
	stderrWriter = &buf

	marshalParams = func() ([]byte, error) {
		return nil, errors.New("simulated failure")
	}

	exitCalled := false
	var exitCode int
	exitFunc = func(code int) {
		exitCalled = true
		exitCode = code
	}

	main()

	if !exitCalled || exitCode != 1 {
		t.Fatalf("expected exit(1), got called=%v code=%d", exitCalled, exitCode)
	}
	if !bytes.Contains(buf.Bytes(), []byte("simulated failure")) {
		t.Fatalf("expected stderr output, got: %s", buf.String())
	}
}
