package ui

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

type Spinner struct {
	ui      *UI
	msg     string
	done    chan struct{}
	stopped chan struct{}
	active  bool
	once    sync.Once
}

func (u *UI) StartSpinner(msg string) *Spinner {
	sp := &Spinner{
		ui:      u,
		msg:     msg,
		done:    make(chan struct{}),
		stopped: make(chan struct{}),
		active:  u.UseColor,
	}
	if !sp.active {
		return sp
	}

	go sp.loop()
	return sp
}

func (s *Spinner) Stop() {
	s.once.Do(func() {
		if !s.active {
			return
		}
		close(s.done)
		<-s.stopped
	})
}

func (s *Spinner) loop() {
	defer close(s.stopped)

	frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	ticker := time.NewTicker(120 * time.Millisecond)
	defer ticker.Stop()

	i := 0
	for {
		select {
		case <-s.done:
			s.clear()
			return
		case <-ticker.C:
			frame := frames[i%len(frames)]
			i++
			s.render(frame)
		}
	}
}

func (s *Spinner) render(frame string) {
	icon := frame
	msg := s.msg
	if s.ui.UseColor {
		icon = s.ui.style(s.ui.styles.Info, frame)
		msg = s.ui.style(s.ui.styles.Dim, msg)
	}
	fmt.Fprintf(s.ui.Out, "\r%s %s", icon, msg)
}

func (s *Spinner) clear() {
	width := len(s.msg) + 4
	fmt.Fprintf(s.ui.Out, "\r%s\r", strings.Repeat(" ", width))
}
