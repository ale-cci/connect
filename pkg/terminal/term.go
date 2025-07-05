package terminal

import (
	"bufio"
	"fmt"
	"io"
	"strings"
	"unicode"

	"golang.org/x/sys/unix"
	"slices"
)

type Terminal struct {
	Input  bufio.Reader
	Output io.Writer
	Prompt string
	buffer []byte

	pos struct {
		row int
		col int
	}
	display [][]rune

	history []string
}

const (
	CTRL_C = 'c' & 0x1f
	CTRL_D = 'd' & 0x1f
	CTRL_L = 'l' & 0x1f
	CTRL_W = 'w' & 0x1f

	CTRL_A = 'a' & 0x1f
	CTRL_E = 'e' & 0x1f
	CTRL_P = 'p' & 0x1f
	CTRL_N = 'n' & 0x1f

	KEY_ENTER     = 13
	KEY_ESCAPE    = 27
	KEY_BACKSPACE = 127
)

func (t *Terminal) ReadCmd() (cmd string, err error) {
	t.pos.row = 0
	t.pos.col = 0
	t.display = [][]rune{{}}
	t.Output.Write([]byte(t.Prompt))
	t.buffer = []byte{}

	escapeRune := '\x00'
	done := false

Loop:
	for {
		b, err := t.Input.Peek(1)

		if err != nil {
			return "", err
		}

		switch b[0] {
		case CTRL_C:
			t.Input.ReadByte()
			for t.delRune() != '\x00' {
			}
			return "", nil

		case CTRL_D:
			t.Input.ReadByte()
			return "", io.EOF

		case CTRL_L:
			t.Input.ReadByte()

			t.buffer = append(t.buffer, []byte("\x1b[2J")...)
			t.buffer = append(t.buffer, []byte("\x1b[H")...)
			t.buffer = append(t.buffer, []byte(t.Prompt)...)
			for _, row := range t.display {
				t.buffer = append(t.buffer, []byte(string(row))...)
			}

			t.buffer = fmt.Appendf(t.buffer, "\x1b[%d;%dH", t.pos.row + 1, t.column())

		case CTRL_W:
			t.Input.ReadByte()

			r := t.delRune()

			isWord := func(r rune) bool {
				return unicode.IsLetter(r) || r == '_' || unicode.IsDigit(r)
			}

			wordDeleted := isWord(r)

			for r != '\x00' {
				if !isWord(t.prevRune()) && wordDeleted {
					break
				}
				r = t.delRune()
				wordDeleted = wordDeleted || !unicode.IsSpace(r)
			}

		case CTRL_A:
			t.Input.ReadByte()
			t.pos.col = 0
			if t.pos.row == 0 {
				t.buffer = fmt.Appendf(t.buffer, "\x1b[%dG", len(t.Prompt)+1)
			} else {
				t.buffer = fmt.Appendf(t.buffer, "\x1b[%dG", 1)
			}

		case CTRL_E:
			t.Input.ReadByte()
			t.pos.col = len(t.display[t.pos.row])

			cursorX := CursorPos(t.display[t.pos.row], t.pos.col)
			if t.pos.row == 0 {
				cursorX += len(t.Prompt)
			}
			t.buffer = fmt.Appendf(t.buffer, "\x1b[%dG", cursorX)

		case KEY_ESCAPE:
			// parse escape seq.
			_, err = t.Input.ReadByte()
			if err != nil {
				return "", err
			}
			t.parseEscape()

		case KEY_ENTER:
			_, err = t.Input.ReadByte()
			t.buffer = append(t.buffer, '\r', '\n')

			currentRow := t.display[t.pos.row]

			t.display = append(
				append(
					t.display[:t.pos.row],
					append(currentRow[:t.pos.col], '\n'),
					currentRow[t.pos.col:],
				),
				t.display[t.pos.row+1:]...,
			)

			if done {
				break Loop
			}

			t.pos.row += 1
			t.pos.col = 0

		case KEY_BACKSPACE:
			_, err = t.Input.ReadByte()
			t.delRune()

		default:
			r, _, err := t.Input.ReadRune()
			if err != nil {
				return "", err
			}
			if isPrintable(r) {
				row := t.display[t.pos.row]

				t.pos.col = min(t.pos.col, len(t.display[t.pos.row])) // move the value of col inbound
				after := row[t.pos.col:]

				t.display[t.pos.row] = []rune(strings.Join(
					[]string{
						string(row[:t.pos.col]),
						string(r),
						string(after),
					},
					"",
				))

				t.buffer = fmt.Appendf(t.buffer, "%s%s", string(r), string(after))
				if len(after) > 0 {
					t.buffer = fmt.Appendf(t.buffer, "\x1b[%dD", len(after))
				}
				t.pos.col += 1

				if r == '"' || r == '\'' {
					if escapeRune == '\x00' {
						escapeRune = r
					} else if escapeRune == r {
						escapeRune = '\x00'
					}

				} else if r == ';' && escapeRune == '\x00' {
					done = true
				}
			} else {
				fmt.Printf("<%d>", r)
			}
		}
		t.flush()
	}
	t.flush()

	var builder strings.Builder
	for _, line := range t.display {
		builder.WriteString(string(line))
	}

	query := builder.String()
	t.history = append(t.history, query)
	if len(t.history) > 20 {
		t.history = t.history[:20]
	}
	return query, nil
}

func (t *Terminal) flush() {
	t.Output.Write(t.buffer)
	t.buffer = []byte{}
}

func (t *Terminal) parseEscape() error {
	b, err := t.Input.ReadByte()
	if err != nil {
		return err
	}
	// https://en.wikipedia.org/wiki/ANSI_escape_code#CSIsection
	if b == '[' {
		// read any number of bytes between [0x30 - 0x3f]
		// then any number of intermediates [0x20 - 0x2f]
		// then a final byte [0x40 - 0x7E]
		// arguments delimited by ;

		b, err := t.Input.ReadByte()
		if err != nil {
			return err
		}

		switch b {
		case 'D': // left
			if t.pos.col > 0 {
				t.pos.col -= 1
			}

		case 'A': // up
			if t.pos.row > 0 {
				t.pos.row -= 1
				t.buffer = append(t.buffer, []byte("\x1b[A")...)
			}

		case 'C': // right
			maxRight := len(t.display[t.pos.row])

			if t.pos.col < maxRight {
				t.pos.col = t.pos.col + 1
			}
		case 'B': // down
			maxDown := len(t.display) - 1

			if t.pos.row < maxDown {
				t.pos.row += 1
				t.buffer = append(t.buffer, []byte("\x1b[B")...)
			}
		default:
			fmt.Printf("<%d>", b)
		}
	}

	cursorX := CursorPos(t.display[t.pos.row], t.pos.col)
	if t.pos.row == 0 {
		cursorX += len(t.Prompt)
	}
	t.buffer = fmt.Appendf(t.buffer, "\x1b[%dG", cursorX)
	return nil
}

func (t *Terminal) column() int {
	cursorX := CursorPos(t.display[t.pos.row], t.pos.col)
	if t.pos.row == 0 {
		cursorX += len(t.Prompt)
	}
	return cursorX
}

func CursorPos(s []rune, nchars int) int {
	column := 1
	for i, chr := range s {
		if i > nchars {
			break
		}
		switch chr {
		case '\n':
			
		case '\t':
			tabSize := 4
			column += (tabSize - (column-1)%tabSize)
		default:
			column += 1
		}
	}
	return column
}

func (t *Terminal) prevRune() rune {
	if t.pos.col == 0 && t.pos.row == 0 {
		return '\x00'
	}

	if t.pos.col == 0 {
		prevRow := t.display[t.pos.row-1]
		return prevRow[len(prevRow)-1]
	}

	currentRow := t.display[t.pos.row]
	return currentRow[len(currentRow)-1]
}

func (t *Terminal) delRune() rune {
	if t.pos.col == 0 && t.pos.row == 0 {
		return '\x00'
	}
	if t.pos.col == 0 && t.pos.row > 0 {
		t.display = slices.Delete(t.display, t.pos.row, t.pos.row+1)
		t.pos.row -= 1
		line := t.display[t.pos.row]
		t.display[t.pos.row] = line[:len(line)-1]

		t.pos.col = len(t.display[t.pos.row])

		right := len(t.display[t.pos.row])

		if t.pos.row == 0 {
			right += len(t.Prompt)
		}

		t.buffer = fmt.Appendf(t.buffer, "\x1b[A\x1b[%dC", right)

		return '\n'
	} else {
		currentRow := t.display[t.pos.row]
		after := currentRow[t.pos.col-1:]
		toDel := currentRow[t.pos.col-1]

		t.display[t.pos.row] = slices.Delete(
			currentRow, t.pos.col-1, t.pos.col,
		)

		t.pos.col -= 1
		t.buffer = fmt.Appendf(t.buffer, "\x1b[D%s \x1b[%dD", string(after), len(after))
		// t.buffer = append(t.buffer, []byte("\x1b[D \x1b[D")...)
		return toDel
	}
	return '\x00'
}

func isPrintable(r rune) bool {
	// the golang implementation checks it's not a surrogate character
	// isSurrogate := key >= 0xd800 && key <= 0xdbff
	// return r >= 32 && !isSurrogate
	// https://cs.opensource.google/go/x/term/+/refs/tags/v0.32.0:terminal.go;l=268
	return r >= 32
}

type State struct {
	termios unix.Termios
}

func MakeRaw(fd int) (*State, error) {
	termios, err := unix.IoctlGetTermios(fd, ioctlReadTermios)
	if err != nil {
		return nil, err
	}

	state := State{termios: *termios}
	// This attempts to replicate the behaviour documented for cfmakeraw in
	// the termios(3) manpage.
	termios.Iflag &^= unix.IGNBRK | unix.BRKINT | unix.PARMRK | unix.ISTRIP | unix.INLCR | unix.IGNCR | unix.ICRNL | unix.IXON
	// termios.Oflag &^= unix.OPOST
	termios.Lflag &^= unix.ECHO | unix.ECHONL | unix.ICANON | unix.ISIG | unix.IEXTEN
	termios.Cflag &^= unix.CSIZE | unix.PARENB
	termios.Cflag |= unix.CS8
	termios.Cc[unix.VMIN] = 1
	termios.Cc[unix.VTIME] = 0
	if err := unix.IoctlSetTermios(fd, ioctlWriteTermios, termios); err != nil {
		return nil, err
	}

	return &state, nil
}

func Restore(fd int, state *State) error {
	return unix.IoctlSetTermios(fd, ioctlWriteTermios, &state.termios)
}
