package logging

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// useStdioMode switches logging into stdio mode for the duration of a test and
// restores the process-wide zerolog state afterwards.
func useStdioMode(t *testing.T, output io.Writer) {
	t.Helper()

	previousLogger, previousLevel := log.Logger, zerolog.GlobalLevel()
	t.Cleanup(func() {
		log.Logger = previousLogger
		zerolog.SetGlobalLevel(previousLevel)
		SetStdioMode(false, nil)
	})

	SetStdioMode(true, output)
}

func TestStdioModeKeepsWarningsAndErrorsOnStderr(t *testing.T) {
	var output bytes.Buffer
	useStdioMode(t, &output)

	Info("info message is suppressed")
	Warn("warning message is visible")
	Error("error message is visible")

	got := output.String()
	if strings.Contains(got, "info message is suppressed") {
		t.Fatalf("expected Info to be suppressed in stdio mode, got %q", got)
	}
	if !strings.Contains(got, "warning message is visible") {
		t.Fatalf("expected Warn on stderr in stdio mode, got %q", got)
	}
	if !strings.Contains(got, "error message is visible") {
		t.Fatalf("expected Error on stderr in stdio mode, got %q", got)
	}
}

func TestStdioModeIgnoresInitialize(t *testing.T) {
	var output bytes.Buffer
	useStdioMode(t, &output)

	var other bytes.Buffer
	Initialize(6, &other)
	Debug("debug message is suppressed")
	Warn("warning after initialize")

	if other.Len() != 0 {
		t.Fatalf("expected Initialize to be a no-op in stdio mode, got %q", other.String())
	}
	if strings.Contains(output.String(), "debug message is suppressed") {
		t.Fatalf("expected Debug to be suppressed in stdio mode, got %q", output.String())
	}
	if !strings.Contains(output.String(), "warning after initialize") {
		t.Fatalf("expected Warn to keep using the stdio sink, got %q", output.String())
	}
}

func TestStdioModeDisabledAllowsInitialize(t *testing.T) {
	var previousSink bytes.Buffer
	useStdioMode(t, &previousSink)

	SetStdioMode(false, nil)

	var output bytes.Buffer
	Initialize(5, &output)
	Info("info message is visible")

	if !strings.Contains(output.String(), "info message is visible") {
		t.Fatalf("expected Initialize to configure the logger after stdio mode is disabled, got %q", output.String())
	}
}
