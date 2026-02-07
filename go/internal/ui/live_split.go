package ui

import (
	"fmt"
	"io"
	"sync"
)

type LiveSplit struct {
	ui        *UI
	apiLines  []string
	taskLines []string
	height    int
	rendered  bool
	active    bool
	mu        sync.Mutex
}

func newLiveSplit(ui *UI) *LiveSplit {
	active := ui.UseColor
	return &LiveSplit{
		ui:     ui,
		active: active,
	}
}

func (l *LiveSplit) Active() bool {
	return l != nil && l.active
}

func (l *LiveSplit) AppendAPI(line string) {
	if l == nil || !l.active {
		return
	}
	l.mu.Lock()
	l.apiLines = append(l.apiLines, line)
	l.mu.Unlock()
	l.render()
}

func (l *LiveSplit) SetTaskLines(lines []string) {
	if l == nil || !l.active {
		return
	}
	l.mu.Lock()
	l.taskLines = append([]string{}, lines...)
	l.mu.Unlock()
	l.render()
}

func (l *LiveSplit) ClearTasks() {
	l.SetTaskLines(nil)
}

func (l *LiveSplit) Stop() {
	if l == nil {
		return
	}
	l.ui.lockWrite()
	defer l.ui.unlockWrite()
	l.mu.Lock()
	defer l.mu.Unlock()
	if !l.active {
		return
	}
	if l.rendered && l.height > 0 {
		fmt.Fprint(l.ui.Out, "\n")
	}
	l.rendered = false
	l.height = 0
	l.active = false
	l.ui.clearSplit(l)
}

func (l *LiveSplit) WriteLineLocked(w io.Writer, text string) {
	if l == nil || !l.active {
		fmt.Fprintln(w, text)
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.rendered {
		l.clearLocked(w)
	}
	fmt.Fprintln(w, text)
	l.renderLocked(w)
}

func (l *LiveSplit) render() {
	l.ui.lockWrite()
	defer l.ui.unlockWrite()
	l.mu.Lock()
	defer l.mu.Unlock()
	l.renderLocked(l.ui.Out)
}

func (l *LiveSplit) renderLocked(w io.Writer) {
	if !l.active {
		return
	}
	lines := l.combinedLinesLocked()
	height := len(lines)
	if height == 0 {
		if l.rendered {
			l.clearLocked(w)
		}
		return
	}
	if l.rendered {
		for i := 0; i < l.height-1; i++ {
			fmt.Fprint(w, "\033[A")
		}
	}
	for i := 0; i < height; i++ {
		fmt.Fprint(w, "\r\033[2K")
		fmt.Fprint(w, lines[i])
		if i < height-1 {
			fmt.Fprint(w, "\n")
		}
	}
	l.height = height
	l.rendered = true
}

func (l *LiveSplit) clearLocked(w io.Writer) {
	if !l.rendered || l.height == 0 {
		return
	}
	for i := 0; i < l.height-1; i++ {
		fmt.Fprint(w, "\033[A")
	}
	for i := 0; i < l.height; i++ {
		fmt.Fprint(w, "\r\033[2K")
		if i < l.height-1 {
			fmt.Fprint(w, "\n")
		}
	}
	for i := 0; i < l.height-1; i++ {
		fmt.Fprint(w, "\033[A")
	}
	fmt.Fprint(w, "\r")
	l.rendered = false
	l.height = 0
}

func (l *LiveSplit) combinedLinesLocked() []string {
	lines := []string{}
	if len(l.apiLines) > 0 {
		lines = append(lines, l.apiLines...)
	}
	if len(l.apiLines) > 0 && len(l.taskLines) > 0 {
		lines = append(lines, "")
	}
	if len(l.taskLines) > 0 {
		lines = append(lines, l.taskLines...)
	}
	return lines
}
