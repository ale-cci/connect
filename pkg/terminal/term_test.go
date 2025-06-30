package terminal_test

import (
	"bufio"
	"bytes"
	"testing"

	"codeberg.org/ale-cci/connect/pkg/terminal"
)


func TestTerminal(t *testing.T) {
    input := bytes.Buffer{}
    output := bytes.Buffer{}

    term := terminal.Terminal{
        Input: *bufio.NewReader(&input),
        Output: &output,
        Prompt: ">",
    }

    _, err := term.ReadCmd()
    if err == nil {
        t.Errorf("expected EOF, got no errors")
    }
}
