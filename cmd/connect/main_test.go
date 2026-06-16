package main

import (
	"bytes"
	"testing"
)

func TestWriteTableSingleLine(t *testing.T) {
	result := &ResultSet{
		Headers: []string{"ID", "Name"},
		Rows: [][]string{
			{"1", "Alice"},
			{"2", "Bob"},
		},
	}
	var buf bytes.Buffer
	writeTable(&buf, result)

	expected := " +----+-------+\n | ID | Name  |\n +----+-------+\n | 1  | Alice |\n | 2  | Bob   |\n +----+-------+\n"
	if buf.String() != expected {
		t.Errorf("expected:\n%q\ngot:\n%q", expected, buf.String())
	}
}

func TestWriteTableMultiLine(t *testing.T) {
	result := &ResultSet{
		Headers: []string{"ID", "Description"},
		Rows: [][]string{
			{"1", "Line 1\nLine 2"},
			{"2", "Single line"},
		},
	}
	var buf bytes.Buffer
	writeTable(&buf, result)

	expected := " +----+-------------+\n | ID | Description |\n +----+-------------+\n | 1  | Line 1      |\n |    | Line 2      |\n | 2  | Single line |\n +----+-------------+\n"
	if buf.String() != expected {
		t.Errorf("expected:\n%q\ngot:\n%q", expected, buf.String())
	}
}
