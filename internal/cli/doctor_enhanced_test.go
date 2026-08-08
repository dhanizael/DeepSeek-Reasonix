package cli

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"reasonix/internal/enhancedmetrics"
)

func TestDoctorEnhancedCommand(t *testing.T) {
	enhancedmetrics.Reset()
	t.Cleanup(enhancedmetrics.Reset)

	// Capture stdout.
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	rc := doctorEnhancedCommand(nil, "test-ver")
	_ = w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	_ = r.Close()

	if rc != 0 {
		t.Fatalf("rc = %d", rc)
	}
	out := buf.String()
	if !strings.Contains(out, "doctor enhanced") {
		t.Fatalf("output missing header: %s", out)
	}
	if !strings.Contains(out, "silent_pass") && !strings.Contains(out, "harness") {
		t.Fatalf("output missing harness knobs: %s", out)
	}
}

func TestStatusShowsEnhancedWhenActive(t *testing.T) {
	enhancedmetrics.Reset()
	t.Cleanup(enhancedmetrics.Reset)
	enhancedmetrics.RecordASTReject("x.go")
	enhancedmetrics.RecordHarnessSilentPass("go test .")

	m := newTestChatTUI()
	m.runSlashCommand("/status")
	out := strings.Join(m.transcript, "\n")
	if !strings.Contains(out, "enhanced") {
		t.Fatalf("/status missing enhanced line:\n%s", out)
	}
	if !strings.Contains(out, "ast_rej=1") {
		t.Fatalf("/status missing compact metrics:\n%s", out)
	}
}
