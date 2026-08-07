//go:build js
// +build js

package canvasdriver

import (
	"sync"
	"syscall/js"
	"testing"

	"golang.org/x/exp/shiny/driver/util/dom"
)

func newEventTestWindow() *windowImpl {
	return &windowImpl{
		mutex:     &sync.Mutex{},
		domEvents: dom.NewDomEventsForTarget(js.Undefined()),
		eventWake: make(chan struct{}, 1),
	}
}

func TestSendNonBlockingWakesNextEvent(t *testing.T) {
	window := newEventTestWindow()
	event := struct{}{}

	if !window.SendNonBlocking(event) {
		t.Fatal("SendNonBlocking unexpectedly rejected event")
	}
	if received := window.NextEvent(); received != event {
		t.Fatalf("NextEvent returned %T, want injected event", received)
	}
}

func TestSendNonBlockingDoesNotBlockWhenQueueIsFull(t *testing.T) {
	window := newEventTestWindow()

	for window.SendNonBlocking(struct{}{}) {
	}

	if window.SendNonBlocking(struct{}{}) {
		t.Fatal("SendNonBlocking accepted event into full queue")
	}
}

func TestSendFirstPrioritizesEventsInLIFOOrder(t *testing.T) {
	window := newEventTestWindow()

	window.Send("normal first")
	window.Send("normal second")
	window.SendFirst("priority first")
	window.SendFirst("priority second")

	want := []string{"priority second", "priority first", "normal first", "normal second"}
	for _, expected := range want {
		if received := window.NextEvent(); received != expected {
			t.Fatalf("NextEvent returned %q, want %q", received, expected)
		}
	}
}

func TestSendFirstPreservesNormalEventBackpressure(t *testing.T) {
	window := newEventTestWindow()
	eventChan := window.domEvents.GetEventChan()
	for i := 0; i < cap(eventChan); i++ {
		eventChan <- i
	}

	window.SendFirst("priority")

	if got, want := len(eventChan), cap(eventChan); got != want {
		t.Fatalf("normal event queue length is %d, want %d", got, want)
	}
	if event := window.NextEvent(); event != "priority" {
		t.Fatalf("NextEvent returned %q, want priority", event)
	}
}

func TestSendFirstWakesNextEvent(t *testing.T) {
	window := newEventTestWindow()
	received := make(chan interface{}, 1)

	go func() {
		received <- window.NextEvent()
	}()
	window.SendFirst("priority")

	if event := <-received; event != "priority" {
		t.Fatalf("NextEvent returned %q, want priority", event)
	}
}

func TestSendAfterReleaseIsIgnored(t *testing.T) {
	window := newEventTestWindow()
	window.released = true

	window.Send(struct{}{})
	window.SendFirst(struct{}{})
	if _, ok := window.nextQueuedEvent(); ok {
		t.Fatal("SendFirst accepted event after release")
	}
	if window.SendNonBlocking(struct{}{}) {
		t.Fatal("SendNonBlocking accepted event after release")
	}
}
