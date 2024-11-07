package gui

import (
	"fmt"
	"gioui.org/app"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget/material"
	"github.com/justjack1521/mevpatch/internal/patch"
	"image/color"
	"os"
)

var warning = color.NRGBA{0xff, 0x68, 0x59, 255}
var success = color.NRGBA{0x1e, 0xb9, 0x80, 255}

var progress float32
var title string
var subtitle string

type C = layout.Context
type D = layout.Dimensions

type Window struct {
	application string
	version     patch.Version
	window      *app.Window
	incrementer chan float32
	broadcaster chan string
	catcher     chan error
}

func NewWindow(a string, v patch.Version, i chan float32, b chan string, c chan error) *Window {
	return &Window{application: a, version: v, incrementer: i, broadcaster: b, catcher: c}
}

func (w *Window) Build() {

	var action = fmt.Sprintf("Patching %s to version %s", w.application, w.version.String())

	w.window = new(app.Window)
	w.window.Option(app.Title(fmt.Sprintf("Blank Project Patcher: %s", action)))
	w.window.Option(app.Size(unit.Dp(600), unit.Dp(150)))
	if err := w.draw(); err != nil {
		panic(err)
	}
	os.Exit(0)
}

func (w *Window) draw() error {

	var ops op.Ops
	th := material.NewTheme()

	go func() {
		for p := range w.incrementer {
			if progress < 1 {
				progress += p
				w.window.Invalidate()
			}
		}
	}()

	go func() {
		for b := range w.broadcaster {
			title = b
			w.window.Invalidate()
		}
	}()

	go func() {
		for s := range w.catcher {
			subtitle = s.Error()
			w.window.Invalidate()
		}
	}()

	for {
		switch e := w.window.Event().(type) {
		case app.FrameEvent:
			gtx := app.NewContext(&ops, e)
			layout.Flex{
				Axis:    layout.Vertical,
				Spacing: layout.SpaceBetween,
			}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					margins := layout.Inset{
						Top:    unit.Dp(25),
						Bottom: unit.Dp(25),
						Right:  unit.Dp(35),
						Left:   unit.Dp(35),
					}
					return margins.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						t := material.H6(th, title)
						return t.Layout(gtx)
					})
				}),
				layout.Rigid(func(gtx C) D {
					margins := layout.Inset{
						Right: unit.Dp(35),
						Left:  unit.Dp(35),
					}
					return margins.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						s := material.Label(th, unit.Sp(15), subtitle)
						s.Color = warning
						return s.Layout(gtx)
					})
				}),
				layout.Rigid(
					func(gtx C) D {
						margins := layout.Inset{
							Top:    unit.Dp(25),
							Bottom: unit.Dp(25),
							Right:  unit.Dp(35),
							Left:   unit.Dp(35),
						}
						return margins.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							bar := material.ProgressBar(th, progress)
							return bar.Layout(gtx)
						})
					},
				),
			)
			e.Frame(gtx.Ops)
		case app.DestroyEvent:
			return e.Err
		}
	}

}
