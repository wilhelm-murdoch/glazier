package harness

import (
	"encoding/json"
	"os"
	"sync"

	"github.com/wilhelm-murdoch/glazier/e2e/report"
)

const (
	reportFileMode     = 0o600
	maxRecordedDetails = 2000
)

type reporter struct {
	mu     sync.Mutex
	file   *os.File
	enc    *json.Encoder
	target string
}

// openReport opens the JSON lines report. An empty path writes no report.
func openReport(path, target string) (*reporter, error) {
	r := &reporter{target: target}
	if path == "" {
		return r, nil
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, reportFileMode) // #nosec G304 G703 -- the path that the runner gives
	if err != nil {
		return nil, err
	}

	r.file, r.enc = f, json.NewEncoder(f)
	return r, nil
}

func (r *reporter) write(test, check string, status report.Status, detail string) {
	if r == nil || r.enc == nil {
		return
	}

	if len(detail) > maxRecordedDetails {
		detail = detail[:maxRecordedDetails] + "…"
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	_ = r.enc.Encode(report.Record{Target: r.target, Test: test, Check: check, Status: status, Detail: detail})
}

func (r *reporter) close() error {
	if r == nil || r.file == nil {
		return nil
	}

	return r.file.Close()
}
