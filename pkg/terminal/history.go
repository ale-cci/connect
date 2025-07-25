package terminal

import (
	"fmt"
	"strings"
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
