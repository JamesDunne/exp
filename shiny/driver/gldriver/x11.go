// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build linux,!android

package gldriver

/*
#cgo LDFLAGS: -lEGL -lGLESv2 -lX11

#include <stdint.h>

void startDriver();
void processEvents();
void makeCurrent(uintptr_t ctx);
void swapBuffers(uintptr_t ctx);
uintptr_t doNewWindow(int width, int height);
uintptr_t doShowWindow(uintptr_t id);
*/
import "C"
import (
	"runtime"
	"time"

	"golang.org/x/exp/shiny/screen"
	"golang.org/x/mobile/event/paint"
	"golang.org/x/mobile/event/size"
	"golang.org/x/mobile/geom"
	"golang.org/x/mobile/gl"
)

func init() {
	// It might not be necessary, but it probably doesn't hurt to try to make
	// 'the main thread' be 'the X11 / OpenGL thread'.
	runtime.LockOSThread()
}

func newWindow(width, height int32) uintptr {
	retc := make(chan uintptr)
	uic <- uiClosure{
		f: func() uintptr {
			return uintptr(C.doNewWindow(C.int(width), C.int(height)))
		},
		retc: retc,
	}
	return <-retc
}

func showWindow(id uintptr) uintptr {
	retc := make(chan uintptr)
	uic <- uiClosure{
		f: func() uintptr {
			return uintptr(C.doShowWindow(C.uintptr_t(id)))
		},
		retc: retc,
	}
	return <-retc
}

func closeWindow(id uintptr) {
	// TODO.
}

func drawLoop(w *windowImpl) {
	glcontextc <- w.ctx
	go func() {
		for range w.publish {
			publishc <- w
		}
	}()
}

var (
	glcontextc = make(chan uintptr)
	publishc   = make(chan *windowImpl)
	uic        = make(chan uiClosure)
)

// uiClosure is a closure to be run on C's UI thread.
type uiClosure struct {
	f    func() uintptr
	retc chan uintptr
}

func main(f func(screen.Screen)) error {
	C.startDriver()

	closec := make(chan struct{})
	go func() {
		f(theScreen)
		close(closec)
	}()

	// heartbeat is a channel that, at regular intervals, directs the select
	// below to also consider X11 events, not just Go events (channel
	// communications).
	//
	// TODO: select instead of poll. Note that knowing whether to call
	// C.processEvents needs to select on a file descriptor, and the other
	// cases below select on Go channels.
	heartbeat := time.NewTicker(time.Second / 60)

	// glWorkAvailable is set to gl.WorkAvailable after we have a GL context.
	// TODO: should we be able to make a shiny.Texture before having a
	// shiny.Window's GL context? Should something like gl.IsProgram be a
	// method instead of a function, and have each shiny.Window have its own
	// gl.Context?
	var glWorkAvailable <-chan struct{}

	for {
		select {
		case <-closec:
			return nil
		case ctx := <-glcontextc:
			glWorkAvailable = gl.WorkAvailable
			// TODO: don't assume that there is only one window, and hence only
			// one (global) GL context.
			//
			// TODO: do we need to synchronize with seeing a size event for
			// this window's context before or after calling makeCurrent?
			// Otherwise, are we racing with the gl.Viewport call? I've
			// occasionally seen a stale viewport, if the window manager sets
			// the window width and height to something other than that
			// requested by XCreateWindow, but it's not easily reproducible.
			C.makeCurrent(C.uintptr_t(ctx))
		case w := <-publishc:
			C.swapBuffers(C.uintptr_t(w.ctx))
		case req := <-uic:
			req.retc <- req.f()
		case <-heartbeat.C:
			C.processEvents()
		case <-glWorkAvailable:
			gl.DoWork()
		}
	}
}

//export onResize
func onResize(id uintptr, width, height int32) {
	theScreen.mu.Lock()
	w := theScreen.windows[id]
	theScreen.mu.Unlock()

	if w == nil {
		return
	}

	// TODO: should this really be done on the receiving end of the w.Events()
	// channel, in the same goroutine as other GL calls in the app's 'business
	// logic'?
	go gl.Viewport(0, 0, int(width), int(height))

	sz := size.Event{
		WidthPx:  int(width),
		HeightPx: int(height),
		WidthPt:  geom.Pt(width),
		HeightPt: geom.Pt(height),
		// TODO: don't assume 72 DPI. DisplayWidth and DisplayWidthMM is
		// probably the best place to start looking.
		PixelsPerPt: 1,
	}

	w.mu.Lock()
	w.sz = sz
	w.mu.Unlock()

	w.Send(sz)

	// TODO: convert X11 expose events to shiny paint events, instead of this
	// one-off hack.
	w.Send(paint.Event{})
}