package main

import (
	"os"
	"strings"

	"github.com/sagernet/sing/common"

	"github.com/mattn/go-runewidth"
	"golang.org/x/term"
)

var (
	stdoutIsTerminal = term.IsTerminal(int(os.Stdout.Fd()))
	stderrIsTerminal = term.IsTerminal(int(os.Stderr.Fd()))
)

func writeStderrLine(message string) {
	if !stderrIsTerminal {
		return
	}
	os.Stderr.WriteString(message + "\n")
}

func writeProgress(message string) {
	if !stderrIsTerminal {
		return
	}
	os.Stderr.WriteString("\r" + message)
}

func stripColors(message string) string {
	if !strings.Contains(message, "\x1b[") {
		return message
	}
	var builder strings.Builder
	start := 0
	for index := 0; index < len(message); {
		if message[index] != '\x1b' || index+1 >= len(message) || message[index+1] != '[' {
			index++
			continue
		}
		end := index + 2
		for end < len(message) && message[end] != 'm' {
			end++
		}
		if end >= len(message) {
			break
		}
		builder.WriteString(message[start:index])
		index = end + 1
		start = index
	}
	builder.WriteString(message[start:])
	return builder.String()
}

type tableWriter struct {
	header       []string
	emptyMessage string
	rows         [][]string
}

func (t *tableWriter) addRow(cells ...string) {
	t.rows = append(t.rows, common.Map(cells, func(it string) string {
		if it == "" {
			return "-"
		}
		return it
	}))
}

func (t *tableWriter) flush() {
	if len(t.rows) == 0 {
		writeStderrLine(t.emptyMessage)
		return
	}
	if !stdoutIsTerminal {
		var output strings.Builder
		for _, row := range t.rows {
			output.WriteString(strings.Join(row, "\t"))
			output.WriteString("\n")
		}
		os.Stdout.WriteString(output.String())
		return
	}
	widths := common.Map(t.header, func(it string) int {
		return runewidth.StringWidth(it)
	})
	for _, row := range t.rows {
		for index, cell := range row {
			widths[index] = max(widths[index], runewidth.StringWidth(cell))
		}
	}
	renderRow := func(cells []string) string {
		var builder strings.Builder
		for index, cell := range cells {
			if index > 0 {
				builder.WriteString("  ")
			}
			builder.WriteString(cell)
			if index < len(cells)-1 {
				builder.WriteString(strings.Repeat(" ", widths[index]-runewidth.StringWidth(cell)))
			}
		}
		return builder.String()
	}
	writeStderrLine(renderRow(t.header))
	var output strings.Builder
	for _, row := range t.rows {
		output.WriteString(renderRow(row))
		output.WriteString("\n")
	}
	os.Stdout.WriteString(output.String())
}

type blockLine struct {
	label string
	value string
}

type blockWriter struct {
	lines []blockLine
}

func (b *blockWriter) addLine(label string, value string) {
	if value == "" {
		value = "-"
	}
	b.lines = append(b.lines, blockLine{label: label, value: value})
}

func (b *blockWriter) flush() {
	if len(b.lines) == 0 {
		return
	}
	labelWidth := len(common.MaxBy(b.lines, func(it blockLine) int {
		return len(it.label)
	}).label) + 3
	var output strings.Builder
	for _, line := range b.lines {
		output.WriteString(line.label)
		output.WriteString(":")
		output.WriteString(strings.Repeat(" ", labelWidth-len(line.label)-1))
		output.WriteString(line.value)
		output.WriteString("\n")
	}
	os.Stdout.WriteString(output.String())
}
