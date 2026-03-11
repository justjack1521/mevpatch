package gui

import (
	"errors"
	"fmt"
	"gioui.org/app"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget/material"
	"github.com/justjack1521/mevpatch/internal/patch"
	"image/color"
	"os"
	"strings"
	"unicode"
)

var warning = color.NRGBA{0xff, 0x68, 0x59, 255}
var success = color.NRGBA{0x1e, 0xb9, 0x80, 255}

type C = layout.Context
type D = layout.Dimensions

type Window struct {
	application string
	version     patch.Version
	window      *app.Window

	title             string
	subtitle          string
	error             string
	primaryProgress   float32
	secondaryProgress float32

	updates <-chan PatchUpdate
}

func NewWindow(a string, v patch.Version, updates <-chan PatchUpdate) *Window {
	return &Window{application: a, version: v, updates: updates}
}

func (w *Window) Build() {

	var action = fmt.Sprintf("Patching %s to version %s", w.application, w.version.String())

	w.window = new(app.Window)
	w.window.Option(app.Title(fmt.Sprintf("Blank Project Patcher: %s", action)))
	w.window.Option(app.Size(unit.Dp(600), unit.Dp(150)))
	go w.listen()
	if err := w.draw(); err != nil {
		panic(err)
	}
	os.Exit(0)
}

func (w *Window) listen() {
	for update := range w.updates {
		switch actual := update.(type) {
		case StatusUpdate:
			w.handleStatusUpdate(actual)
		case ProgressUpdate:
			w.handleSecondaryProgressUpdate(actual)
		case ErrorUpdate:
			w.handleErrorUpdate(actual)
		}
	}
}

func (w *Window) handleStatusUpdate(u StatusUpdate) {
	if u.Primary != "" {
		w.title = u.Primary
	}
	w.subtitle = u.Secondary
	w.window.Invalidate()
}

func (w *Window) handleSecondaryProgressUpdate(u ProgressUpdate) {
	if u.ProgressUpdateType == ProgressUpdateTypePrimary {
		if u.Reset {
			w.primaryProgress = 0
		}
		w.primaryProgress += u.Value
	} else {
		if u.Reset {
			w.secondaryProgress = 0
		}
		w.secondaryProgress += u.Value
	}
	w.window.Invalidate()
}

func (w *Window) handleErrorUpdate(u ErrorUpdate) {
	w.error = w.FormatError(u.Value)
	w.window.Invalidate()
}

func (w *Window) FormatError(err error) string {
	var parts []string
	for err != nil {
		msg := err.Error()
		if i := strings.Index(msg, ": "); i != -1 {
			msg = msg[:i]
		}
		runes := []rune(msg)
		for i, r := range runes {
			if unicode.IsLetter(r) {
				runes[i] = unicode.ToUpper(r)
				break
			}
		}
		parts = append(parts, string(runes))
		err = errors.Unwrap(err)
	}
	return strings.Join(parts, ": ")
}

func (w *Window) draw() error {

	var ops op.Ops
	th := material.NewTheme()

	for {
		switch e := w.window.Event().(type) {
		case app.FrameEvent:
			gtx := app.NewContext(&ops, e)
			layout.Flex{
				Axis:    layout.Vertical,
				Spacing: layout.SpaceEvenly,
			}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					margins := layout.Inset{
						Top:    unit.Dp(15),
						Bottom: unit.Dp(15),
						Right:  unit.Dp(35),
						Left:   unit.Dp(35),
					}
					return margins.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						t := material.H6(th, fmt.Sprintf("%s...", w.title))
						return t.Layout(gtx)
					})
				}),
				layout.Rigid(func(gtx C) D {
					margins := layout.Inset{
						Top:    unit.Dp(0),
						Bottom: unit.Dp(0),
						Right:  unit.Dp(35),
						Left:   unit.Dp(35),
					}
					return margins.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						s := material.Label(th, unit.Sp(15), w.subtitle)
						return s.Layout(gtx)
					})
				}),
				layout.Rigid(func(gtx C) D {
					margins := layout.Inset{
						Top:   unit.Dp(0),
						Right: unit.Dp(35),
						Left:  unit.Dp(35),
					}
					return margins.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						s := material.Label(th, unit.Sp(15), w.error)
						s.Color = warning
						return s.Layout(gtx)
					})
				}),
				layout.Rigid(
					func(gtx C) D {
						margins := layout.Inset{
							Top:    unit.Dp(5),
							Bottom: unit.Dp(5),
							Right:  unit.Dp(35),
							Left:   unit.Dp(35),
						}
						return margins.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							bar := material.ProgressBar(th, w.primaryProgress)
							return bar.Layout(gtx)
						})
					},
				),
				layout.Rigid(
					func(gtx C) D {
						margins := layout.Inset{
							Top:    unit.Dp(5),
							Bottom: unit.Dp(5),
							Right:  unit.Dp(35),
							Left:   unit.Dp(35),
						}
						return margins.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							bar := material.ProgressBar(th, w.secondaryProgress)
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
