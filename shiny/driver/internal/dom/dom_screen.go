// +build js

package dom

import (
	"strings"
	"syscall/js"

	"golang.org/x/mobile/event/size"
)

func GetDocWidth() int {
	return js.Global().Get("innerWidth").Int()
}

func GetDocHeight() int {
	return js.Global().Get("innerHeight").Int()
}

func GetOrientation() (orientation size.Orientation) {
	defer func() {
		if recover() != nil {
			orientation = size.OrientationUnknown
		}
	}()
	orientationType := js.Global().Get("screen").Get("orientation").Get("type").String()
	if strings.HasPrefix(orientationType, "landscape") {
		orientation = size.OrientationLandscape
		return
	}
	if strings.HasPrefix(orientationType, "portrait") {
		orientation = size.OrientationLandscape
		return
	}
	orientation = size.OrientationUnknown
	return
}
