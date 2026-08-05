// +build js

package canvasdriver

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"sync"
	"syscall/js"

	"golang.org/x/exp/shiny/driver/util/dom"
	"golang.org/x/exp/shiny/screen"
	"golang.org/x/image/math/f64"
)

type windowImpl struct {
	screen *screenImpl
	// internal
	mutex *sync.Mutex
	// state
	canvasEl       js.Value
	ctx2d          js.Value
	released       bool
	domEvents      *dom.DomEvents
	resizeCallback js.Func
}

func newWindow(screen *screenImpl, opts *screen.NewWindowOptions) *windowImpl {
	canvasEl := screen.doc.Call("createElement", "canvas")
	screen.doc.Get("body").Call("appendChild", canvasEl)

	// When an explicit geometry is requested via options, the canvas is
	// positioned at that location instead of covering the whole viewport.
	hasGeometry := opts != nil && opts.Width > 0 && opts.Height > 0
	if hasGeometry {
		style := canvasEl.Get("style")
		style.Set("position", "absolute")
		style.Set("left", fmt.Sprintf("%dpx", opts.X))
		style.Set("top", fmt.Sprintf("%dpx", opts.Y))
		canvasEl.Set("width", opts.Width)
		canvasEl.Set("height", opts.Height)
	}

	adaptCanvas := func() {
		if hasGeometry {
			return
		}
		// Use the document viewport size directly. It must stay in sync with
		// dom.emitSizeEvent(), which reports the very same values. Do NOT divide
		// by dom.GetBrowserZoomRatio() here — that heuristic compares innerWidth
		// against outerWidth, which is not comparable under Chrome device
		// emulation or inside an iframe (outerWidth always refers to the real
		// top-level browser window), and it silently scaled the canvas down.
		width := dom.GetDocWidth()
		if canvasEl.Get("width").Int() != width {
			canvasEl.Set("width", width)
		}
		height := dom.GetDocHeight()
		if canvasEl.Get("height").Int() != height {
			canvasEl.Set("height", height)
		}
	}
	adaptCanvas()
	resizeCallback := js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		adaptCanvas()
		return nil
	})
	js.Global().Call("addEventListener", "resize", resizeCallback)

	if opts.Width != 0 {
		dom.SetWindowWidth(opts.Width)
	}
	if opts.Height != 0 {
		dom.SetWindowHeight(opts.Height)
	}

	if opts.Title != "" {
		screen.doc.Get("head").Call("getElementsByTagName", "title").Call("item", 0).Set("innerHTML", opts.Title)
	}

	ctx2d := canvasEl.Call("getContext", "2d")
	if ctx2d.IsUndefined() || ctx2d.IsNull() {
		panic(fmt.Errorf("Cannot get 2d context of canvas"))
	}

	domEvents := dom.NewDomEvents()

	w := &windowImpl{
		screen:         screen,
		mutex:          &sync.Mutex{},
		canvasEl:       canvasEl,
		ctx2d:          ctx2d,
		domEvents:      domEvents,
		resizeCallback: resizeCallback,
	}

	domEvents.BindEvents()

	return w
}

// Window methods

func (w *windowImpl) Release() {
	w.mutex.Lock()
	defer w.mutex.Unlock()
	if w.released {
		return
	}

	js.Global().Call("removeEventListener", "resize", w.resizeCallback)
	w.resizeCallback.Release()

	w.canvasEl.Call("remove")

	w.domEvents.Release()

	w.released = true
}

func (w *windowImpl) Publish() screen.PublishResult {
	if w.released {
		return screen.PublishResult{false}
	}
	// swap buffers (not implemented in Canvas)
	// by default, it's swapped automatically
	return screen.PublishResult{false}
}

// EventDeque methods

func (w *windowImpl) Send(event interface{}) {
	if w.released {
		return
	}
	panic("Not implemented")
}

func (w *windowImpl) SendFirst(event interface{}) {
	if w.released {
		return
	}
	panic("Not implemented")
}

func (w *windowImpl) NextEvent() interface{} {
	ev := <-w.domEvents.GetEventChan()
	return ev
}

// Uploader methods

func (w *windowImpl) Upload(dp image.Point, src screen.Buffer, sr image.Rectangle) {
	if w.released {
		return
	}

	panic(fmt.Errorf("Not implemented, use UploadYCbCr instead"))
}

func (w *windowImpl) UploadYCbCr(dp image.Point, src screen.Buffer, sr image.Rectangle) {
	if w.released {
		return
	}

	// rendering of JS ArrayBuffer YCbCr image to reduce copying between JS and Go
	switch jsSrc := src.(type) {
	case *BufferImpl:
		w.ctx2d.Call("drawImage", *jsSrc.YCbCrJS().CanvasImageSource, 0, 0)
		return
	}
	panic(fmt.Errorf("Not implemented, use canvasdriver.BufferImpl instead"))
}

func (w *windowImpl) Fill(dr image.Rectangle, src color.Color, op draw.Op) {
	if w.released {
		return
	}
	panic("Not implemented")
}

// Drawer methods

func (w *windowImpl) Draw(src2dst f64.Aff3, src screen.Texture, sr image.Rectangle, op draw.Op, opts *screen.DrawOptions) {
	if w.released {
		return
	}
	panic("Not implemented")
}

func (w *windowImpl) DrawUniform(src2dst f64.Aff3, src color.Color, sr image.Rectangle, op draw.Op, opts *screen.DrawOptions) {
	if w.released {
		return
	}
	panic("Not implemented")
}

func (w *windowImpl) Copy(dp image.Point, src screen.Texture, sr image.Rectangle, op draw.Op, opts *screen.DrawOptions) {
	if w.released {
		return
	}
	panic("Not implemented")
}

func (w *windowImpl) Scale(dr image.Rectangle, src screen.Texture, sr image.Rectangle, op draw.Op, opts *screen.DrawOptions) {
	if w.released {
		return
	}
	panic("Not implemented")
}
