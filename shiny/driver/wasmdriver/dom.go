// +build js,wasm

package wasmdriver

import (
	"strings"
	"syscall/js"

	"golang.org/x/mobile/event/size"
	"golang.org/x/mobile/geom"
)

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
