//go:build js
// +build js

package webgldriver

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"sync"
	"syscall/js"

	"github.com/nuberu/webgl"
	"github.com/nuberu/webgl/types"
	"golang.org/x/exp/shiny/driver/util/dom"
	"golang.org/x/exp/shiny/imageutil"
	"golang.org/x/exp/shiny/screen"
	"golang.org/x/image/math/f64"
)

type windowImpl struct {
	screen *screenImpl
	// internal
	mutex *sync.Mutex
	// state
	canvasEl      js.Value
	gl            *webgl.RenderingContext
	programRGBA   *types.Program
	imageTexRGBA  *types.Texture
	programYUV420 *types.Program
	imageTexY     *types.Texture
	imageTexU     *types.Texture
	imageTexV     *types.Texture
	vertexArray   *types.VertexArray
	released      bool
	domEvents     *dom.DomEvents
	frontEvents   []interface{}
	backEvents    []interface{}
	eventWake     chan struct{}
	width         int
	height        int
}

func newWindow(screen *screenImpl, opts *screen.NewWindowOptions) *windowImpl {
	canvasEl := screen.doc.Call("createElement", "canvas")
	screen.doc.Get("body").Call("appendChild", canvasEl)

	// When an explicit geometry is requested via options, the canvas is
	// positioned at that location instead of covering the whole viewport.
	if opts != nil && opts.Width > 0 && opts.Height > 0 {
		style := canvasEl.Get("style")
		style.Set("position", "absolute")
		style.Set("left", fmt.Sprintf("%dpx", opts.X))
		style.Set("top", fmt.Sprintf("%dpx", opts.Y))
		style.Set("width", fmt.Sprintf("%dpx", opts.Width))
		style.Set("height", fmt.Sprintf("%dpx", opts.Height))
	}

	gl, err := webgl.FromCanvas(canvasEl)
	if err != nil {
		panic(err)
	}

	domEvents := dom.NewDomEventsForTarget(canvasEl)

	w := &windowImpl{
		screen:    screen,
		mutex:     &sync.Mutex{},
		canvasEl:  canvasEl,
		gl:        gl,
		domEvents: domEvents,
		eventWake: make(chan struct{}, 1),
	}

	if opts.Width != 0 {
		dom.SetWindowWidth(opts.Width)
	}
	if opts.Height != 0 {
		dom.SetWindowHeight(opts.Height)
	}

	if opts.Title != "" {
		screen.doc.Get("head").Call("getElementsByTagName", "title").Call("item", 0).Set("innerHTML", opts.Title)
	}

	// General
	w.vertexArray = w.createBuffers()

	domEvents.BindEvents()

	return w
}

func (w *windowImpl) ensureCanvasSize(width int, height int) {
	// TODO consider using different width/height, this come from SR which is size of the buffer to draw
	// but we should probably match canvas size to window size instead. Decoded frame can be larger due to devicePixelRatio
	// and we want to render scaled down to current screen size. Same is done for canvas in canvasdriver impl.
	if w.canvasEl.Get("width").Int() != width {
		w.canvasEl.Set("width", width)
	}
	if w.canvasEl.Get("height").Int() != height {
		w.canvasEl.Set("height", height)
	}
}

func (w *windowImpl) clear() {
	w.gl.ClearColor(0.0, 0.0, 0.0, 1.0)
	w.gl.Clear(uint32(webgl.COLOR_BUFFER_BIT))
}

// Window methods

func (w *windowImpl) Release() {
	w.mutex.Lock()
	defer w.mutex.Unlock()
	if w.released {
		return
	}

	w.canvasEl.Call("remove")

	w.domEvents.Release()

	w.released = true
}

func (w *windowImpl) Publish() screen.PublishResult {
	if w.released {
		return screen.PublishResult{false}
	}
	// swap buffers (not implemented in WebGL)
	// by default, it's swapped automatically
	return screen.PublishResult{false}
}

// EventDeque methods

func (w *windowImpl) Send(event interface{}) {
	w.mutex.Lock()
	if w.released {
		w.mutex.Unlock()
		return
	}
	eventChan := w.domEvents.GetEventChan()
	w.mutex.Unlock()
	eventChan <- event
}

func (w *windowImpl) SendNonBlocking(event interface{}) bool {
	w.mutex.Lock()
	defer w.mutex.Unlock()
	if w.released {
		return false
	}
	select {
	case w.domEvents.GetEventChan() <- event:
		return true
	default:
		return false
	}
}

func (w *windowImpl) SendFirst(event interface{}) {
	w.mutex.Lock()
	defer w.mutex.Unlock()
	if w.released {
		return
	}

	w.frontEvents = append(w.frontEvents, event)
	select {
	case w.eventWake <- struct{}{}:
	default:
	}
}

func (w *windowImpl) NextEvent() interface{} {
	for {
		w.mutex.Lock()
		if event, ok := w.nextQueuedEvent(); ok {
			w.mutex.Unlock()
			return event
		}
		w.mutex.Unlock()

		select {
		case event := <-w.domEvents.GetEventChan():
			w.mutex.Lock()
			// A concurrent SendFirst can drain later channel events before this
			// receive acquires the mutex, so preserve this event ahead of them.
			w.backEvents = append(w.backEvents, nil)
			copy(w.backEvents[1:], w.backEvents[:len(w.backEvents)-1])
			w.backEvents[0] = event
			w.mutex.Unlock()
		case <-w.eventWake:
		}
	}
}

func (w *windowImpl) nextQueuedEvent() (interface{}, bool) {
	if last := len(w.frontEvents) - 1; last >= 0 {
		event := w.frontEvents[last]
		w.frontEvents[last] = nil
		w.frontEvents = w.frontEvents[:last]
		return event, true
	}
	if len(w.backEvents) > 0 {
		event := w.backEvents[0]
		w.backEvents[0] = nil
		w.backEvents = w.backEvents[1:]
		return event, true
	}
	return nil, false
}

// Uploader methods

func (w *windowImpl) Upload(dp image.Point, src screen.Buffer, sr image.Rectangle) {
	if w.released {
		return
	}

	w.drawBufferRGBA(dp, src, sr)
}

func (w *windowImpl) UploadYCbCr(dp image.Point, src screen.Buffer, sr image.Rectangle) {
	if w.released {
		return
	}

	// rendering of JS ArrayBuffer YCbCr image to reduce copying between JS and Go
	switch jsSrc := src.(type) {
	case *BufferImpl:
		if jsSrc.YCbCrJS().Use {
			if jsSrc.YCbCrJS().SubsampleRatio != image.YCbCrSubsampleRatio420 {
				panic("Only image.YCbCrSubsampleRatio420 SubsampleRatio is currently supported")
			}
			w.drawBufferYUV420JSArrayBuffers(dp, jsSrc.YCbCrJS().Y.slice(), jsSrc.YCbCrJS().Cb.slice(), jsSrc.YCbCrJS().Cr.slice(), sr)
			return
		}
	}

	if src.YCbCr().SubsampleRatio == image.YCbCrSubsampleRatio420 {
		// currently only YUV 420 format is accelerated on GPU
		w.drawBufferYUV420(dp, src, sr)
	} else {
		if len(src.RGBA().Pix) == 0 {
			imageutil.ConvertYCbCrToRGBA(src)
		}
		w.drawBufferRGBA(dp, src, sr)
	}
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
