/*
Copyright (C) 2016 Andreas T Jonsson

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

package main

import (
	"image"
	"os"
	"runtime"
	"unsafe"

	"github.com/veandco/go-sdl2/sdl"
)

func toggleFullscreen(window *sdl.Window) {
	isFullscreen := (window.GetFlags() & sdl.WINDOW_FULLSCREEN) != 0
	if isFullscreen {
		window.SetFullscreen(0)
		sdl.ShowCursor(1)
	} else {
		window.SetFullscreen(sdl.WINDOW_FULLSCREEN_DESKTOP)
		sdl.ShowCursor(0)
	}
}

func init() {
	runtime.LockOSThread()
}

func main() {
	sdl.Init(sdl.INIT_EVERYTHING)
	defer sdl.Quit()

	window, err := sdl.CreateWindow("raytracer", sdl.WINDOWPOS_UNDEFINED, sdl.WINDOWPOS_UNDEFINED, 640, 320, sdl.WINDOW_SHOWN)
	if err != nil {
		panic(err)
	}
	defer window.Destroy()

	renderer, err := sdl.CreateRenderer(window, -1, sdl.RENDERER_ACCELERATED)
	if err != nil {
		panic(err)
	}
	defer renderer.Destroy()

	width, height := 640, 360
	sdl.SetHint(sdl.HINT_RENDER_SCALE_QUALITY, "linear")
	renderer.SetLogicalSize(width, height)
	renderer.SetDrawColor(0, 0, 0, 255)

	surface := image.NewRGBA(image.Rect(0, 0, width, height))
	texture, err := renderer.CreateTexture(sdl.PIXELFORMAT_ABGR8888, sdl.TEXTUREACCESS_STREAMING, width, height)
	if err != nil {
		panic(err)
	}
	defer texture.Destroy()

	fp, err := os.Open("pack/test.oct")
	if err != nil {
		panic(err)
	}
	defer fp.Close()

	tree, err := loadOctree(fp)
	if err != nil {
		panic(err)
	}

	for {
		for event := sdl.PollEvent(); event != nil; event = sdl.PollEvent() {
			switch t := event.(type) {
			case *sdl.QuitEvent:
				return
			case *sdl.KeyUpEvent:
				switch t.Keysym.Sym {
				case sdl.K_ESCAPE:
					return
				case sdl.K_f:
					toggleFullscreen(window)
				}
			}
		}

		renderer.Clear()

		startTrace(tree, surface)

		texture.Update(nil, unsafe.Pointer(&surface.Pix[0]), surface.Stride)
		renderer.Copy(texture, nil, nil)
		renderer.Present()
	}
}
