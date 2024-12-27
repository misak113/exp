// +build js

package webgldriver

import (
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
	width         int
	height        int
}

func newWindow(screen *screenImpl, opts *screen.NewWindowOptions) *windowImpl {
	canvasEl := screen.doc.Call("createElement", "canvas")
	screen.doc.Get("body").Call("appendChild", canvasEl)

	gl, err := webgl.FromCanvas(canvasEl)
	if err != nil {
		panic(err)
	}

	domEvents := dom.NewDomEvents()

	w := &windowImpl{
		screen:    screen,
		canvasEl:  canvasEl,
		gl:        gl,
		domEvents: domEvents,
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
