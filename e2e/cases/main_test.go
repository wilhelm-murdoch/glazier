// Package cases holds the end-to-end cases. Each file covers one area of
// glaze; each case is one parallel subtest with its own tmux server.
package cases

import (
	"os"
	"testing"

	"github.com/wilhelm-murdoch/glazier/e2e/harness"
)

func TestMain(m *testing.M) { os.Exit(harness.Main(m)) }
