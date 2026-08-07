//go:build js
// +build js

package webgldriver

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

func TestSendAfterReleaseIsIgnored(t *testing.T) {
	window := newEventTestWindow()
	window.released = true

	window.Send(struct{}{})
	if window.SendNonBlocking(struct{}{}) {
		t.Fatal("SendNonBlocking accepted event after release")
	}
}
