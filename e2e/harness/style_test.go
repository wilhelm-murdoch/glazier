package harness

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// closingBrace is a line that only closes a block: }, }), }() and the like.
// A literal element such as }, is not a statement, so it does not match.
var closingBrace = regexp.MustCompile(`^\s*\}[)\]]*(\(\))?\s*$`)

// continuesBlock is a line that may follow a closing brace directly: another
// closing brace, the end of a declaration group or a case of a switch.
var continuesBlock = regexp.MustCompile(`^\s*(\}|\)|case\b|default:)`)

// TestBlankLineAfterClosingBrace keeps the house style of the repository, for
// glaze and for this module: a statement that ends with a closing brace has a
// blank line after it, unless the enclosing block ends there too.
func TestBlankLineAfterClosingBrace(t *testing.T) {
	root, err := findRoot()
	if err != nil {
		t.Fatal(err)
	}

	repo := filepath.Dir(root)
	err = fs.WalkDir(os.DirFS(repo), ".", func(rel string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		if d.IsDir() && rel != "." && (strings.HasPrefix(d.Name(), ".") || rel == "e2e/results" || rel == "bin" || rel == "release") {
			return fs.SkipDir
		}

		if d.IsDir() || !strings.HasSuffix(rel, ".go") {
			return nil
		}

		data, err := os.ReadFile(filepath.Join(repo, rel)) // #nosec G304 -- a source file of the repository
		if err != nil {
			return err
		}

		for _, n := range crampedBraces(strings.Split(string(data), "\n")) {
			t.Errorf("%s:%d: put a blank line after the closing brace", rel, n)
		}

		return nil
	})

	if err != nil {
		t.Fatal(err)
	}
}

// crampedBraces returns the line number of each closing brace that the next
// statement follows directly. A line in a raw string is text, not code, so
// the rule does not apply to it.
func crampedBraces(lines []string) []int {
	raw := rawStringLines(lines)
	var found []int
	for i := 0; i+1 < len(lines); i++ {
		next := lines[i+1]
		if !raw[i] && closingBrace.MatchString(lines[i]) && strings.TrimSpace(next) != "" && !continuesBlock.MatchString(next) {
			found = append(found, i+1)
		}
	}

	return found
}

// rawStringLines marks each line that starts inside a raw string literal. A
// backquote in a comment, in an interpreted string or in a rune opens none.
func rawStringLines(lines []string) map[int]bool {
	inside := map[int]bool{}
	inRaw := false
	for i, line := range lines {
		inside[i] = inRaw
		var quote rune
		escaped := false
	scan:
		for j, ch := range line {
			switch {
			case inRaw:
				inRaw = ch != '`'
			case quote != 0:
				switch {
				case escaped:
					escaped = false
				case ch == '\\':
					escaped = true
				case ch == quote:
					quote = 0
				}

			case strings.HasPrefix(line[j:], "//"):
				break scan
			case ch == '"' || ch == '\'':
				quote = ch
			case ch == '`':
				inRaw = true
			}
		}
	}

	return inside
}

func TestCrampedBraces(t *testing.T) {
	src := strings.Join([]string{
		"func f() {",           // 1
		"\tif a {",             // 2
		"\t}",                  // 3: cramped, a statement follows
		"\tx := 1",             // 4
		"\tfor {",              // 5
		"\t}",                  // 6: cramped, a comment follows
		"\t// c",               // 7
		"\tswitch x {",         // 8
		"\tcase 1:",            // 9
		"\t\tif b {",           // 10
		"\t\t}",                // 11: a case follows
		"\tcase 2:",            // 12
		"\t}",                  // 13
		"",                     // 14
		"\ts := []int{",        // 15
		"\t\t1,",               // 16
		"\t}",                  // 17: cramped, a literal that ends a statement
		"\treturn s, func() {", // 18
		"\t}",                  // 19: the function ends
		"}",                    // 20
		"",                     // 21
		"var p = `a {",         // 22
		"}",                    // 23: in a raw string
		"b {",                  // 24
		"}`",                   // 25
		"var q = \"`\"",        // 26: a backquote in a string opens no raw string
		"func g() {",           // 27
		"\tif c {",             // 28
		"\t}",                  // 29: cramped
		"\tg()",                // 30
		"}",                    // 31
	}, "\n")
	got := crampedBraces(strings.Split(src, "\n"))
	want := []int{3, 6, 17, 29}
	if !slices.Equal(got, want) {
		t.Errorf("crampedBraces = %v, want %v", got, want)
	}
}
