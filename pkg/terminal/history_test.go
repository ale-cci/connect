package terminal_test

import "testing"
import "codeberg.org/ale-cci/connect/pkg/terminal"


func TestHistorySearch(t *testing.T) {
	h := terminal.History {}
	h.Add("answer")
	h.Add("but")

	got, err := h.Search("a")
	if err != nil {
		t.Errorf("expected nil error, got %v", err)
		return
	}

	expect := "answer"

	if expect != got {
		t.Errorf("expected %v, got %v", expect, got)
	}
}
