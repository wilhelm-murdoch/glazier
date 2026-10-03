package report

import (
	"strings"
	"testing"
)

func TestRead(t *testing.T) {
	in := `{"target":"t","test":"TestA/x","check":"one","status":"pass"}

{"target":"t","test":"TestA/x","check":"two","status":"xpass","detail":"d"}
`
	records, err := Read(strings.NewReader(in))
	if err != nil {
		t.Fatal(err)
	}

	if len(records) != 2 {
		t.Fatalf("got %d records", len(records))
	}

	if records[0].Failed() || !records[1].Failed() {
		t.Error("Failed: a pass failed or an unexpected pass did not")
	}

	if got := records[1].ID(); got != "TestA/x | two" {
		t.Errorf("ID %q", got)
	}
}

func TestReadRejectsBadJSON(t *testing.T) {
	if _, err := Read(strings.NewReader("{\n")); err == nil || !strings.Contains(err.Error(), "line 1") {
		t.Errorf("error %v, want one that names line 1", err)
	}
}
