package ui

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-isatty"
)

type UI struct {
	Out      io.Writer
	Err      io.Writer
	UseColor bool
	styles   styles
}

type styles struct {
	Success   lipgloss.Style
	Error     lipgloss.Style
	Warn      lipgloss.Style
	Info      lipgloss.Style
	Dim       lipgloss.Style
	Heading   lipgloss.Style
	Bold      lipgloss.Style
	Status    lipgloss.Style
	Highlight lipgloss.Style
}

const (
	iconCheck = "✓"
	iconCross = "✗"
	iconInfo  = "•"
	iconWarn  = "!"
)

func New() *UI {
	out := os.Stdout
	errOut := os.Stderr
	useColor := isatty.IsTerminal(out.Fd())
	base := lipgloss.NewStyle()

	s := styles{}
	if useColor {
		s.Success = base.Foreground(lipgloss.Color("42"))
		s.Error = base.Foreground(lipgloss.Color("196"))
		s.Warn = base.Foreground(lipgloss.Color("214"))
		s.Info = base.Foreground(lipgloss.Color("69"))
		s.Dim = base.Foreground(lipgloss.Color("246"))
		s.Heading = base.Bold(true)
		s.Bold = base.Bold(true)
		s.Status = base.Foreground(lipgloss.Color("42")).Bold(true)
		s.Highlight = base.Foreground(lipgloss.Color("214")).Bold(true)
	}

	return &UI{Out: out, Err: errOut, UseColor: useColor, styles: s}
}

func (u *UI) Heading(text string) {
	fmt.Fprintln(u.Out, u.style(u.styles.Heading, text))
}

func (u *UI) Infof(format string, args ...any) {
	u.line(u.Out, u.style(u.styles.Info, icon(u.UseColor, iconInfo, "*")), format, args...)
}

func (u *UI) Successf(format string, args ...any) {
	u.line(u.Out, u.style(u.styles.Success, icon(u.UseColor, iconCheck, "+")), format, args...)
}

func (u *UI) Warnf(format string, args ...any) {
	u.line(u.Out, u.style(u.styles.Warn, icon(u.UseColor, iconWarn, "!")), format, args...)
}

func (u *UI) Errorf(format string, args ...any) {
	u.line(u.Err, u.style(u.styles.Error, icon(u.UseColor, iconCross, "x")), format, args...)
}

func (u *UI) Dimf(format string, args ...any) {
	text := fmt.Sprintf(format, args...)
	fmt.Fprintln(u.Out, u.style(u.styles.Dim, text))
}

func (u *UI) InfofIndent(level int, format string, args ...any) {
	prefix := u.style(u.styles.Info, icon(u.UseColor, iconInfo, "*"))
	text := fmt.Sprintf(format, args...)
	if prefix != "" {
		text = prefix + " " + text
	}
	fmt.Fprintln(u.Out, u.Indent(level, text))
}

func (u *UI) Detailf(level int, format string, args ...any) {
	text := fmt.Sprintf(format, args...)
	prefix := "->"
	line := u.Indent(level, fmt.Sprintf("%s %s", prefix, text))
	fmt.Fprintln(u.Out, u.style(u.styles.Dim, line))
}

func (u *UI) Indent(level int, text string) string {
	if level <= 0 {
		return text
	}
	pad := strings.Repeat("  ", level)
	return pad + text
}

func (u *UI) line(w io.Writer, prefix string, format string, args ...any) {
	text := fmt.Sprintf(format, args...)
	if prefix != "" {
		text = prefix + " " + text
	}
	fmt.Fprintln(w, text)
}

func (u *UI) style(s lipgloss.Style, text string) string {
	if !u.UseColor {
		return text
	}
	return s.Render(text)
}

func (u *UI) Bold(text string) string {
	return u.style(u.styles.Bold, text)
}

func (u *UI) Status(text string) string {
	return u.style(u.styles.Status, text)
}

func (u *UI) Highlight(text string) string {
	return u.style(u.styles.Highlight, text)
}

func icon(useColor bool, fancy string, fallback string) string {
	if useColor {
		return fancy
	}
	return fallback
}
