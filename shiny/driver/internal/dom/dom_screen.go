// +build js

package dom

import (
	"math"
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

func GetScreenWidth() int {
	return js.Global().Get("screen").Get("width").Int()
}

func GetScreenHeight() int {
	return js.Global().Get("screen").Get("height").Int()
}

func GetDevicePixelRatio() float64 {
	return js.Global().Get("devicePixelRatio").Float()
}

func GetBrowserZoomRatio() float64 {
	// TODO currently works only for chromium based browsers
	windowInnerWidth := js.Global().Get("innerWidth").Float()
	windowOuterWidth := js.Global().Get("outerWidth").Float()
	docElClientWidth := js.Global().Get("document").Get("documentElement").Get("clientWidth").Float()
	scrollerWidthInNoZoom := windowInnerWidth - docElClientWidth
	chromeZoom1 := (windowOuterWidth - scrollerWidthInNoZoom) / windowInnerWidth
	scrollerWidthInZoom1 := math.Round(scrollerWidthInNoZoom * chromeZoom1)
	chromeZoom2 := (windowOuterWidth - scrollerWidthInZoom1) / windowInnerWidth
	scrollerWidthInZoom2 := math.Round(scrollerWidthInNoZoom * chromeZoom2)
	chromeZoom3 := (windowOuterWidth - scrollerWidthInZoom2) / windowInnerWidth
	return chromeZoom3
}

func SetWindowWidth(newInnerWidth int) {
	innerWidth := js.Global().Get("innerWidth").Int()
	outerWidth := js.Global().Get("outerWidth").Int()
	outerHeight := js.Global().Get("outerHeight").Int()
	newOuterWidth := newInnerWidth + (outerWidth - innerWidth)
	screenWidth := GetScreenWidth()
	if newOuterWidth > screenWidth {
		newOuterWidth = screenWidth
	}
	js.Global().Call("resizeTo", newOuterWidth, outerHeight)
}

func SetWindowHeight(newInnerHeight int) {
	innerHeight := js.Global().Get("innerHeight").Int()
	outerHeight := js.Global().Get("outerHeight").Int()
	outerWidth := js.Global().Get("outerWidth").Int()
	newOuterHeight := newInnerHeight + (outerHeight - innerHeight)
	screenHeight := GetScreenHeight()
	if newOuterHeight > screenHeight {
		newOuterHeight = screenHeight
	}
	js.Global().Call("resizeTo", outerWidth, newOuterHeight)
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
