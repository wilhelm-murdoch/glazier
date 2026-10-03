package harness

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/wilhelm-murdoch/glazier/e2e/report"
)

// Baseline is the set of checks that are known to fail on a target. A check in
// the baseline that fails is an expected failure; a check in the baseline that
// passes fails the run, so the file cannot go stale.
type Baseline struct {
	entries map[string]string // check id → reason
}

// LoadBaseline reads the entries for one target. Each line is
// "TARGET | TEST/CASE | CHECK | REASON", where TARGET "*" means every target.
// A missing file is an empty baseline.
func LoadBaseline(path, target string) (*Baseline, error) {
	b := &Baseline{entries: map[string]string{}}
	f, err := os.Open(path) // #nosec G304 -- the baseline of the module
	if errors.Is(err, os.ErrNotExist) {
		return b, nil
	}
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	scanner := bufio.NewScanner(f)
	for n := 1; scanner.Scan(); n++ {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Split(line, report.Separator)
		if len(fields) != 4 {
			return nil, fmt.Errorf("%s:%d: want TARGET | TEST/CASE | CHECK | REASON", path, n)
		}
		for i := range fields {
			fields[i] = strings.TrimSpace(fields[i])
		}
		if fields[3] == "" {
			return nil, fmt.Errorf("%s:%d: an expected failure needs a reason", path, n)
		}
		if fields[0] == "*" || fields[0] == target {
			b.entries[report.ID(fields[1], fields[2])] = fields[3]
		}
	}
	return b, scanner.Err()
}

// Expected returns the reason when a check is in the baseline.
func (b *Baseline) Expected(id string) (string, bool) {
	if b == nil {
		return "", false
	}
	reason, ok := b.entries[id]
	return reason, ok
}
