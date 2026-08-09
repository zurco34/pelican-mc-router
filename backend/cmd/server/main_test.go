package main

import (
	"bytes"
	"context"
	"errors"
	"log"
	"strings"
	"testing"
)

func TestRunLogsStableFailureMessage(t *testing.T) {
	secret := "https://panel.example.test/api-key=secret-value"
	var output bytes.Buffer
	logger := log.New(&output, "", 0)

	code := run(context.Background(), func(context.Context) error {
		return errors.New(secret)
	}, logger)

	if code != 1 {
		t.Fatalf("run() code = %d, want 1", code)
	}
	if strings.Contains(output.String(), secret) || strings.Contains(output.String(), "secret-value") {
		t.Fatalf("process log disclosed error: %q", output.String())
	}
	if !strings.Contains(output.String(), "operational failure") {
		t.Fatalf("process log = %q, want stable failure category", output.String())
	}
}

func TestRunReturnsSuccess(t *testing.T) {
	if code := run(context.Background(), func(context.Context) error { return nil }, log.New(&bytes.Buffer{}, "", 0)); code != 0 {
		t.Fatalf("run() code = %d, want 0", code)
	}
}
