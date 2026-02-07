package ui

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type Progress struct {
	ui      *UI
	label   string
	total   int64
	current int64
	lastLen int64

	done    chan struct{}
	stopped chan struct{}
	active  bool
	once    sync.Once
}

func (u *UI) Progress(total int, label string) *Progress {
	p := &Progress{
		ui:      u,
		label:   label,
		total:   int64(total),
		done:    make(chan struct{}),
		stopped: make(chan struct{}),
		active:  u.UseColor && total > 0,
	}
	if !p.active {
		return p
	}

	go p.loop()
	return p
}

func (p *Progress) Increment() {
	if p == nil {
		return
	}
	atomic.AddInt64(&p.current, 1)
}

func (p *Progress) Done() {
	if p == nil {
		return
	}
	p.once.Do(func() {
		if !p.active {
			return
		}
		atomic.StoreInt64(&p.current, p.total)
		close(p.done)
		<-p.stopped
	})
}

func (p *Progress) loop() {
	defer close(p.stopped)

	frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	ticker := time.NewTicker(120 * time.Millisecond)
	defer ticker.Stop()

	i := 0
	for {
		select {
		case <-p.done:
			p.clear()
			return
		case <-ticker.C:
			frame := frames[i%len(frames)]
			i++
			p.render(frame)
		}
	}
}

func (p *Progress) render(frame string) {
	current := atomic.LoadInt64(&p.current)
	total := atomic.LoadInt64(&p.total)
	percent := int64(0)
	if total > 0 {
		percent = current * 100 / total
		if percent > 100 {
			percent = 100
		}
	}

	icon := frame
	label := p.label
	if p.ui.UseColor {
		icon = p.ui.style(p.ui.styles.Info, frame)
		label = p.ui.style(p.ui.styles.Dim, label)
	}

	text := fmt.Sprintf("%s %s %d/%d %d%%", icon, label, current, total, percent)
	atomic.StoreInt64(&p.lastLen, int64(plainLen(p.label, current, total, percent)))
	fmt.Fprintf(p.ui.Out, "\r%s", text)
}

func (p *Progress) clear() {
	width := int(atomic.LoadInt64(&p.lastLen))
	if width == 0 {
		width = len(p.label) + 20
	}
	fmt.Fprintf(p.ui.Out, "\r%s\r", strings.Repeat(" ", width))
}

func plainLen(label string, current, total, percent int64) int {
	return len(label) + len(strconv.FormatInt(current, 10)) + len(strconv.FormatInt(total, 10)) + len(strconv.FormatInt(percent, 10)) + 10
}
