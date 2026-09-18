package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRenderFixtures renders every TwinCAT fixture in
// examples/twincat and asserts that the output is well-formed and
// contains the expected sections. This is the closest thing we have
// to an end-to-end test that mirrors what Gitea will actually run.
func TestRenderFixtures(t *testing.T) {
	fixtures := []struct {
		path string
		must []string
	}{
		{
			path: "../../examples/twincat/FB_Boolean.TcPOU",
			must: []string{"FUNCTION_BLOCK", "bResult", "bInput"},
		},
		{
			path: "../../examples/twincat/FB_Integer.TcPOU",
			must: []string{"FUNCTION_BLOCK", "nResult", "nInput"},
		},
		{
			path: "../../examples/twincat/FB_Real.TcPOU",
			must: []string{"FUNCTION_BLOCK", "rResult", "rInput"},
		},
		{
			path: "../../examples/twincat/FB_String.TcPOU",
			must: []string{"FUNCTION_BLOCK", "sOutput", "sInput"},
		},
		{
			path: "../../examples/twincat/FB_Combined.TcPOU",
			must: []string{"FUNCTION_BLOCK", "PROPERTY", "Status", "Reset", "Apply"},
		},
		{
			path: "../../examples/twincat/ST_Record.TcDUT",
			must: []string{"STRUCT", "END_STRUCT"},
		},
		{
			path: "../../examples/twincat/GVL_Shared.TcGVL",
			must: []string{"VAR_GLOBAL", "bReady"},
		},
	}

	for _, f := range fixtures {
		abs, err := filepath.Abs(f.path)
		if err != nil {
			t.Fatalf("abs(%s): %v", f.path, err)
		}
		var stdout bytes.Buffer
		if err := runRender([]string{abs}, &stdout); err != nil {
			t.Fatalf("render %s: %v", abs, err)
		}
		out := stdout.String()
		for _, want := range f.must {
			if !strings.Contains(out, want) {
				t.Fatalf("render %s: missing %q in output", abs, want)
			}
		}
		// Basic safety: no raw <script> tag should ever appear.
		if strings.Contains(out, "<script") {
			t.Fatalf("render %s: output contains <script>", abs)
		}
	}
}

// TestRenderMissingFile verifies that a missing file returns an error
// with a non-zero exit class (caller maps it to IO exit).
func TestRenderMissingFile(t *testing.T) {
	err := runRender([]string{"/nonexistent/path.TcPOU"}, &bytes.Buffer{})
	if err == nil {
		t.Fatal("expected error for missing file")
	}
	if !isIOErr(err) {
		t.Fatalf("expected ioErr, got %T", err)
	}
}

// TestRenderDirectory verifies that a directory is rejected with an IO
// error rather than being treated as content.
func TestRenderDirectory(t *testing.T) {
	dir := t.TempDir()
	err := runRender([]string{dir}, &bytes.Buffer{})
	if err == nil {
		t.Fatal("expected error for directory")
	}
	if !isIOErr(err) {
		t.Fatalf("expected ioErr, got %T", err)
	}
}

// TestVersionCommand verifies the binary's --version handling without
// spawning a subprocess (we don't test exit codes here, only that the
// constant string is sensible).
func TestVersionString(t *testing.T) {
	if version == "" {
		t.Fatal("version should not be empty")
	}
}

// TestMainEntrypointHelp ensures the help text contains the render
// command so users get the right usage.
func TestMainEntrypointHelp(t *testing.T) {
	var buf bytes.Buffer
	usage(&buf)
	if !strings.Contains(buf.String(), "render <file>") {
		t.Fatalf("usage missing render example: %s", buf.String())
	}
	_ = os.Getenv // touch os so test compiles on stripped builds
}
