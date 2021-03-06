// +build js,wasm

package wasmdriver

import (
	"golang.org/x/exp/shiny/driver/internal/errscreen"
	"golang.org/x/exp/shiny/screen"
)

func Main(f func(screen.Screen)) {
	if err := main(f); err != nil {
		f(errscreen.Stub(err))
	}
}

func main(f func(screen.Screen)) (retErr error) {
	s, err := newScreenImpl()
	if err != nil {
		return err
	}
	f(s)
	return nil
}
