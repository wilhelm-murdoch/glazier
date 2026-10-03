package harness

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wilhelm-murdoch/glazier/e2e/report"
)

func TestLoadBaseline(t *testing.T) {
	path := filepath.Join(t.TempDir(), "expected-failures.txt")
	content := strings.Join([]string{
		"# comment",
		"",
		"*        | TestA/x | check one | everywhere",
		"bookworm | TestA/y | check two | only bookworm",
		"alpine   | TestA/z | check three | only alpine",
	}, "\n")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	b, err := LoadBaseline(path, "bookworm")
	if err != nil {
		t.Fatal(err)
	}

	for id, want := range map[string]bool{
		report.ID("TestA/x", "check one"):   true,
		report.ID("TestA/y", "check two"):   true,
		report.ID("TestA/z", "check three"): false,
	} {
		if _, got := b.Expected(id); got != want {
			t.Errorf("%s: expected %v, want %v", id, got, want)
		}
	}
}

func TestLoadBaselineErrors(t *testing.T) {
	for name, line := range map[string]string{
		"three fields": "* | TestA/x | check",
		"no reason":    "* | TestA/x | check |  ",
	} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "expected-failures.txt")
			if err := os.WriteFile(path, []byte(line+"\n"), 0o600); err != nil {
				t.Fatal(err)
			}

			if _, err := LoadBaseline(path, "x"); err == nil {
				t.Error("no error")
			}
		})
	}
}

func TestMissingBaselineIsEmpty(t *testing.T) {
	b, err := LoadBaseline(filepath.Join(t.TempDir(), "none"), "x")
	if err != nil {
		t.Fatal(err)
	}

	if _, ok := b.Expected("anything"); ok {
		t.Error("an empty baseline expects a failure")
	}
}
