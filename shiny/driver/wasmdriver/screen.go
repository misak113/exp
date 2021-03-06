// +build js,wasm

package wasmdriver

import (
	"fmt"
	"image"
	"syscall/js"

	"golang.org/x/exp/shiny/screen"
)

type screenImpl struct {
	doc js.Value
}

func newScreenImpl() (*screenImpl, error) {
	s := &screenImpl{
		doc: js.Global().Get("document"),
	}
	return s, nil
}

func (s *screenImpl) NewBuffer(size image.Point) (screen.Buffer, error) {
	buffer := newBuffer(s, size)
	return buffer, nil
}

func (s *screenImpl) NewTexture(size image.Point) (screen.Texture, error) {
	return nil, fmt.Errorf("Texture not implemented")
}

func (s *screenImpl) NewWindow(opts *screen.NewWindowOptions) (screen.Window, error) {
	window := newWindow(s, opts)
	return window, nil
}
