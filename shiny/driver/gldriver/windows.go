// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build windows

package gldriver

import (
	"golang.org/x/exp/shiny/driver/internal/errscreen"
	"golang.org/x/exp/shiny/driver/internal/win32"
	"golang.org/x/exp/shiny/screen"
)

func newWindow(width, height int32) uintptr {
	return 0
}
func showWindow(w *windowImpl) {}
func closeWindow(id uintptr)   {}
func drawLoop(w *windowImpl)   {}

func main(f func(screen.Screen)) error {
	if err := win32.Main(func() { f(theScreen) }); err != nil {
		f(errscreen.Stub(err))
	}
	return nil
}
