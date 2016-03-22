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

const (
	enableJitter    = true
	enableDepthTest = false
	screenWidth     = 640
	screenHeight    = 360
	resolutionX     = 640
	resolutionY     = 360
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
	window, err := sdl.CreateWindow(title, sdl.WINDOWPOS_UNDEFINED, sdl.WINDOWPOS_UNDEFINED, screenWidth, screenHeight, sdl.WINDOW_SHOWN)
	if err != nil {
		panic(err)
	}
	defer window.Destroy()

	renderer, err := sdl.CreateRenderer(window, -1, sdl.RENDERER_ACCELERATED)
	if err != nil {
		panic(err)
	}
	defer renderer.Destroy()

	sdl.SetHint(sdl.HINT_RENDER_SCALE_QUALITY, "linear")
	renderer.SetLogicalSize(resolutionX, resolutionY)
	renderer.SetDrawColor(0, 0, 0, 255)

	rect := image.Rect(0, 0, resolutionX, resolutionY)
	if enableJitter {
		rect.Max.X /= 2
	}

	surfaces := [2]*image.RGBA{image.NewRGBA(rect), image.NewRGBA(rect)}
	backBuffer := image.NewRGBA(image.Rect(0, 0, resolutionX, resolutionY))

	texture, err := renderer.CreateTexture(sdl.PIXELFORMAT_ABGR8888, sdl.TEXTUREACCESS_STREAMING, resolutionX, resolutionY)
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
		ViewDist:     10,
		Tree:         tree,
		Images:       [2]*image.RGBA{surfaces[0], surfaces[1]},
		Jitter:       enableJitter,
		Depth:        enableDepthTest,
	}

	raytracer := trace.NewRaytracer(cfg)
	camera := trace.FreeFlightCamera{XRot: 0, YRot: 0}

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
				case sdl.K_UP:
					camera.YRot += dtf * cameraSpeed
				case sdl.K_DOWN:
					camera.YRot -= dtf * cameraSpeed
				case sdl.K_LEFT:
					camera.XRot += dtf * cameraSpeed
				case sdl.K_RIGHT:
					camera.XRot -= dtf * cameraSpeed
				case sdl.K_w:
					camera.Move(dtf * cameraSpeed)
				case sdl.K_s:
					camera.Move(dtf * -cameraSpeed)
				case sdl.K_a:
					camera.Strafe(dtf * cameraSpeed)
				case sdl.K_d:
					camera.Strafe(dtf * -cameraSpeed)
				case sdl.K_e:
					camera.Lift(dtf * cameraSpeed)
				case sdl.K_q:
					camera.Lift(dtf * -cameraSpeed)
				}
			}
		}

		renderer.Clear()

		if enableDepthTest {
			raytracer.ClearDepth(raytracer.Frame())
		}

		raytracer.Trace(&camera)
		if enableJitter {
			raytracer.Wait(0)
			raytracer.Wait(1)
			if err := trace.Reconstruct(cfg.Images[0], cfg.Images[1], backBuffer); err != nil {
				panic(err)
			}
		} else {
			backBuffer = cfg.Images[0]
			raytracer.Wait(0)
		}

		texture.Update(nil, unsafe.Pointer(&backBuffer.Pix[0]), backBuffer.Stride)
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
