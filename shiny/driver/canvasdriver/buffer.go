// +build js

package canvasdriver

import (
	"image"
	"sync"
	"syscall/js"
)

// YCbCrJS is a replacement of YCbCr image with memory pointers to CanvasImageSource within JS (not Go)
// it's useful for rendering data already available in JS and reduce data bytes copying between JS & GoLang
type YCbCrJS struct {
	CanvasImageSource *js.Value
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

// YCbCrJS is for optimized rendering of Image data already available in JS (not in Go WASM)
// Follow example bellow in JS,WASM only environment
/*

// +build js

ycbcrJSImg := outputBuffer.(*canvasdriver.BufferImpl).YCbCrJS()
ycbcrJSImg.CanvasImageSource = frame
*/
func (b *BufferImpl) YCbCrJS() *YCbCrJS {
	if b.ycbcrJS == nil {
		b.ycbcrJS = &YCbCrJS{
			CanvasImageSource: nil,
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
