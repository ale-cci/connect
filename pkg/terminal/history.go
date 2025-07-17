package terminal

import (
	"fmt"
	"strings"
	"io"
	"bufio"
	"encoding/base64"
)

type History struct {
	Strings []string
	idx     int
	Size    int
}

func (h *History) Previous() (string, error) {
	i := h.idx
	if i < 0 || i >= len(h.Strings) {
		return "", fmt.Errorf("no more commands")
	}

	result := h.Strings[len(h.Strings)-1-i]
	h.idx += 1
	return result, nil
}

func (h *History) Next() (string, error) {
	i := h.idx - 2
	if i == -1 {
		h.idx -= 1
		return "", nil
	}
	if i < 0 || i >= len(h.Strings) {
		return "", fmt.Errorf("no more commands")
	}
	result := h.Strings[len(h.Strings)-1-i]
	h.idx -= 1
	return result, nil
}

func (h *History) ResetCounter() {
	h.idx = 0
}

func (h *History) Add(s string) {
	h.Strings = append(h.Strings, s)

	// trim history
	if h.Size > 0 {
		h.Strings = h.Strings[len(h.Strings)-h.Size:]
	}
}


func (h *History) Search(s string) (string, error) {
	// search back
	for {
		cmd, err := h.Previous()
		if err != nil {
			return "", err
		}

		if strings.Contains(cmd, s) {
			return cmd, nil
		}
	}
}

func (h *History) Save(fd io.Writer) {
	for _, cmd := range h.Strings {
		toWrite := base64.StdEncoding.EncodeToString([]byte(cmd))
		fd.Write([]byte(toWrite))
		fd.Write([]byte("\n"))
	}
}

func (h *History) Load(fd io.Reader) {
	reader := bufio.NewScanner(fd)
	for reader.Scan() {
		cmd, err := base64.StdEncoding.DecodeString(reader.Text())
		if err == nil {
			h.Add(string(cmd))
		}
	}
	
}
