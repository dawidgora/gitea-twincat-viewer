// Command gitea-twincat-viewer renders Beckhoff TwinCAT XML files
// (`.TcPOU`, `.TcDUT`, `.TcGVL`) into HTML.
//
// Two invocation modes are supported:
//
//  1. Direct: `gitea-twincat-viewer <file>` writes a complete HTML
//     document to stdout. This is the form Gitea uses when invoking
//     an external renderer: the file path is passed as the only
//     positional argument.
//
//  2. Sub-command: `gitea-twincat-viewer render <file>` for manual
//     debugging from the shell. `gitea-twincat-viewer version`
//     prints the version. `gitea-twincat-viewer help` prints usage.
//
// On any failure the binary exits non-zero and writes a short
// diagnostic to stderr.
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/dawidgora/gitea-twincat-viewer/internal/parser"
	"github.com/dawidgora/gitea-twincat-viewer/internal/renderer"
)

// version is set at build time via -ldflags.
var version = "dev"

// Exit codes - chosen to match common CLI conventions and to keep
// Gitea's renderer pipeline happy (any non-zero causes fallback).
const (
	exitOK    = 0
	exitUsage = 2
	exitIO    = 3
	exitParse = 4
)

func main() {
	if len(os.Args) < 2 {
		usage(os.Stderr)
		os.Exit(exitUsage)
	}

	if !isFlag(os.Args[1]) && !isCommand(os.Args[1]) {
		if err := runRender(os.Args[1:], os.Stdout); err != nil {
			fmt.Fprintf(os.Stderr, "render: %v\n", err)
			os.Exit(exitFor(err))
		}
		return
	}

	switch os.Args[1] {
	case "render":
		if len(os.Args) < 3 {
			usage(os.Stderr)
			os.Exit(exitUsage)
		}
		if err := runRender(os.Args[2:], os.Stdout); err != nil {
			fmt.Fprintf(os.Stderr, "render: %v\n", err)
			os.Exit(exitFor(err))
		}
	case "version", "--version", "-v":
		fmt.Printf("gitea-twincat-viewer %s\n", version)
	case "help", "-h", "--help":
		usage(os.Stdout)
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		usage(os.Stderr)
		os.Exit(exitUsage)
	}
}

// isCommand reports whether the argument is a recognised sub-command
// name (render, version, help). We use this to disambiguate between
// `gitea-twincat-viewer render` (sub-command) and
// `gitea-twincat-viewer render.TcPOU` (file path that happens to
// start with "render").
func isCommand(arg string) bool {
	switch arg {
	case "render", "version", "help":
		return true
	}
	return false
}

// isFlag reports whether the argument looks like a CLI flag.
func isFlag(arg string) bool {
	return len(arg) > 1 && arg[0] == '-'
}

// usage prints CLI usage.
func usage(w io.Writer) {
	fmt.Fprintln(w, `gitea-twincat-viewer - render Beckhoff TwinCAT files for Gitea

Usage:
  gitea-twincat-viewer <file>          # direct (used by Gitea)
  gitea-twincat-viewer render <file>   # explicit sub-command
  gitea-twincat-viewer version

Reads the TwinCAT XML file at <file>, parses it, and writes a
complete HTML document to stdout. On any failure it writes a
diagnostic to stderr and exits non-zero.`)
}

// exitFor maps an error to a stable exit code so external tooling
// (Gitea's renderer pipeline, CI logs) can distinguish failure modes.
func exitFor(err error) int {
	switch {
	case isUsageErr(err):
		return exitUsage
	case isIOErr(err):
		return exitIO
	default:
		return exitParse
	}
}

type usageErr struct{ err error }

func (e *usageErr) Error() string { return e.err.Error() }
func (e *usageErr) Unwrap() error { return e.err }

type ioErr struct{ err error }

func (e *ioErr) Error() string { return e.err.Error() }
func (e *ioErr) Unwrap() error { return e.err }

func isUsageErr(err error) bool { _, ok := err.(*usageErr); return ok }
func isIOErr(err error) bool    { _, ok := err.(*ioErr); return ok }

// runRender is split out from main so it can be exercised by tests.
func runRender(args []string, stdout io.Writer) error {
	if len(args) != 1 {
		return &usageErr{err: fmt.Errorf("render expects exactly one argument (file path)")}
	}
	path := args[0]

	info, err := os.Stat(path)
	if err != nil {
		return &ioErr{err: fmt.Errorf("stat: %w", err)}
	}
	if info.IsDir() {
		return &ioErr{err: fmt.Errorf("%s is a directory", path)}
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return &ioErr{err: fmt.Errorf("read: %w", err)}
	}

	ext := filepath.Ext(path)
	f, err := parser.Parse(data, ext)
	if err != nil {
		return err
	}

	html := renderer.Render(f, renderer.Options{Title: f.Name})
	_, err = io.WriteString(stdout, html)
	return err
}
