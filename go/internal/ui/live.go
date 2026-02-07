package ui

import (
	"fmt"
	"io"
	"sync"
)

type LiveArea struct {
	ui       *UI
	lines    []string
	height   int
	rendered bool
	active   bool
	mu       sync.Mutex
}

func newLiveArea(ui *UI, lines int) *LiveArea {
	active := ui.UseColor && lines > 0
	return &LiveArea{
		ui:     ui,
		lines:  make([]string, lines),
		height: lines,
		active: active,
	}
}

func (l *LiveArea) Active() bool {
	return l != nil && l.active
}

func (l *LiveArea) SetLines(lines []string) {
	if l == nil || !l.active {
		return
	}
	l.mu.Lock()
	l.lines = append(l.lines[:0], lines...)
	l.height = len(lines)
	l.mu.Unlock()
	l.render()
}

func (l *LiveArea) Stop() {
	if l == nil || !l.active {
		return
	}
	l.ui.lockWrite()
	defer l.ui.unlockWrite()
	l.mu.Lock()
	if l.rendered && l.height > 0 {
		fmt.Fprint(l.ui.Out, "\n")
	}
	l.rendered = false
	l.height = 0
	l.active = false
	l.mu.Unlock()
	l.ui.clearLive(l)
}

func (l *LiveArea) WriteLineLocked(w io.Writer, text string) {
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

func (l *LiveArea) render() {
	l.ui.lockWrite()
	defer l.ui.unlockWrite()
	l.mu.Lock()
	defer l.mu.Unlock()
	l.renderLocked(l.ui.Out)
}

func (l *LiveArea) renderLocked(w io.Writer) {
	if !l.active {
		return
	}
	if l.height == 0 {
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
	for i := 0; i < l.height; i++ {
		fmt.Fprint(w, "\r\033[2K")
		if i < len(l.lines) {
			fmt.Fprint(w, l.lines[i])
		}
		if i < l.height-1 {
			fmt.Fprint(w, "\n")
		}
	}
	l.rendered = true
}

func (l *LiveArea) clearLocked(w io.Writer) {
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
}
