package terminal_test

import (
	"bufio"
	"bytes"
	"fmt"
	"testing"

	"codeberg.org/ale-cci/connect/pkg/terminal"
)

func TestTerminal(t *testing.T) {
	input := bytes.Buffer{}
	output := bytes.Buffer{}

	term := terminal.Terminal{
		Input:  *bufio.NewReader(&input),
		Output: &output,
		Prompt: "> ",
	}

	_, err := term.ReadCmd()
	if err == nil {
		t.Errorf("expected EOF, got no errors")
	}

	got := string(output.Bytes())
	expectOut := "> "
	if got != expectOut {
		t.Errorf("expected prompt %q, got %q", expectOut, got)
	}
}

func TestCommandReadInput(t *testing.T) {
	tt := []struct {
		input  string
		expect string
	}{
		{
			input:  "select 1;\r",
			expect: "select 1;\n",
		},
		{
			input:  "select \r1;\r",
			expect: "select \n1;\n",
		},
		{
			// test backspace
			input:  "select 1\x7f2;\r",
			expect: "select 2;\n",
		},
		{
			// 3. test arrows left and right
			input:  "select 1\x1b[D2\x1b[C;\r",
			expect: "select 21;\n",
		},
		{
			// test ctrl-w
			input:  "select 1234\x175;\r",
			expect: "select 5;\n",
		},
		{
			// test ctrl-w 2
			input:  "select \x17show tables;\r",
			expect: "show tables;\n",
		},
		{
			// test ctrl-w 3
			input:  "select 1\x17'test';\r",
			expect: "select 'test';\n",
		},
		{
			// test ctrl-a + ctrl-e
			input:  "elect 1;\x01s\x05\r",
			expect: "select 1;\n",
		},
		{
			// test ctrl-a + ctrl-e
			input:  "abc\r\x7f\x7f\x7f\x7fI;\r",
			expect: "I;\n",
		},
	}


	for i, test := range tt {
		fmt.Printf("testing %d\n", i)
		output := bytes.Buffer{}
		inputBytes := []byte(test.input)

		term := terminal.Terminal{
			Input:  *bufio.NewReader(bytes.NewBuffer(inputBytes)),
			Output: &output,
			Prompt: "> ",
		}

		got, err := term.ReadCmd()

		if err != nil {
			t.Errorf("expected nil error, got %v", err)
		}

		if got != test.expect {
			t.Errorf("expected command %q, got %q", test.expect, got)
		}
	}
}

func TestTerminalOutput(t *testing.T) {
}
