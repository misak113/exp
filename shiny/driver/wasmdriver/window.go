// +build js,wasm

package wasmdriver

import (
	"context"
	"image"
	"image/color"
	"image/draw"
	"sync"
	"syscall/js"
	"time"

	"github.com/nuberu/webgl"
	"github.com/nuberu/webgl/types"
	"golang.org/x/exp/shiny/imageutil"
	"golang.org/x/exp/shiny/screen"
	"golang.org/x/image/math/f64"
)

type windowImpl struct {
	screen *screenImpl
	width  int
	height int
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
}

func newWindow(screen *screenImpl, opts *screen.NewWindowOptions) *windowImpl {
	canvasEl := screen.doc.Call("createElement", "canvas")
	screen.doc.Get("body").Call("appendChild", canvasEl)

	width := opts.Width
	if opts.Width == 0 {
		width = screen.doc.Get("documentElement").Get("clientWidth").Int()
	}
	canvasEl.Set("width", width)

	height := opts.Height
	if opts.Height == 0 {
		height = screen.doc.Get("documentElement").Get("clientHeight").Int()
	}
	canvasEl.Set("height", height)

	if opts.Title != "" {
		screen.doc.Get("head").Call("getElementsByTagName", "title").Call("item", 0).Set("innerHTML", opts.Title)
	}

	gl, err := webgl.FromCanvas(canvasEl)
	if err != nil {
		panic(err)
	}

	w := &windowImpl{
		screen:   screen,
		width:    width,
		height:   height,
		canvasEl: canvasEl,
		gl:       gl,
	}

	// RGBA program
	w.programRGBA, err = w.createAndLinkProgramRGBA()
	if err != nil {
		panic(err)
	}
	w.imageTexRGBA = w.createTexture(textureUnitRGBA, webgl.RGBA, w.width, w.height)

	// YUV 420 program
	w.programYUV420, err = w.createAndLinkProgramYUV420()
	if err != nil {
		panic(err)
	}
	w.imageTexY = w.createTexture(textureUnitY, webgl.LUMINANCE, w.width, w.height)
	w.imageTexU = w.createTexture(textureUnitU, webgl.LUMINANCE, w.width/2, w.height/2)
	w.imageTexV = w.createTexture(textureUnitV, webgl.LUMINANCE, w.width/2, w.height/2)

	// General
	w.vertexArray = w.createBuffers()

	w.gl.Enable(webgl.DEPTH_TEST)
	w.gl.Viewport(0, 0, w.width, w.height)
	w.clear()

	return w
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
	//panic("Not implemented")
	time.Sleep(1 * time.Hour)
	return <-context.Background().Done()
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
