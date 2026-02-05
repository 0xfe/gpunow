package ui

import (
	"time"

	"github.com/vbauerster/mpb/v8"
	"github.com/vbauerster/mpb/v8/decor"
)

type Progress struct {
	bar   *mpb.Bar
	total int64
}

func (u *UI) Progress(total int, label string) *Progress {
	if !u.UseColor || total <= 0 {
		return &Progress{bar: nil}
	}

	p := mpb.New(mpb.WithOutput(u.Out), mpb.WithRefreshRate(120*time.Millisecond))
	bar := p.New(int64(total),
		mpb.BarStyle().Rbound("|").Lbound("|").Filler("=").Tip("=").Padding("."),
		mpb.PrependDecorators(
			decor.Name(label+" "),
			decor.CountersNoUnit("%d/%d"),
		),
		mpb.AppendDecorators(
			decor.Percentage(),
		),
	)

	return &Progress{bar: bar, total: int64(total)}
}

func (p *Progress) Increment() {
	if p == nil || p.bar == nil {
		return
	}
	p.bar.Increment()
}

func (p *Progress) Done() {
	if p == nil || p.bar == nil {
		return
	}
	p.bar.SetTotal(p.total, true)
}
