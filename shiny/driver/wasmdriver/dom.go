// +build js,wasm

package wasmdriver

import (
	"strings"
	"syscall/js"

	"golang.org/x/mobile/event/key"
	"golang.org/x/mobile/event/mouse"
	"golang.org/x/mobile/event/size"
	"golang.org/x/mobile/geom"
)

const mobileMouseButtonBack mouse.Button = 8
const mobileMouseButtonForward mouse.Button = 9

func getDocWidth() int {
	return js.Global().Get("innerWidth").Int()
}

func getDocHeight() int {
	return js.Global().Get("innerHeight").Int()
}

func getOrientation() (orientation size.Orientation) {
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

func (w *windowImpl) bindSizeEvents() {
	onResize := js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		w.emitSizeEvent()
		return nil
	})
	js.Global().Call("addEventListener", "resize", onResize)
	w.releases = append(w.releases, func() {
		js.Global().Call("removeEventListener", "resize", onResize)
		onResize.Release()
	})
}

func (w *windowImpl) emitSizeEvent() {
	orientation := getOrientation()
	// TODO(nigeltao): don't assume 72 DPI. DisplayWidth and DisplayWidthMM
	// is probably the best place to start looking.
	pixelsPerPt := float32(1)
	width := getDocWidth()
	height := getDocHeight()
	w.eventChan <- size.Event{
		WidthPx:     width,
		HeightPx:    height,
		WidthPt:     geom.Pt(width / int(pixelsPerPt)),
		HeightPt:    geom.Pt(height / int(pixelsPerPt)),
		PixelsPerPt: pixelsPerPt,
		Orientation: orientation,
	}
}

func (w *windowImpl) bindMouseEvents() {
	// move
	onMove := js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		args[0].Call("preventDefault")
		w.eventChan <- mouse.Event{
			X:         float32(args[0].Get("offsetX").Float()),
			Y:         float32(args[0].Get("offsetY").Float()),
			Button:    mouse.ButtonNone,
			Direction: mouse.DirNone,
			Modifiers: getEventModifiers(args[0]),
		}
		return nil
	})
	js.Global().Call("addEventListener", "mousemove", onMove)

	// press/release
	onClick := js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		args[0].Call("preventDefault")
		w.eventChan <- mouse.Event{
			X:         float32(args[0].Get("offsetX").Float()),
			Y:         float32(args[0].Get("offsetY").Float()),
			Button:    getMouseButton(args[0]),
			Direction: getMouseDirection(args[0]),
			Modifiers: getEventModifiers(args[0]),
		}
		return nil
	})
	js.Global().Call("addEventListener", "mousedown", onClick)
	js.Global().Call("addEventListener", "mouseup", onClick)

	// wheel
	onWheel := js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		w.eventChan <- mouse.Event{
			X:         float32(args[0].Get("offsetX").Float()),
			Y:         float32(args[0].Get("offsetY").Float()),
			Button:    getWheelButton(args[0]),
			Direction: mouse.DirStep,
			Modifiers: getEventModifiers(args[0]),
		}
		return nil
	})
	js.Global().Call("addEventListener", "wheel", onWheel)

	w.releases = append(w.releases, func() {
		js.Global().Call("removeEventListener", "mousemove", onMove)
		js.Global().Call("removeEventListener", "mousedown", onClick)
		js.Global().Call("removeEventListener", "mouseup", onClick)
		js.Global().Call("removeEventListener", "wheel", onWheel)
		onMove.Release()
		onClick.Release()
		onWheel.Release()
	})
}

func getMouseDirection(ev js.Value) (dir mouse.Direction) {
	dir = mouse.DirNone
	if ev.Get("type").String() == "mousedown" {
		dir = mouse.DirPress
	}
	if ev.Get("type").String() == "mouseup" {
		dir = mouse.DirRelease
	}
	return
}

func getWheelButton(ev js.Value) mouse.Button {
	deltaY := ev.Get("deltaY").Int()
	deltaX := ev.Get("deltaX").Int()

	if deltaY > 0 {
		return mouse.ButtonWheelDown
	}
	if deltaY < 0 {
		return mouse.ButtonWheelUp
	}
	if deltaX > 0 {
		return mouse.ButtonWheelRight
	}
	if deltaX < 0 {
		return mouse.ButtonWheelLeft
	}
	return mouse.ButtonNone
}

const (
	domMouseLeft    = 0
	domMouseMiddle  = 1
	domMouseRight   = 2
	domMouseBack    = 3
	domMouseForward = 4
)

func getMouseButton(ev js.Value) mouse.Button {
	switch ev.Get("button").Int() {
	case domMouseLeft:
		return mouse.ButtonLeft
	case domMouseMiddle:
		return mouse.ButtonMiddle
	case domMouseRight:
		return mouse.ButtonRight
	case domMouseBack:
		return mobileMouseButtonBack
	case domMouseForward:
		return mobileMouseButtonForward
	default:
		return mouse.ButtonNone
	}
}

func getEventModifiers(ev js.Value) (mod key.Modifiers) {
	if ev.Get("altKey").Bool() {
		mod |= key.ModAlt
	}
	if ev.Get("ctrlKey").Bool() {
		mod |= key.ModControl
	}
	if ev.Get("shiftKey").Bool() {
		mod |= key.ModShift
	}
	if ev.Get("metaKey").Bool() {
		mod |= key.ModMeta
	}

	return
}

func (w *windowImpl) bindKeyEvents() {
	// press/release
	onKey := js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		args[0].Call("preventDefault")
		w.eventChan <- key.Event{
			Rune:      getKeyRune(args[0]),
			Code:      getKeyCode(args[0]),
			Direction: getKeyDirection(args[0]),
			Modifiers: getEventModifiers(args[0]),
		}
		return nil
	})
	js.Global().Call("addEventListener", "keydown", onKey)
	js.Global().Call("addEventListener", "keyup", onKey)

	w.releases = append(w.releases, func() {
		js.Global().Call("removeEventListener", "keydown", onKey)
		js.Global().Call("removeEventListener", "keyup", onKey)
		onKey.Release()
	})
}

func getKeyRune(ev js.Value) rune {
	key := []rune(ev.Get("key").String())
	if len(key) == 1 {
		return key[0]
	}
	return -1
}

func getKeyDirection(ev js.Value) (dir key.Direction) {
	dir = key.DirNone
	if ev.Get("type").String() == "keydown" {
		dir = key.DirPress
	}
	if ev.Get("type").String() == "keyup" {
		dir = key.DirRelease
	}
	return
}

func getKeyCode(ev js.Value) key.Code {
	domCode := ev.Get("code").String()
	if code, exists := keyCodesMap[domCode]; exists {
		return code
	}
	domKey := ev.Get("key").String()
	if code, exists := keyCodesByKeyMap[domKey]; exists {
		return code
	}
	return key.CodeUnknown
}

// inspired https://www.freecodecamp.org/news/javascript-keycode-list-keypress-event-key-codes/
var keyCodesMap = map[string]key.Code{
	"KeyA": key.CodeA,
	"KeyB": key.CodeB,
	"KeyC": key.CodeC,
	"KeyD": key.CodeD,
	"KeyE": key.CodeE,
	"KeyF": key.CodeF,
	"KeyG": key.CodeG,
	"KeyH": key.CodeH,
	"KeyI": key.CodeI,
	"KeyJ": key.CodeJ,
	"KeyK": key.CodeK,
	"KeyL": key.CodeL,
	"KeyM": key.CodeM,
	"KeyN": key.CodeN,
	"KeyO": key.CodeO,
	"KeyP": key.CodeP,
	"KeyQ": key.CodeQ,
	"KeyR": key.CodeR,
	"KeyS": key.CodeS,
	"KeyT": key.CodeT,
	"KeyU": key.CodeU,
	"KeyV": key.CodeV,
	"KeyW": key.CodeW,
	"KeyX": key.CodeX,
	"KeyY": key.CodeY,
	"KeyZ": key.CodeZ,

	"Digit1": key.Code1,
	"Digit2": key.Code2,
	"Digit3": key.Code3,
	"Digit4": key.Code4,
	"Digit5": key.Code5,
	"Digit6": key.Code6,
	"Digit7": key.Code7,
	"Digit8": key.Code8,
	"Digit9": key.Code9,
	"Digit0": key.Code0,

	"Enter":        key.CodeReturnEnter,
	"Escape":       key.CodeEscape,
	"Backspace":    key.CodeDeleteBackspace,
	"Tab":          key.CodeTab,
	"Space":        key.CodeSpacebar,
	"Minus":        key.CodeHyphenMinus,
	"Equal":        key.CodeEqualSign,
	"BracketLeft":  key.CodeLeftSquareBracket,
	"BracketRight": key.CodeRightSquareBracket,
	"Backslash":    key.CodeBackslash,
	"Semicolon":    key.CodeSemicolon,
	"Quote":        key.CodeApostrophe,
	"Backquote":    key.CodeGraveAccent,
	"Comma":        key.CodeComma,
	"Period":       key.CodeFullStop,
	"Slash":        key.CodeSlash,
	"CapsLock":     key.CodeCapsLock,

	"F1":  key.CodeF1,
	"F2":  key.CodeF2,
	"F3":  key.CodeF3,
	"F4":  key.CodeF4,
	"F5":  key.CodeF5,
	"F6":  key.CodeF6,
	"F7":  key.CodeF7,
	"F8":  key.CodeF8,
	"F9":  key.CodeF9,
	"F10": key.CodeF10,
	"F11": key.CodeF11,
	"F12": key.CodeF12,

	"Pause":    key.CodePause,
	"Insert":   key.CodeInsert,
	"Home":     key.CodeHome,
	"PageUp":   key.CodePageUp,
	"Delete":   key.CodeDeleteForward,
	"End":      key.CodeEnd,
	"PageDown": key.CodePageDown,

	"ArrowRight": key.CodeRightArrow,
	"ArrowLeft":  key.CodeLeftArrow,
	"ArrowDown":  key.CodeDownArrow,
	"ArrowUp":    key.CodeUpArrow,

	"NumLock":        key.CodeKeypadNumLock,
	"NumpadDivide":   key.CodeKeypadSlash,
	"NumpadMultiply": key.CodeKeypadAsterisk,
	"NumpadSubtract": key.CodeKeypadHyphenMinus,
	"NumpadAdd":      key.CodeKeypadPlusSign,
	"NumpadEnter":    key.CodeKeypadEnter,
	"Numpad1":        key.CodeKeypad1,
	"Numpad2":        key.CodeKeypad2,
	"Numpad3":        key.CodeKeypad3,
	"Numpad4":        key.CodeKeypad4,
	"Numpad5":        key.CodeKeypad5,
	"Numpad6":        key.CodeKeypad6,
	"Numpad7":        key.CodeKeypad7,
	"Numpad8":        key.CodeKeypad8,
	"Numpad9":        key.CodeKeypad9,
	"Numpad0":        key.CodeKeypad0,
	"NumpadDecimal":  key.CodeKeypadFullStop,
	"NumpadEqual":    key.CodeKeypadEqualSign, // TODO no keyboard has this key

	"F13": key.CodeF13,
	"F14": key.CodeF14,
	"F15": key.CodeF15,
	"F16": key.CodeF16,
	"F17": key.CodeF17,
	"F18": key.CodeF18,
	"F19": key.CodeF19,
	"F20": key.CodeF20,
	"F21": key.CodeF21,
	"F22": key.CodeF22,
	"F23": key.CodeF23,
	"F24": key.CodeF24,

	"Help":        key.CodeHelp, // TODO no keyboard has this key
	"ContextMenu": key.CodeProps,

	"VolumeMute": key.CodeMute,       // FF only
	"VolumeUp":   key.CodeVolumeUp,   // FF only
	"VolumeDown": key.CodeVolumeDown, // FF only

	"ControlLeft":  key.CodeLeftControl,
	"ShiftLeft":    key.CodeLeftShift,
	"AltLeft":      key.CodeLeftAlt,
	"MetaLeft":     key.CodeLeftGUI,
	"ControlRight": key.CodeRightControl,
	"ShiftRight":   key.CodeRightShift,
	"AltRight":     key.CodeRightAlt,
	"MetaRight":    key.CodeRightGUI,

	"PrintScreen": key.CodeUnknown, // TODO mobile key doesn't have this key.Code
}

// Following keys cannot be detected based on ev.code
var keyCodesByKeyMap = map[string]key.Code{
	"AudioVolumeMute": key.CodeMute,       // non FF
	"AudioVolumeUp":   key.CodeVolumeUp,   // non FF
	"AudioVolumeDown": key.CodeVolumeDown, // non FF
}
