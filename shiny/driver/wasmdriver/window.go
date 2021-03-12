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

	w.programRGBA, err = w.createAndLinkProgramRGBA()
	if err != nil {
		panic(err)
	}
	w.programYUV420, err = w.createAndLinkProgramYUV420()
	if err != nil {
		panic(err)
	}
	w.vertexArray = w.createBuffers()
	w.imageTexRGBA = w.createTexture(textureUnitRGBA, webgl.RGBA, w.width, w.height)
	w.imageTexY = w.createTexture(textureUnitY, webgl.LUMINANCE, w.width, w.height)
	w.imageTexU = w.createTexture(textureUnitU, webgl.LUMINANCE, w.width/2, w.height/2)
	w.imageTexV = w.createTexture(textureUnitV, webgl.LUMINANCE, w.width/2, w.height/2)

	w.gl.Enable(webgl.DEPTH_TEST)
	w.gl.Viewport(0, 0, w.width, w.height)
	w.clear()

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

const fragmentShaderCodeRGBA = `
precision mediump float;
varying vec2 v_texCoord;
uniform sampler2D u_image;

void main(void) {
	gl_FragColor = texture2D(u_image, v_texCoord);
}
`

const fragmentShaderCodeYUV420 = `
precision mediump float;
varying vec2 v_texCoord;
uniform sampler2D u_imageY;
uniform sampler2D u_imageU;
uniform sampler2D u_imageV;

void main(void) {
	float yChannel = texture2D(u_imageY, v_texCoord).x;
	float uChannel = texture2D(u_imageU, v_texCoord).x;
	float vChannel = texture2D(u_imageV, v_texCoord).x;

	// This does the colorspace conversion from Y'UV to RGB as a matrix
	// multiply.  It also does the offset of the U and V channels from
	// [0,1] to [-.5,.5] as part of the transform.
	vec4 channels = vec4(yChannel, uChannel, vChannel, 1.0);

	mat4 conversion = mat4(1.0,  0.0,    1.402, -0.701,
							1.0, -0.344, -0.714,  0.529,
							1.0,  1.772,  0.0,   -0.886,
							0, 0, 0, 0);
	vec3 rgb = (channels * conversion).xyz;

	// This is another Y'UV transform that can be used, but it doesn't
	// accurately transform my source image.  Your images may well fare
	// better with it, however, considering they come from a different
	// source, and because I'm not sure that my original was converted
	// to Y'UV420p with the same RGB->YUV (or YCrCb) conversion as
	// yours.
	//
	// vec4 channels = vec4(yChannel, uChannel, vChannel, 1.0);
	// float3x4 conversion = float3x4(1.0,  0.0,      1.13983, -0.569915,
	//                                1.0, -0.39465, -0.58060,  0.487625,
	//                                1.0,  2.03211,  0.0,     -1.016055);
	// float3 rgb = mul(conversion, channels);
	gl_FragColor = vec4(rgb, 1.0);
}
`

const (
	textureUnitRGBA = webgl.TEXTURE0
	textureUnitY    = webgl.TEXTURE1
	textureUnitU    = webgl.TEXTURE2
	textureUnitV    = webgl.TEXTURE3
)

const (
	aPositionIndex  = 0
	aTexCoordsIndex = 1
)

func (w *windowImpl) createAndLinkProgramRGBA() (*types.Program, error) {
	vertShader := w.gl.CreateVertexShader()
	w.gl.ShaderSource(vertShader, vertexShaderCode)
	w.gl.CompileShader(vertShader)
	if !w.gl.GetShaderParameterCompileStatus(vertShader) {
		return nil, fmt.Errorf("Vertex shader: %s", w.gl.GetShaderInfoLog(vertShader))
	}

	fragShader := w.gl.CreateFragmentShader()
	w.gl.ShaderSource(fragShader, fragmentShaderCodeRGBA)
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

	// this won't actually delete the shaders until the program is closed but it's a good practice
	w.gl.DeleteShader(vertShader)
	w.gl.DeleteShader(fragShader)

	w.gl.UseProgram(shaderProgram)

	imageLoc := w.gl.GetUniformLocation(shaderProgram, "u_image")
	// Alias for webgl.TEXTURE0 = textureUnitRGBA
	w.gl.Uniform1i(imageLoc, 0)

	return shaderProgram, nil
}

func (w *windowImpl) createAndLinkProgramYUV420() (*types.Program, error) {
	vertShader := w.gl.CreateVertexShader()
	w.gl.ShaderSource(vertShader, vertexShaderCode)
	w.gl.CompileShader(vertShader)
	if !w.gl.GetShaderParameterCompileStatus(vertShader) {
		return nil, fmt.Errorf("Vertex shader: %s", w.gl.GetShaderInfoLog(vertShader))
	}

	fragShader := w.gl.CreateFragmentShader()
	w.gl.ShaderSource(fragShader, fragmentShaderCodeYUV420)
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

	// this won't actually delete the shaders until the program is closed but it's a good practice
	w.gl.DeleteShader(vertShader)
	w.gl.DeleteShader(fragShader)

	w.gl.UseProgram(shaderProgram)

	imageYLoc := w.gl.GetUniformLocation(shaderProgram, "u_imageY")
	// Alias for webgl.TEXTURE1 = textureUnitY
	w.gl.Uniform1i(imageYLoc, 1)

	imageULoc := w.gl.GetUniformLocation(shaderProgram, "u_imageU")
	// Alias for webgl.TEXTURE2 = textureUnitU
	w.gl.Uniform1i(imageULoc, 2)

	imageVLoc := w.gl.GetUniformLocation(shaderProgram, "u_imageV")
	// Alias for webgl.TEXTURE3 = textureUnitV
	w.gl.Uniform1i(imageVLoc, 3)

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

func (w *windowImpl) createBuffers() *types.VertexArray {
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

func (w *windowImpl) createTexture(
	unit types.GLEnum,
	format types.GLEnum,
	width int,
	height int,
) *types.Texture {
	w.gl.ActiveTexture(uint32(unit))
	imageTex := w.gl.CreateTexture()
	w.gl.BindTexture(webgl.TEXTURE_2D, imageTex)
	w.gl.TexParameterWrapS(webgl.TEXTURE_2D, webgl.REPEAT)
	w.gl.TexParameterWrapT(webgl.TEXTURE_2D, webgl.REPEAT)
	w.gl.TexParameterMinFilter(webgl.TEXTURE_2D, webgl.LINEAR)
	w.gl.TexParameterMagFilter(webgl.TEXTURE_2D, webgl.LINEAR)
	w.gl.TexImage2Db(webgl.TEXTURE_2D, 0, format, width, height, 0, format, nil)

	return imageTex
}

func (w *windowImpl) clear() {
	w.gl.ClearColor(0.0, 0.0, 0.0, 1.0)
	w.gl.Clear(uint32(webgl.COLOR_BUFFER_BIT))
}

func (w *windowImpl) drawBufferRGBA(dp image.Point, src screen.Buffer, sr image.Rectangle) {
	w.gl.UseProgram(w.programRGBA)
	w.gl.BindTexture(webgl.TEXTURE_2D, w.imageTexRGBA)
	w.gl.TexSubImage2D(webgl.TEXTURE_2D, 0, dp.X, dp.Y, sr.Max.X, sr.Max.Y, webgl.RGBA, webgl.UNSIGNED_BYTE, webgl.TypedArrayOf(src.RGBA().Pix))

	w.gl.BindVertexArray(w.vertexArray)
	w.gl.DrawElements(webgl.TRIANGLES, len(elementsIndices), webgl.UNSIGNED_SHORT, 0)
}

func (w *windowImpl) drawBufferYUV420(dp image.Point, src screen.Buffer, sr image.Rectangle) {
	w.gl.UseProgram(w.programYUV420)
	w.gl.BindTexture(webgl.TEXTURE_2D, w.imageTexY)
	w.gl.TexSubImage2D(webgl.TEXTURE_2D, 0, dp.X, dp.Y, sr.Max.X, sr.Max.Y, webgl.LUMINANCE, webgl.UNSIGNED_BYTE, webgl.TypedArrayOf(src.YCbCr().Y))

	w.gl.BindTexture(webgl.TEXTURE_2D, w.imageTexU)
	w.gl.TexSubImage2D(webgl.TEXTURE_2D, 0, dp.X, dp.Y, sr.Max.X/2, sr.Max.Y/2, webgl.LUMINANCE, webgl.UNSIGNED_BYTE, webgl.TypedArrayOf(src.YCbCr().Cb))

	w.gl.BindTexture(webgl.TEXTURE_2D, w.imageTexV)
	w.gl.TexSubImage2D(webgl.TEXTURE_2D, 0, dp.X, dp.Y, sr.Max.X/2, sr.Max.Y/2, webgl.LUMINANCE, webgl.UNSIGNED_BYTE, webgl.TypedArrayOf(src.YCbCr().Cr))

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
