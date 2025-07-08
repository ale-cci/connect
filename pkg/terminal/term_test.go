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
			expect: "select 1;",
		},
		{
			input:  "select \r1;\r",
			expect: "select \n1;",
		},
		{
			// test backspace
			input:  "select 1\x7f2;\r",
			expect: "select 2;",
		},
		{
			// 3. test arrows left and right
			input:  "select 1\x1b[D2\x1b[C;\r",
			expect: "select 21;",
		},
		{
			// test ctrl-w
			input:  "select 1234\x175;\r",
			expect: "select 5;",
		},
		{
			// test ctrl-w 2
			input:  "select \x17show tables;\r",
			expect: "show tables;",
		},
		{
			// test ctrl-w 3
			input:  "select 1\x17'test';\r",
			expect: "select 'test';",
		},
		{
			// test ctrl-a + ctrl-e
			input:  "elect 1;\x01s\x05\r",
			expect: "select 1;",
		},
		{
			input:  "abc\r\x7f\x7f\x7f\x7fI;\r",
			expect: "I;",
		},
		{
			input:  "select\r * from utenti;\x01\x7f\r",
			expect: "select * from utenti;",
		},
		{
			input:  "select bc;\x1bba\r",
			expect: "select abc;",
		},
		{
			input:  "select bc   \x1bba\x05;\r",
			expect: "select abc   ;",
		},
		{
			input:  "selec  bc;\x01\x1bft\r",
			expect: "select  bc;",
		},
		{
			input:  "a\x01\r\x1b[C;\r",
			expect: "a\n;",
		},
		{
			input:  "ab\x1b[D\r\x1b[C;\r",
			expect: "a\nb;",
		},
	}

	for _, test := range tt {
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

func TestCursorPos(t *testing.T) {
	tt := []struct {
		text     string
		nchar    int
		expected int
	}{
		{text: "asdf", nchar: 0, expected: 1},    // 0.
		{text: "asdf", nchar: 1, expected: 2},    // 1.
		{text: "asdf\n", nchar: 10, expected: 5}, // 2.
		{text: "asdfe", nchar: 10, expected: 6},  // 3.
		{text: "a\tb", nchar: 2, expected: 5},    // 4.
		{text: "a\tb", nchar: 3, expected: 6},    // 5.
	}

	for idx, tc := range tt {
		t.Run(
			fmt.Sprintf("TestCursorPos[%d]", idx),
			func(t *testing.T) {
				xpos := terminal.CursorPos([]rune(tc.text), tc.nchar)
				if xpos != tc.expected {
					t.Errorf("expected %d, got %d", tc.expected, xpos)
				}
			},
		)
	}
}

func TestTermOutput(t *testing.T) {
	tt := []struct {
		input  string
		expect string
	}{
		{
			input:  "abc;\r",
			expect: "> abc;\r\n",
		},
	}

	for idx, tc := range tt {
		t.Run(
			fmt.Sprintf("TestTermOutput[%d]", idx),
			func(t *testing.T) {
				input := bytes.NewBuffer([]byte(tc.input))
				output := bytes.Buffer{}

				term := terminal.Terminal{
					Input:  *bufio.NewReader(input),
					Output: &output,
					Prompt: "> ",
				}

				_, err := term.ReadCmd()
				if err != nil {
					t.Errorf("expected nil error, got %v", err)
					return
				}

				got := output.String()
				if got != tc.expect {
					t.Errorf("expected %q, got %q", tc.expect, got)
				}
			},
		)
	}
}

func TestPreviousWithoutValuesReturnsNothing(t *testing.T) {
	h := terminal.History{}

	_, err := h.Previous()
	if err == nil {
		t.Errorf("expected error, got nil")
	}
}


func TestPreviousReturnsItem(t *testing.T) {
	h := terminal.History{}
	h.Add("sample")

	cmd, err := h.Previous()
	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}

	if cmd != "sample" {
		t.Errorf("expected command, got %v", cmd)
	}
}
