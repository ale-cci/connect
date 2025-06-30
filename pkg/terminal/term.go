package terminal

import (
	"bufio"
	"errors"
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
		line int
		col  int
	}

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
	command := [][]rune{{}}
	t.Output.Write([]byte(t.Prompt))

	escapeRune := '\x00'
	done := false

Loop:
	for {
		b, err := t.Input.Peek(1)
		if err != nil {
			if errors.Is(io.EOF, err) {
				break Loop
			} else {
				return "", err
			}
		}

		switch b[0] {
		case CTRL_C:
			t.Input.ReadByte()
			for t.delRune(&command) != '\x00' {
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

			r := t.delRune(&command)

			isWord := func(r rune) bool {
				return unicode.IsLetter(r) || r == '_'
			}

			wordDeleted := isWord(r)

			for r != '\x00' {
				if !isWord(lastRune(&command)) && wordDeleted {
					break
				}
				r = t.delRune(&command)
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
			command = append(command, []rune{'\n'})
			if done {
				break Loop
			}

		case KEY_BACKSPACE:
			_, err = t.Input.ReadByte()
			t.delRune(&command)

		default:
			r, _, err := t.Input.ReadRune()
			if err != nil {
				return "", err
			}
			if isPrintable(r) {
				command[len(command)-1] = append(command[len(command)-1], r)
				t.outBuf = append(t.outBuf, []byte(string([]rune{r}))...)

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
	for _, line := range command {
		builder.WriteString(string(line))
	}
	return builder.String(), nil
}

func (t *Terminal) parseEscape() (error) {
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
		case 'A': // up
		case 'C': // right
		case 'B': // down
		default:
			fmt.Printf("<%d>", b)
		}
	}
	return nil
}

func lastRune(command *[][]rune) rune {
	line := (*command)[len(*command)-1]
	if len(line) == 0 {
		return '\x00'
	}
	return line[len(line)-1]
}

func (t *Terminal) delRune(command *[][]rune) rune {
	current := (*command)[len(*command)-1]

	toDel := '\x00'
	if len(current) > 0 {
		toDel = current[len(current)-1]

		if toDel != '\n' {
			t.outBuf = append(t.outBuf, KEY_ESCAPE, '[', '1', 'D')
			t.outBuf = append(t.outBuf, ' ')
			t.outBuf = append(t.outBuf, KEY_ESCAPE, '[', '1', 'D')

			(*command)[len(*command)-1] = current[:len(current)-1]

		} else {
			// update the command, removing the last line
			*command = (*command)[:len(*command)-1]
			current = (*command)[len(*command)-1]

			// move the cursor
			t.outBuf = append(t.outBuf, []byte("\x1b[1A")...)

			offset := len(current)
			if len(*command) == 1 {
				offset += len(t.Prompt) + 1
			}

			t.outBuf = append(t.outBuf, []byte(fmt.Sprintf("\x1b[%dC", offset-1))...)
		}
	}
	return toDel
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
