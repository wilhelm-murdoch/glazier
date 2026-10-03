// Package report is the format of the end-to-end report: one JSON object for
// each check, one line for each object. The harness writes it and the matrix
// runner reads it.
package report

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
)

const (
	// Separator joins the test and the check in an id, and the fields of the baseline file.
	Separator = " | "
	maxLine   = 1 << 20 // the longest report line that Read accepts
)

// Status is the result of one check.
type Status string

// The statuses of a check. An expected failure is in the baseline and failed;
// an unexpected pass is in the baseline and passed, which fails the run.
const (
	Pass            Status = "pass"
	Fail            Status = "fail"
	ExpectedFailure Status = "xfail"
	UnexpectedPass  Status = "xpass"
)

// Record is one check on one target.
type Record struct {
	Target string `json:"target"`
	Test   string `json:"test"`
	Check  string `json:"check"`
	Status Status `json:"status"`
	Detail string `json:"detail,omitempty"`
}

// ID is the stable name of a check. It is the same on every target.
func (r Record) ID() string { return ID(r.Test, r.Check) }

// Failed reports whether the check makes the run fail.
func (r Record) Failed() bool { return r.Status == Fail || r.Status == UnexpectedPass }

// ID is the stable name of a check from its test and its label.
func ID(test, check string) string { return test + Separator + check }

// ReadFile reads a report file.
func ReadFile(path string) ([]Record, error) {
	f, err := os.Open(path) // #nosec G304 -- a report that the runner wrote
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	return Read(f)
}

// Read reads the records of a report.
func Read(r io.Reader) ([]Record, error) {
	var records []Record
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), maxLine)
	for n := 1; scanner.Scan(); n++ {
		if len(scanner.Bytes()) == 0 {
			continue
		}
		var rec Record
		if err := json.Unmarshal(scanner.Bytes(), &rec); err != nil {
			return nil, fmt.Errorf("line %d: %w", n, err)
		}
		records = append(records, rec)
	}
	return records, scanner.Err()
}
