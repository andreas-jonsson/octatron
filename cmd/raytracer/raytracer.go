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
	"fmt"
	"image"
	"os"
	"runtime"
	"time"
	"unsafe"

	"github.com/andreas-jonsson/octatron/trace"
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

	title := "AJ's Raytracer"
	window, err := sdl.CreateWindow(title, sdl.WINDOWPOS_UNDEFINED, sdl.WINDOWPOS_UNDEFINED, 640, 320, sdl.WINDOW_SHOWN)
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

	tree, err := trace.LoadOctree(fp)
	if err != nil {
		panic(err)
	}

	cfg := trace.Config{
		FieldOfView:  45,
		TreeScale:    1,
		TreePosition: [3]float32{-0.5, -0.5, -3},
		Tree:         tree,
		Image:        surface,
	}

	camera := trace.Camera{LookAt: [3]float32{0, 0, -1}, Up: [3]float32{0, 1, 0}}

	nf := 0
	dt := time.Duration(1000 / 60)
	ft := time.Duration(nf)

	for {
		t := time.Now()
		dtf := float32(dt / time.Millisecond)

		const cameraSpeed = 0.001

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
				/*
					case sdl.K_UP:
						camera.Position[2] += dtf * cameraSpeed
					case sdl.K_DOWN:
						camera.Position[2] -= dtf * cameraSpeed
					case sdl.K_LEFT:
						camera.Position[0] += dtf * cameraSpeed
					case sdl.K_RIGHT:
						camera.Position[0] -= dtf * cameraSpeed
					case sdl.K_w:
						camera.LookAt[2] += dtf * cameraSpeed
					case sdl.K_s:
						camera.LookAt[2] -= dtf * cameraSpeed
					case sdl.K_a:
						camera.LookAt[0] += dtf * cameraSpeed
					case sdl.K_d:
						camera.LookAt[0] -= dtf * cameraSpeed
				*/
				case sdl.K_UP:
					camera.Position[2] -= dtf * cameraSpeed
					camera.LookAt[2] -= dtf * cameraSpeed
				case sdl.K_DOWN:
					camera.Position[2] += dtf * cameraSpeed
					camera.LookAt[2] += dtf * cameraSpeed
				case sdl.K_LEFT:
					camera.Position[0] += dtf * cameraSpeed
					camera.LookAt[0] += dtf * cameraSpeed
				case sdl.K_RIGHT:
					camera.Position[0] -= dtf * cameraSpeed
					camera.LookAt[0] -= dtf * cameraSpeed
				}
			}
		}

		renderer.Clear()

		trace.Raytrace(&cfg, &camera)

		texture.Update(nil, unsafe.Pointer(&surface.Pix[0]), surface.Stride)
		renderer.Copy(texture, nil, nil)
		renderer.Present()

		dt = time.Since(t)
		ft += dt
		nf++

		if ft >= time.Second {
			window.SetTitle(fmt.Sprintf("%v - fps: %v, dt: %vms", title, nf, int(ft/time.Millisecond)/nf))
			nf = 0
			ft = 0
		}
	}
}
