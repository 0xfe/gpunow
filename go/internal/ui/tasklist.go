package ui

import (
	"fmt"
	"sync"
	"time"
)

type TaskList struct {
	ui      *UI
	area    *LiveArea
	split   *LiveSplit
	label   string
	items   []taskItem
	mu      sync.RWMutex
	done    chan struct{}
	stopped chan struct{}
	active  bool
	once    sync.Once
}

type taskItem struct {
	name    string
	percent int32
	message string
}

func (u *UI) TaskList(label string, names []string) *TaskList {
	items := make([]taskItem, len(names))
	for i, name := range names {
		items[i] = taskItem{name: name}
	}
	var area *LiveArea
	var split *LiveSplit
	if current := u.currentSplit(); current != nil && current.Active() {
		split = current
	} else {
		area = u.LiveArea(len(names))
	}
	t := &TaskList{
		ui:      u,
		area:    area,
		split:   split,
		label:   label,
		items:   items,
		done:    make(chan struct{}),
		stopped: make(chan struct{}),
		active:  (split != nil && split.Active()) || (area != nil && area.Active()),
	}
	if t.active {
		go t.loop()
	}
	return t
}

func (t *TaskList) Update(index int, percent int32) {
	if t == nil || index < 0 || index >= len(t.items) {
		return
	}
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}
	t.mu.Lock()
	t.items[index].percent = percent
	t.mu.Unlock()
}

func (t *TaskList) MarkDone(index int, message string) {
	if t == nil || index < 0 || index >= len(t.items) {
		return
	}
	if message == "" {
		return
	}
	icon := icon(t.ui.UseColor, iconCheck, "+")
	line := fmt.Sprintf("%s %s", icon, message)
	if t.ui.UseColor {
		line = t.ui.style(t.ui.styles.Success, line)
	}
	t.setMessage(index, line)
}

func (t *TaskList) MarkWarning(index int, message string) {
	if t == nil || index < 0 || index >= len(t.items) {
		return
	}
	if message == "" {
		return
	}
	icon := icon(t.ui.UseColor, iconWarn, "!")
	line := fmt.Sprintf("%s %s", icon, message)
	if t.ui.UseColor {
		line = t.ui.style(t.ui.styles.Warn, line)
	}
	t.setMessage(index, line)
}

func (t *TaskList) setMessage(index int, line string) {
	t.mu.Lock()
	t.items[index].message = line
	t.mu.Unlock()
}

func (t *TaskList) Stop() {
	if t == nil {
		return
	}
	t.once.Do(func() {
		if !t.active {
			return
		}
		close(t.done)
		<-t.stopped
		if t.area != nil {
			t.area.Stop()
		}
	})
}

func (t *TaskList) Clear() {
	if t == nil || !t.active {
		return
	}
	if t.split != nil {
		t.split.ClearTasks()
		return
	}
	if t.area != nil {
		t.area.SetLines(nil)
	}
}

func (t *TaskList) loop() {
	defer close(t.stopped)

	frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	ticker := time.NewTicker(300 * time.Millisecond)
	defer ticker.Stop()

	i := 0
	for {
		select {
		case <-t.done:
			t.render("")
			return
		case <-ticker.C:
			frame := frames[i%len(frames)]
			i++
			t.render(frame)
		}
	}
}

func (t *TaskList) render(frame string) {
	if t == nil || !t.active {
		return
	}
	t.mu.RLock()
	items := make([]taskItem, len(t.items))
	copy(items, t.items)
	t.mu.RUnlock()

	lines := make([]string, 0, len(items))
	icon := frame
	label := t.label
	if t.ui.UseColor && frame != "" {
		icon = t.ui.style(t.ui.styles.Info, frame)
		label = t.ui.style(t.ui.styles.Dim, label)
	}
	for i := range items {
		if items[i].message != "" {
			lines = append(lines, items[i].message)
			continue
		}
		line := fmt.Sprintf("%s %s", label, items[i].name)
		if icon != "" {
			line = fmt.Sprintf("%s %s", line, icon)
		}
		lines = append(lines, line)
	}
	if t.split != nil {
		t.split.SetTaskLines(lines)
		return
	}
	if t.area != nil {
		t.area.SetLines(lines)
	}
}
