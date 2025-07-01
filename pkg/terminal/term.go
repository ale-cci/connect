package terminal

import (
	"bufio"
	"fmt"
	"io"
	"strings"
	"unicode"

	"golang.org/x/sys/unix"
)

type Terminal struct {
	Input  bufio.Reader
	Output io.Writer
	Prompt string
	outBuf []byte

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

			t.outBuf = append(t.outBuf, []byte("\x1b[2J")...)
			t.outBuf = append(t.outBuf, []byte("\x1b[H")...)

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

		case KEY_ESCAPE:
			// parse escape seq.
			_, err = t.Input.ReadByte()
			if err != nil {
				return "", err
			}
			t.parseEscape()

		case KEY_ENTER:
			_, err = t.Input.ReadByte()
			t.outBuf = append(t.outBuf, '\r', '\n')

			newDisplay := append(
				append(
					t.display[:t.pos.row+1],
					[]rune{'\n'},
				),
				t.display[t.pos.row+1:]...,
			)
			t.display = newDisplay
			if done {
				break Loop
			}

			t.pos.row += 1
			t.pos.col = 1

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

				t.display[t.pos.row] = []rune(strings.Join(
					[]string{
						string(row[:t.pos.col]),
						string(r),
						string(row[t.pos.col:]),
					},
					"",
				))

				t.pos.col += 1

				t.outBuf = append(t.outBuf, byte(r))

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
		t.Output.Write(t.outBuf)
		t.outBuf = []byte{}
	}

	t.Output.Write(t.outBuf)
	t.outBuf = []byte{}

	var builder strings.Builder
	for _, line := range t.display {
		builder.WriteString(string(line))
	}
	return builder.String(), nil
}

func (t *Terminal) parseEscape() error {
	b, err := t.Input.ReadByte()
	if err != nil {
		return err
	}
	if b == '[' {
		b, err := t.Input.ReadByte()
		if err != nil {
			return err
		}

		switch b {
		case 'D': // left
			t.pos.col = max(0, t.pos.col-1)
		case 'A': // up
			t.pos.row = max(0, t.pos.row-1)
		case 'C': // right
			maxRight := len(t.display[t.pos.row]) + 1
			t.pos.col = min(t.pos.col+1, maxRight)
		case 'B': // down
			maxDown := len(t.display) - 1
			t.pos.row = min(t.pos.row+1, maxDown)
		default:
			fmt.Printf("<%d>", b)
		}
	}
	return nil
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
	if t.pos.col == 1 && t.pos.row > 0 {
		t.display = append(t.display[:t.pos.row], t.display[t.pos.row+1:]...)
		t.pos.row -= 1
		t.pos.col = len(t.display[t.pos.row])

		return '\n'
	} else if t.pos.col > 0 {
		currentRow := t.display[t.pos.row]
		toDel := currentRow[t.pos.col-1]

		t.display[t.pos.row] = append(
			currentRow[:t.pos.col-1],
			currentRow[t.pos.col:]...,
		)

		t.pos.col -= 1
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
