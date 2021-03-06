// +build js,wasm

package wasmdriver

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"sync"
	"syscall/js"
	"time"
	"unsafe"

	"github.com/nuberu/webgl"
	"github.com/nuberu/webgl/types"
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
	canvasEl    js.Value
	gl          *webgl.RenderingContext
	imageTex    *types.Texture
	vertexArray *types.VertexArray
	released    bool
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

	program, err := w.createAndUseProgram()
	if err != nil {
		panic(err)
	}
	w.vertexArray = w.createBuffers(program)
	w.imageTex = w.createTexture()
	w.clear()

	w.gl.Enable(webgl.DEPTH_TEST)
	w.gl.Viewport(0, 0, w.width, w.height)

	return w
}

const vertexShaderCode = `
attribute vec2 a_postion;
attribute vec2 a_texCoord;
varying vec2 v_texCoord;

void main() {
	gl_Position = vec4(a_postion, 0.0, 1.0);
	v_texCoord = a_texCoord;
}
`

const fragmentShaderCode = `
precision mediump float;
varying vec2 v_texCoord;
uniform sampler2D u_image;

void main(void) {
	gl_FragColor = texture2D(u_image, v_texCoord);
}
`

const aPositionIndex = 0
const aTexCoordsIndex = 1

func (w *windowImpl) createAndUseProgram() (*types.Program, error) {
	vertShader := w.gl.CreateVertexShader()
	w.gl.ShaderSource(vertShader, vertexShaderCode)
	w.gl.CompileShader(vertShader)
	if !w.gl.GetShaderParameterCompileStatus(vertShader) {
		return nil, fmt.Errorf("Vertex shader: %s", w.gl.GetShaderInfoLog(vertShader))
	}

	fragShader := w.gl.CreateFragmentShader()
	w.gl.ShaderSource(fragShader, fragmentShaderCode)
	w.gl.CompileShader(fragShader)
	if !w.gl.GetShaderParameterCompileStatus(fragShader) {
		return nil, fmt.Errorf("Fragment shader: %s", w.gl.GetShaderInfoLog(fragShader))
	}

	shaderProgram := w.gl.CreateProgram()

	w.gl.AttachShader(shaderProgram, vertShader)
	w.gl.AttachShader(shaderProgram, fragShader)
	w.gl.BindAttribLocation(shaderProgram, aPositionIndex, "a_position")
	w.gl.BindAttribLocation(shaderProgram, aTexCoordsIndex, "a_texCoord")

	w.gl.LinkProgram(shaderProgram)
	if !w.gl.GetProgramParameterLinkStatus(shaderProgram) {
		return nil, fmt.Errorf("Program: %s", w.gl.GetProgramInfoLog(shaderProgram))
	}

	w.gl.UseProgram(shaderProgram)

	// this won't actually delete the shaders until the program is closed but it's a good practice
	w.gl.DeleteShader(vertShader)
	w.gl.DeleteShader(fragShader)

	return shaderProgram, nil
}

var texCoordsVertices = []float32{
	// positions // texture coords
	1.0, 1.0, 1.0, 0.0, // top right
	1.0, -1.0, 1.0, 1.0, // bottom right
	-1.0, -1.0, 0.0, 1.0, // bottom left
	-1.0, 1.0, 0.0, 0.0, // bottom left
}

var elementsIndices = []uint16{
	0, 1, 3, // first triangle
	1, 2, 3, // second triangle
}

func (w *windowImpl) createBuffers(shaderProgram *types.Program) *types.VertexArray {
	imageLoc := w.gl.GetUniformLocation(shaderProgram, "u_image")
	// Alias for webgl.TEXTURE0
	w.gl.Uniform1i(imageLoc, 0)

	vertexArray := w.gl.CreateVertexArray()
	vertexBuffer := w.gl.CreateBuffer()
	elementBuffer := w.gl.CreateBuffer()

	w.gl.BindVertexArray(vertexArray)

	w.gl.BindBuffer(webgl.ARRAY_BUFFER, vertexBuffer)
	w.gl.BufferData(webgl.ARRAY_BUFFER, texCoordsVertices, webgl.STATIC_DRAW)

	w.gl.BindBuffer(webgl.ELEMENT_ARRAY_BUFFER, elementBuffer)
	w.gl.BufferDataUI16(webgl.ELEMENT_ARRAY_BUFFER, elementsIndices, webgl.STATIC_DRAW)

	var emptyFloat float32
	floatLen := int(unsafe.Sizeof(emptyFloat))
	w.gl.VertexAttribPointer(aPositionIndex, 2, webgl.FLOAT, false, 4*floatLen, 0)
	w.gl.EnableVertexAttribArray(aPositionIndex)
	w.gl.VertexAttribPointer(aTexCoordsIndex, 2, webgl.FLOAT, false, 4*floatLen, 2*floatLen)
	w.gl.EnableVertexAttribArray(aTexCoordsIndex)

	return vertexArray
}

func (w *windowImpl) createTexture() *types.Texture {
	w.gl.ActiveTexture(uint32(webgl.TEXTURE0))
	imageTex := w.gl.CreateTexture()
	w.gl.BindTexture(webgl.TEXTURE_2D, imageTex)
	w.gl.TexParameterWrapS(webgl.TEXTURE_2D, webgl.REPEAT)
	w.gl.TexParameterWrapT(webgl.TEXTURE_2D, webgl.REPEAT)
	w.gl.TexParameterMinFilter(webgl.TEXTURE_2D, webgl.LINEAR)
	w.gl.TexParameterMagFilter(webgl.TEXTURE_2D, webgl.LINEAR)
	w.gl.TexImage2Db(webgl.TEXTURE_2D, 0, webgl.RGBA, w.width, w.height, 0, webgl.RGBA, nil)

	return imageTex
}

func (w *windowImpl) clear() {
	w.gl.ClearColor(0.0, 0.0, 0.0, 1.0)
	w.gl.Clear(uint32(webgl.COLOR_BUFFER_BIT))
}

func (w *windowImpl) drawBuffer(dp image.Point, src screen.Buffer, sr image.Rectangle) {
	w.gl.BindTexture(webgl.TEXTURE_2D, w.imageTex)
	w.gl.TexSubImage2D(webgl.TEXTURE_2D, 0, dp.X, dp.Y, sr.Max.X, sr.Max.Y, webgl.RGBA, webgl.UNSIGNED_BYTE, webgl.TypedArrayOf(src.RGBA().Pix))

	w.gl.BindVertexArray(w.vertexArray)
	w.gl.DrawElements(webgl.TRIANGLES, len(elementsIndices), webgl.UNSIGNED_SHORT, 0)
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

	w.drawBuffer(dp, src, sr)
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
