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
}

func newBuffer(screen *screenImpl, size image.Point) *bufferImpl {
	b := &bufferImpl{
		screen: screen,
		size:   size,
		mutex:  &sync.Mutex{},
		rgba: image.RGBA{
			Stride: 4 * size.X,
			Rect:   image.Rectangle{Max: size},
			Pix:    make([]uint8, 4*size.X*size.Y),
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

func (b *bufferImpl) Release() {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	if b.released {
		return
	}

	b.released = true
}
