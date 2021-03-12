// +build js,wasm

package wasmdriver

import (
	"image"
	"sync"
)

type bufferImpl struct {
	screen *screenImpl
	size   image.Point
	// internal
	mutex *sync.Mutex
	// state
	released bool
	rgba     image.RGBA
	ycbcr    image.YCbCr
}

func newBuffer(screen *screenImpl, size image.Point) *bufferImpl {
	rect := image.Rectangle{Max: size}
	b := &bufferImpl{
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

func (b *bufferImpl) Size() image.Point {
	return b.size
}

func (b *bufferImpl) Bounds() image.Rectangle {
	return image.Rectangle{Max: b.size}
}

func (b *bufferImpl) RGBA() *image.RGBA {
	return &b.rgba
}

func (b *bufferImpl) YCbCr() *image.YCbCr {
	return &b.ycbcr
}

func (b *bufferImpl) Release() {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	if b.released {
		return
	}

	b.released = true
}
