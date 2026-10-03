package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// testHeader is a line of go test -v that names the test of the next lines;
// testMessage is a line that a test logged.
var (
	testHeader  = regexp.MustCompile(`^=== (?:RUN|CONT|NAME|PAUSE)\s+(\S+)`)
	testMessage = regexp.MustCompile(`^\s+(\w+\.go:\d+): (.*)$`)
)

// failedTest is a test that failed, with its first error line.
type failedTest struct {
	name    string
	message string
}

// parseFailedTests reads the failed tests from a go test -v log, each with its
// first error line. A test can fail without a failed check, for example when
// a case stops with Fatal.
func parseFailedTests(path string) []failedTest {
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return nil
	}

	var (
		failed  []failedTest
		current string
		first   = map[string]string{}
	)
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if m := testHeader.FindStringSubmatch(line); m != nil {
			current = m[1]
			continue
		}

		if name, ok := strings.CutPrefix(trimmed, "--- FAIL: "); ok {
			if i := strings.LastIndex(name, " ("); i > 0 {
				name = name[:i]
			}

			failed = append(failed, failedTest{name: name, message: first[name]})
			continue
		}

		// The harness logs each command as "file.go:N: $ command"; an error line has no "$ ".
		if m := testMessage.FindStringSubmatch(line); m != nil && current != "" && first[current] == "" && !strings.HasPrefix(m[2], "$ ") {
			first[current] = m[1] + ": " + m[2]
		}
	}

	return failed
}
