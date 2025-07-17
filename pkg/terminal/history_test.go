package terminal_test

import (
	"bytes"
	"fmt"
	"reflect"
	"testing"

	"codeberg.org/ale-cci/connect/pkg/terminal"
)

func TestHistorySearch(t *testing.T) {
	h := terminal.History{}
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

func TestHistorySavesOnFile(t *testing.T) {
	tt := []struct {
		commands []string
	}{
		{
			commands: []string{"sample"},
		},
		{
			commands: []string{
				"first",
				"second",
			},
		},
		{
			commands: []string{
				"first\ncommand",
				"second",
			},
		},
		{
			commands: []string{
				"first\\nsecond",
				"third",
			},
		},
	}

	for i, tc := range tt {
		t.Run(
			fmt.Sprintf("TestHistorySavesOnFile[%d]", i),
			func (t *testing.T) {
				h := terminal.History{}
				for _, cmd := range tc.commands {
					h.Add(cmd)
				}

				histfile := bytes.Buffer{}
				h.Save(&histfile)

				newhist := terminal.History{}
				newhist.Load(&histfile)

				if !reflect.DeepEqual(h.Strings, newhist.Strings) {
					t.Errorf("saved strings are loaded differently %v %v", h.Strings, newhist.Strings)
				}
			},
		)
	}
}
