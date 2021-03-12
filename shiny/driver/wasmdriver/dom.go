// +build js,wasm

package wasmdriver

import (
	"strings"
	"syscall/js"

	"golang.org/x/mobile/event/key"
	"golang.org/x/mobile/event/mouse"
	"golang.org/x/mobile/event/size"
	"golang.org/x/mobile/geom"
)

const mobileMouseButtonBack mouse.Button = 8
const mobileMouseButtonForward mouse.Button = 9

func getDocWidth() int {
	return js.Global().Get("innerWidth").Int()
}

func getDocHeight() int {
	return js.Global().Get("innerHeight").Int()
}

func getOrientation() (orientation size.Orientation) {
	defer func() {
		if recover() != nil {
			orientation = size.OrientationUnknown
		}
	}()
	orientationType := js.Global().Get("screen").Get("orientation").Get("type").String()
	if strings.HasPrefix(orientationType, "landscape") {
		orientation = size.OrientationLandscape
		return
	}
	if strings.HasPrefix(orientationType, "portrait") {
		orientation = size.OrientationLandscape
		return
	}
	orientation = size.OrientationUnknown
	return
}

func (w *windowImpl) bindSizeEvents() {
	onResize := js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		w.emitSizeEvent()
		return nil
	})
	js.Global().Call("addEventListener", "resize", onResize)
	w.releases = append(w.releases, func() {
		js.Global().Call("removeEventListener", "resize", onResize)
		onResize.Release()
	})
}

func (w *windowImpl) emitSizeEvent() {
	orientation := getOrientation()
	// TODO(nigeltao): don't assume 72 DPI. DisplayWidth and DisplayWidthMM
	// is probably the best place to start looking.
	pixelsPerPt := float32(1)
	width := getDocWidth()
	height := getDocHeight()
	w.eventChan <- size.Event{
		WidthPx:     width,
		HeightPx:    height,
		WidthPt:     geom.Pt(width / int(pixelsPerPt)),
		HeightPt:    geom.Pt(height / int(pixelsPerPt)),
		PixelsPerPt: pixelsPerPt,
		Orientation: orientation,
	}
}

func (w *windowImpl) bindMouseEvents() {
	// move
	onMove := js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		w.eventChan <- mouse.Event{
			X:         float32(args[0].Get("offsetX").Float()),
			Y:         float32(args[0].Get("offsetY").Float()),
			Button:    mouse.ButtonNone,
			Direction: mouse.DirNone,
			Modifiers: getEventModifiers(args[0]),
		}
		return nil
	})
	js.Global().Call("addEventListener", "mousemove", onMove)

	// press/release
	onClick := js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		w.eventChan <- mouse.Event{
			X:         float32(args[0].Get("offsetX").Float()),
			Y:         float32(args[0].Get("offsetY").Float()),
			Button:    getMouseButton(args[0]),
			Direction: getMouseDirection(args[0]),
			Modifiers: getEventModifiers(args[0]),
		}
		return nil
	})
	js.Global().Call("addEventListener", "mousedown", onClick)
	js.Global().Call("addEventListener", "mouseup", onClick)

	// wheel
	onWheel := js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		w.eventChan <- mouse.Event{
			X:         float32(args[0].Get("offsetX").Float()),
			Y:         float32(args[0].Get("offsetY").Float()),
			Button:    getWheelButton(args[0]),
			Direction: mouse.DirStep,
			Modifiers: getEventModifiers(args[0]),
		}
		return nil
	})
	js.Global().Call("addEventListener", "wheel", onWheel)

	w.releases = append(w.releases, func() {
		js.Global().Call("removeEventListener", "mousemove", onMove)
		js.Global().Call("removeEventListener", "mousedown", onClick)
		js.Global().Call("removeEventListener", "mouseup", onClick)
		js.Global().Call("removeEventListener", "wheel", onWheel)
		onMove.Release()
		onClick.Release()
		onWheel.Release()
	})
}

func getMouseDirection(ev js.Value) (dir mouse.Direction) {
	dir = mouse.DirNone
	if ev.Get("type").String() == "mousedown" {
		dir = mouse.DirPress
	}
	if ev.Get("type").String() == "mouseup" {
		dir = mouse.DirRelease
	}
	return
}

func getWheelButton(ev js.Value) mouse.Button {
	deltaY := ev.Get("deltaY").Int()
	deltaX := ev.Get("deltaX").Int()

	if deltaY > 0 {
		return mouse.ButtonWheelDown
	}
	if deltaY < 0 {
		return mouse.ButtonWheelUp
	}
	if deltaX > 0 {
		return mouse.ButtonWheelRight
	}
	if deltaX < 0 {
		return mouse.ButtonWheelLeft
	}
	return mouse.ButtonNone
}

const (
	domMouseLeft    = 0
	domMouseMiddle  = 1
	domMouseRight   = 2
	domMouseBack    = 3
	domMouseForward = 4
)

func getMouseButton(ev js.Value) mouse.Button {
	switch ev.Get("button").Int() {
	case domMouseLeft:
		return mouse.ButtonLeft
	case domMouseMiddle:
		return mouse.ButtonMiddle
	case domMouseRight:
		return mouse.ButtonRight
	case domMouseBack:
		return mobileMouseButtonBack
	case domMouseForward:
		return mobileMouseButtonForward
	default:
		return mouse.ButtonNone
	}
}

func getEventModifiers(ev js.Value) (mod key.Modifiers) {
	if ev.Get("altKey").Bool() {
		mod |= key.ModAlt
	}
	if ev.Get("ctrlKey").Bool() {
		mod |= key.ModControl
	}
	if ev.Get("shiftKey").Bool() {
		mod |= key.ModShift
	}
	if ev.Get("metaKey").Bool() {
		mod |= key.ModMeta
	}

	return
}
