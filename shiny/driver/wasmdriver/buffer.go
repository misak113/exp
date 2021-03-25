// +build js,wasm

package wasmdriver

import (
	"image"
	"sync"
	"syscall/js"
)

// ArrayBufferSlice is pointer to ArrayBuffer and the offset where the data starts from beginning of ArrayBuffer & length of data to be used.
// To get the right data of buffer, you can use for example arrayBuffer.Slice.Call("subarray", arrayBufferSlice.Offset, arrayBufferSlice.Length)
type ArrayBufferSlice struct {
	ArrayBuffer js.Value
	Offset      int
	Length      int
}

// YCbCrJS is a replacement of YCbCr image with memory pointers to data within JS (not Go)
// it's useful for rendering data already available in JS and reduce data bytes copying between JS & GoLang
type YCbCrJS struct {
	Y              ArrayBufferSlice
	Cb             ArrayBufferSlice
	Cr             ArrayBufferSlice
	YStride        int
	CStride        int
	SubsampleRatio image.YCbCrSubsampleRatio
	Rect           image.Rectangle
	// Use if you'd like to render from this object YCbCrJS() rather than standard YCbCr(), set Use to true. Otherwise YCbCr() is used.
	Use bool
}

func (a *ArrayBufferSlice) slice() js.Value {
	return a.ArrayBuffer.Call("subarray", a.Offset, a.Length)
}

type BufferImpl struct {
	screen *screenImpl
	size   image.Point
	// internal
	mutex *sync.Mutex
	// state
	released bool
	rgba     image.RGBA
	ycbcr    image.YCbCr
	ycbcrJS  *YCbCrJS
}

func newBuffer(screen *screenImpl, size image.Point) *BufferImpl {
	rect := image.Rectangle{Max: size}
	b := &BufferImpl{
		screen: screen,
		size:   size,
		mutex:  &sync.Mutex{},
		rgba: image.RGBA{
			Stride: 4 * size.X,
			Rect:   rect,
			Pix:    make([]uint8, 4*size.X*size.Y),
		},
		ycbcr: image.YCbCr{
			Rect:           rect,
			SubsampleRatio: image.YCbCrSubsampleRatio420,
			YStride:        size.X,
			CStride:        size.X / 2,
			Y:              make([]uint8, size.X*size.Y),
			Cb:             make([]uint8, size.X*size.Y/4),
			Cr:             make([]uint8, size.X*size.Y/4),
		},
	}
	return b
}

func (b *BufferImpl) Size() image.Point {
	return b.size
}

func (b *BufferImpl) Bounds() image.Rectangle {
	return image.Rectangle{Max: b.size}
}

func (b *BufferImpl) RGBA() *image.RGBA {
	return &b.rgba
}

func (b *BufferImpl) YCbCr() *image.YCbCr {
	return &b.ycbcr
}

// YCbCrJS is for optimized rendering of YUV data already available in JS (not in Go WASM)
// Follow example bellow in JS,WASM only environment
/*

// +build js,wasm

ycbcrJSImg := outputBuffer.(*wasmdriver.BufferImpl).YCbCrJS()
ycbcrJSImg.Y = wasmdriver.ArrayBufferSlice{pictureBuffer, 0, ySize}
ycbcrJSImg.Cb = wasmdriver.ArrayBufferSlice{pictureBuffer, ySize, ySize + cSize}
ycbcrJSImg.Cr = wasmdriver.ArrayBufferSlice{pictureBuffer, ySize + cSize, ySize + cSize*2}
ycbcrJSImg.YStride = yStride
ycbcrJSImg.CStride = cStride
ycbcrJSImg.SubsampleRatio = image.YCbCrSubsampleRatio420
ycbcrJSImg.Rect = image.Rect(0, 0, width, height)
ycbcrJSImg.Use = true
*/
func (b *BufferImpl) YCbCrJS() *YCbCrJS {
	if b.ycbcrJS == nil {
		emptyBuffer := js.Global().Get("ArrayBuffer").New()
		b.ycbcrJS = &YCbCrJS{
			Y:              ArrayBufferSlice{emptyBuffer, 0, 0},
			Cb:             ArrayBufferSlice{emptyBuffer, 0, 0},
			Cr:             ArrayBufferSlice{emptyBuffer, 0, 0},
			Rect:           b.ycbcr.Rect,
			SubsampleRatio: image.YCbCrSubsampleRatio420,
			YStride:        b.ycbcr.YStride,
			CStride:        b.ycbcr.CStride,
			Use:            false,
		}
	}
	return b.ycbcrJS
}

func (b *BufferImpl) Release() {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	if b.released {
		return
	}

	b.released = true
}
