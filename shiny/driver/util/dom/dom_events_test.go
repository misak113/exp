//go:build js
// +build js

package dom

import (
	"reflect"
	"syscall/js"
	"testing"
)

func TestInputPositionRelativeToTarget(t *testing.T) {
	getBoundingClientRect := js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		rect := js.Global().Get("Object").New()
		rect.Set("left", 120)
		rect.Set("top", 80)
		return rect
	})
	defer getBoundingClientRect.Release()

	inputTarget := js.Global().Get("Object").New()
	inputTarget.Set("getBoundingClientRect", getBoundingClientRect)
	event := js.Global().Get("Object").New()
	event.Set("clientX", 145)
	event.Set("clientY", 115)

	domEvents := NewDomEventsForTarget(inputTarget)
	x, y := domEvents.inputPosition(event)
	if x != 25 || y != 35 {
		t.Fatalf("input position is not target-relative: got (%v, %v), want (25, 35)", x, y)
	}
}

func TestInputEventsBindToTarget(t *testing.T) {
	var addedEvents []string
	var removedEvents []string

	addEventListener := js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		addedEvents = append(addedEvents, args[0].String())
		return nil
	})
	defer addEventListener.Release()

	removeEventListener := js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		removedEvents = append(removedEvents, args[0].String())
		return nil
	})
	defer removeEventListener.Release()

	inputTarget := js.Global().Get("Object").New()
	inputTarget.Set("addEventListener", addEventListener)
	inputTarget.Set("removeEventListener", removeEventListener)

	domEvents := NewDomEventsForTarget(inputTarget)
	domEvents.bindMouseEvents()
	domEvents.bindTouchEvents()

	expectedEvents := []string{
		"mousemove",
		"mousedown",
		"mouseup",
		"wheel",
		"touchstart",
		"touchend",
		"touchcancel",
		"touchmove",
	}
	if !reflect.DeepEqual(addedEvents, expectedEvents) {
		t.Fatalf("input events added to wrong target: got %v, want %v", addedEvents, expectedEvents)
	}

	domEvents.Release()
	if !reflect.DeepEqual(removedEvents, expectedEvents) {
		t.Fatalf("input events removed from wrong target: got %v, want %v", removedEvents, expectedEvents)
	}
}
