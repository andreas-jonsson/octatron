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
	"flag"
	"fmt"
	"image"
	"net/http"
	_ "net/http/pprof"
	"os"
	"runtime"
	"time"
	"unsafe"

	"github.com/andreas-jonsson/octatron/trace"
	"github.com/veandco/go-sdl2/sdl"
)

const (
	enableDepthTest = false
	cameraSpeed     = 0.001
	mouseSpeed      = 0.00001
)

var arguments struct {
	ppm,
	pprof,
	multiThreaded,
	enableJitter bool

	fieldOfView int
	viewDistance,
	treeScale float64

	windowSize,
	resolution,
	treePosition,
	scaleFilter,
	inputFile string
}

var (
	enableInput = true

	screenWidth,
	screenHeight,
	resolutionX,
	resolutionY int
)

func toggleFullscreen(window *sdl.Window) {
	isFullscreen := (window.GetFlags() & sdl.WINDOW_FULLSCREEN) != 0
	if isFullscreen {
		window.SetFullscreen(0)
	} else {
		window.SetFullscreen(sdl.WINDOW_FULLSCREEN_DESKTOP)
	}
}

func moveCamera(camera *trace.FreeFlightCamera, dtf float32) {
	state := sdl.GetKeyboardState()
	switch {
	case state[sdl.GetScancodeFromKey(sdl.K_UP)] != 0:
		camera.YRot += dtf * cameraSpeed
	case state[sdl.GetScancodeFromKey(sdl.K_DOWN)] != 0:
		camera.YRot -= dtf * cameraSpeed
	case state[sdl.GetScancodeFromKey(sdl.K_LEFT)] != 0:
		camera.XRot += dtf * cameraSpeed
	case state[sdl.GetScancodeFromKey(sdl.K_RIGHT)] != 0:
		camera.XRot -= dtf * cameraSpeed
	case state[sdl.GetScancodeFromKey(sdl.K_w)] != 0:
		camera.Move(dtf * cameraSpeed)
	case state[sdl.GetScancodeFromKey(sdl.K_s)] != 0:
		camera.Move(dtf * -cameraSpeed)
	case state[sdl.GetScancodeFromKey(sdl.K_a)] != 0:
		camera.Strafe(dtf * cameraSpeed)
	case state[sdl.GetScancodeFromKey(sdl.K_d)] != 0:
		camera.Strafe(dtf * -cameraSpeed)
	case state[sdl.GetScancodeFromKey(sdl.K_e)] != 0:
		camera.Lift(dtf * cameraSpeed)
	case state[sdl.GetScancodeFromKey(sdl.K_q)] != 0:
		camera.Lift(dtf * -cameraSpeed)
	}
}

func writePPM(img image.Image) {
	size := img.Bounds().Max
	fmt.Printf("P6 %d %d 255\n", size.X, size.Y)

	for y := 0; y < size.Y; y++ {
		for x := 0; x < size.X; x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			rgba := [3]byte{byte(r), byte(g), byte(b)}
			os.Stdout.Write(rgba[:])
		}
	}

	os.Stdout.Sync()
}

func init() {
	runtime.LockOSThread()

	flag.Usage = func() {
		fmt.Printf("Usage: raytracer [options]\n\n")
		flag.PrintDefaults()
	}

	flag.StringVar(&arguments.inputFile, "tree", "tree.oct", "path to .oct file.")
	flag.StringVar(&arguments.treePosition, "pos", "0 0 0", "octree position in world")
	flag.StringVar(&arguments.scaleFilter, "filter", "linear", "used to scale image")
	flag.StringVar(&arguments.windowSize, "window", "640 360", "window size")
	flag.StringVar(&arguments.resolution, "resolution", "640 360", "back-buffer size")
	flag.IntVar(&arguments.fieldOfView, "fov", 45, "camera field-of-view")
	flag.Float64Var(&arguments.viewDistance, "dist", 1, "max view-distance")
	flag.Float64Var(&arguments.treeScale, "scale", 1, "octree scale")
	flag.BoolVar(&arguments.enableJitter, "jitter", true, "enables frame jitter")
	flag.BoolVar(&arguments.multiThreaded, "mt", true, "enables multi-threading")
	flag.BoolVar(&arguments.pprof, "pprof", false, "enables pprof over http, port 6060")
	flag.BoolVar(&arguments.ppm, "ppm", false, "write ppm-stream to stdout")
}

func main() {
	flag.Parse()

	if arguments.pprof {
		go func() {
			fmt.Fprintln(os.Stderr, http.ListenAndServe("localhost:6060", nil))
		}()
	}

	fmt.Sscan(arguments.windowSize, &screenWidth, &screenHeight)
	fmt.Sscan(arguments.resolution, &resolutionX, &resolutionY)

	fp, err := os.Open(arguments.inputFile)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
	}
	defer fp.Close()

	tree, vpa, err := trace.LoadOctree(fp)
	if err != nil {
		panic(err)
	}
	maxDepth := trace.TreeWidthToDepth(vpa)

	sdl.Init(sdl.INIT_EVERYTHING)
	defer sdl.Quit()

	title := "AJ's Raytracer"
	window, err := sdl.CreateWindow(title, sdl.WINDOWPOS_UNDEFINED, sdl.WINDOWPOS_UNDEFINED, screenWidth, screenHeight, sdl.WINDOW_SHOWN)
	if err != nil {
		panic(err)
	}
	defer window.Destroy()
	sdl.SetRelativeMouseMode(true)

	renderer, err := sdl.CreateRenderer(window, -1, sdl.RENDERER_ACCELERATED)
	if err != nil {
		panic(err)
	}
	defer renderer.Destroy()

	sdl.SetHint(sdl.HINT_RENDER_SCALE_QUALITY, arguments.scaleFilter)
	renderer.SetLogicalSize(resolutionX, resolutionY)
	renderer.SetDrawColor(0, 0, 0, 255)

	rect := image.Rect(0, 0, resolutionX, resolutionY)
	if arguments.enableJitter {
		rect.Max.X /= 2
	}

	surfaces := [2]*image.RGBA{image.NewRGBA(rect), image.NewRGBA(rect)}
	backBuffer := image.NewRGBA(image.Rect(0, 0, resolutionX, resolutionY))

	texture, err := renderer.CreateTexture(sdl.PIXELFORMAT_ABGR8888, sdl.TEXTUREACCESS_STREAMING, resolutionX, resolutionY)
	if err != nil {
		panic(err)
	}
	defer texture.Destroy()

	var pos [3]float32
	fmt.Sscan(arguments.treePosition, &pos[0], &pos[1], &pos[2])

	cfg := trace.Config{
		FieldOfView:   float32(arguments.fieldOfView),
		TreeScale:     float32(arguments.treeScale),
		TreePosition:  pos,
		ViewDist:      float32(arguments.viewDistance),
		Images:        [2]*image.RGBA{surfaces[0], surfaces[1]},
		Jitter:        arguments.enableJitter,
		MultiThreaded: arguments.multiThreaded,
		Depth:         enableDepthTest,
	}

	raytracer := trace.NewRaytracer(cfg)
	camera := trace.FreeFlightCamera{XRot: 0, YRot: 0}

	nf := 0
	dt := time.Duration(1000 / 60)
	ft := time.Duration(nf)

	for {
		t := time.Now()
		dtf := float32(dt / time.Millisecond)

		for event := sdl.PollEvent(); event != nil; event = sdl.PollEvent() {
			switch t := event.(type) {
			case *sdl.QuitEvent:
				return
			case *sdl.MouseMotionEvent:
				if enableInput {
					camera.XRot -= dtf * float32(t.XRel) * mouseSpeed
					camera.YRot -= dtf * float32(t.YRel) * mouseSpeed
				}
			case *sdl.KeyUpEvent:
				switch t.Keysym.Sym {
				case sdl.K_ESCAPE:
					return
				case sdl.K_f:
					toggleFullscreen(window)
				case sdl.K_SPACE:
					enableInput = !enableInput
					sdl.SetRelativeMouseMode(enableInput)
				}
			}
		}

		renderer.Clear()

		if enableInput || arguments.ppm || arguments.pprof {
			if enableInput {
				moveCamera(&camera, dtf)
				window.WarpMouseInWindow(screenWidth/2, screenHeight/2)
			}

			if enableDepthTest {
				raytracer.ClearDepth(raytracer.Frame())
			}

			raytracer.Trace(&camera, tree, maxDepth)
		}

		raytracer.Wait(0)
		if arguments.enableJitter {
			raytracer.Wait(1)
			if err := trace.Reconstruct(cfg.Images[0], cfg.Images[1], backBuffer); err != nil {
				panic(err)
			}
		} else {
			backBuffer = cfg.Images[0]
		}

		texture.Update(nil, unsafe.Pointer(&backBuffer.Pix[0]), backBuffer.Stride)
		renderer.Copy(texture, nil, nil)
		renderer.Present()

		if arguments.ppm {
			writePPM(backBuffer)
		}

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
