package tmux

import (
	"errors"
	"slices"
	"strconv"
	"testing"
)

func TestSplitTmuxLine(t *testing.T) {
	tests := []struct {
		name    string   // Name of the test case
		line    string   // Input line
		want    []string // A list of strings representing derived components of the tmux line
		wantErr error    // The error we expect for a failed case
	}{
		// Sessions:
		{
			name: "session/plain",
			line: "$0;main;/home/user",
			want: []string{"$0", "main", "/home/user"},
		},
		{
			name: "session/escaped spaces in name and path",
			line: `$1;my\ session;/home/user/my\ project`,
			want: []string{"$1", "my session", "/home/user/my project"},
		},
		{
			name: "session/escaped delimiter in name",
			line: `$2;foo\;bar;/tmp`,
			want: []string{"$2", "foo;bar", "/tmp"},
		},
		{
			name: "session/escaped quote and space in path",
			line: `$3;work;/srv/it\'s\ here`,
			want: []string{"$3", "work", "/srv/it's here"},
		},

		// Windows:
		{
			name: "window/plain single-pane layout",
			line: "@3;1;editor;b25d,238x57,0,0,3;1",
			want: []string{"@3", "1", "editor", "b25d,238x57,0,0,3", "1"},
		},
		{
			name: "window/escaped parens in name, split layout",
			line: `@12;4;vim\ \(main\);c3e5,238x57,0,0{119x57,0,0,1,118x57,120,0,2};0`,
			want: []string{"@12", "4", "vim (main)", "c3e5,238x57,0,0{119x57,0,0,1,118x57,120,0,2}", "0"},
		},

		// Panes:
		{
			name: "pane/plain",
			line: "%0;0;hostname;1;/home/user",
			want: []string{"%0", "0", "hostname", "1", "/home/user"},
		},
		{
			name: "pane/escaped backslash in title",
			line: `%5;2;C:\\Users;0;/mnt/c/Users`,
			want: []string{"%5", "2", `C:\Users`, "0", "/mnt/c/Users"},
		},
		{
			name: "pane/empty title",
			line: "%9;3;;0;/var/log",
			want: []string{"%9", "3", "", "0", "/var/log"},
		},

		//  Edge cases
		{
			name: "empty line yields one empty part",
			line: "",
			want: []string{""},
		},
		{
			name: "only delimiters",
			line: ";;",
			want: []string{"", "", ""},
		},
		{
			name: "trailing delimiter yields trailing empty part",
			line: "$0;main;",
			want: []string{"$0", "main", ""},
		},
		{
			name: "escaped delimiter at end of line",
			line: `$0;main\;`,
			want: []string{"$0", "main;"},
		},

		//  Failures
		{
			name:    "trailing escape on session line",
			line:    `$0;main;/home/user\`,
			wantErr: ErrTrailingEscape,
		},
		{
			name:    "trailing escape on pane line",
			line:    `%1;0;title;1;/tmp\`,
			wantErr: ErrTrailingEscape,
		},
		{
			name:    "lone backslash",
			line:    `\`,
			wantErr: ErrTrailingEscape,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := splitTmuxLine(tc.line, tmuxLinePartDelimiter)

			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("splitTmuxLine(%q) error = %v, want %v", tc.line, err, tc.wantErr)
			}
			if tc.wantErr != nil {
				if got != nil {
					t.Errorf("splitTmuxLine(%q) parts = %q, want nil on error", tc.line, got)
				}
				return
			}
			if !slices.Equal(got, tc.want) {
				t.Errorf("splitTmuxLine(%q) = %q, want %q", tc.line, got, tc.want)
			}
		})
	}
}

func TestGetPartsFromTmuxLine(t *testing.T) {
	tests := []struct {
		name      string // Name of the test case
		line      string // Input line
		prefix    string // The prefix of the id component
		length    int    // The number of parts we expect
		wantId    int    // The id we derived using the specified prefix
		wantErr   error  // The error we expect for a failed case
		wantCause error  // The underlying error wrapped beneath wantErr, if any
	}{
		//  Passing
		{
			name:   "session/id zero",
			line:   "$0;main;/home/user",
			prefix: "$",
			length: 3,
			wantId: 0,
		},
		{
			name:   "session/multi-digit id with escaped name",
			line:   `$42;my\ session;/home/user`,
			prefix: "$",
			length: 3,
			wantId: 42,
		},
		{
			name:   "session/escaped delimiter does not add a part",
			line:   `$3;a\;b;/tmp`,
			prefix: "$",
			length: 3,
			wantId: 3,
		},
		{
			name:   "window/plain",
			line:   "@7;2;logs;b25d,238x57,0,0,3;1",
			prefix: "@",
			length: 5,
			wantId: 7,
		},
		{
			name:   "window/split layout",
			line:   `@12;4;vim\ \(main\);c3e5,238x57,0,0{119x57,0,0,1,118x57,120,0,2};0`,
			prefix: "@",
			length: 5,
			wantId: 12,
		},
		{
			name:   "pane/plain",
			line:   "%13;0;hostname;1;/home/user",
			prefix: "%",
			length: 5,
			wantId: 13,
		},
		{
			name:   "session/negative id is currently accepted",
			line:   "$-1;main;/home/user",
			prefix: "$",
			length: 3,
			wantId: -1,
		},

		//  Failing:
		{
			name:    "session/too few parts",
			line:    "$0;main",
			prefix:  "$",
			length:  3,
			wantErr: ErrUnexpectedPartCount,
		},
		{
			name:    "session/too many parts",
			line:    "$0;main;/home/user;extra",
			prefix:  "$",
			length:  3,
			wantErr: ErrUnexpectedPartCount,
		},
		{
			name:    "session/unescaped delimiter in name",
			line:    "$0;foo;bar;/tmp",
			prefix:  "$",
			length:  3,
			wantErr: ErrUnexpectedPartCount,
		},
		{
			name:    "window line parsed with session length",
			line:    "@3;1;editor;b25d,238x57,0,0,3;1",
			prefix:  "@",
			length:  3,
			wantErr: ErrUnexpectedPartCount,
		},
		{
			name:    "empty line",
			line:    "",
			prefix:  "$",
			length:  3,
			wantErr: ErrUnexpectedPartCount,
		},
		{
			name:      "pane line with session prefix",
			line:      "%1;0;title;1;/tmp",
			prefix:    "$",
			length:    5,
			wantErr:   ErrInvalidDerivedId,
			wantCause: ErrIdPrefixNotFound,
		},
		{
			name:      "session/missing prefix",
			line:      "0;main;/home/user",
			prefix:    "$",
			length:    3,
			wantErr:   ErrInvalidDerivedId,
			wantCause: ErrIdPrefixNotFound,
		},
		{
			name:      "session/non-numeric id",
			line:      "$abc;main;/home/user",
			prefix:    "$",
			length:    3,
			wantErr:   ErrInvalidDerivedId,
			wantCause: strconv.ErrSyntax,
		},
		{
			name:      "session/empty id after prefix",
			line:      "$;main;/home/user",
			prefix:    "$",
			length:    3,
			wantErr:   ErrInvalidDerivedId,
			wantCause: strconv.ErrSyntax,
		},
		{
			name:      "window/whitespace in id",
			line:      "@ 3;1;editor;b25d,238x57,0,0,3;1",
			prefix:    "@",
			length:    5,
			wantErr:   ErrInvalidDerivedId,
			wantCause: strconv.ErrSyntax,
		},
		{
			name:      "pane/id overflows int",
			line:      "%99999999999999999999;0;title;1;/tmp",
			prefix:    "%",
			length:    5,
			wantErr:   ErrInvalidDerivedId,
			wantCause: strconv.ErrRange,
		},
		{
			name:    "session/trailing escape",
			line:    `$0;main;/home/user\`,
			prefix:  "$",
			length:  3,
			wantErr: ErrTrailingEscape,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			parts, id, err := getPartsFromTmuxLine(tc.line, tc.prefix, tc.length)

			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("getPartsFromTmuxLine(%q, %q, %d) error = %v, want %v",
					tc.line, tc.prefix, tc.length, err, tc.wantErr)
			}
			if tc.wantCause != nil && !errors.Is(err, tc.wantCause) {
				t.Fatalf("getPartsFromTmuxLine(%q, %q, %d) error = %v, want cause %v",
					tc.line, tc.prefix, tc.length, err, tc.wantCause)
			}
			if tc.wantErr != nil {
				if id != 0 {
					t.Errorf("id = %d on error, want 0", id)
				}
				return
			}
			if id != tc.wantId {
				t.Errorf("id = %d, want %d", id, tc.wantId)
			}
			if len(parts) != tc.length {
				t.Errorf("len(parts) = %d, want %d", len(parts), tc.length)
			}
		})
	}
}
