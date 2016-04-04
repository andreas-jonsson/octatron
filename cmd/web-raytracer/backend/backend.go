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
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/color/palette"
	"image/draw"
	_ "image/png"
	"log"
	"net/http"
	"os"
	"runtime/pprof"
	"time"

	"golang.org/x/net/websocket"

	"github.com/andreas-jonsson/octatron/trace"
)

var closeFrameErr = errors.New("close-frame")

var (
	messageCodec = websocket.Codec{Marshal: nil, Unmarshal: unmarshalMessage}
	streamCodec  = websocket.Codec{Marshal: marshalData, Unmarshal: nil}
)

var loadedTree struct {
	maxDepth int
	tree     trace.Octree
	pal      color.Palette
	rawPal   []byte
}

type (
	setupMessage struct {
		Width       int     `width`
		Height      int     `height`
		FieldOfView float32 `field_of_view`
		ViewDist    float32 `view_dist`
		ColorFormat string  `color_format`
		ClearColor  [4]byte `clear_color`
	}

	updateMessage struct {
		Camera struct {
			Position [3]float32 `position`
			XRot     float32    `x_rot`
			YRot     float32    `y_rot`
		} "camera"
	}
)

func marshalData(v interface{}) ([]byte, byte, error) {
	return v.([]byte), websocket.BinaryFrame, nil
}

func unmarshalMessage(data []byte, ty byte, v interface{}) error {
	switch ty {
	case websocket.CloseFrame:
		return closeFrameErr
	case websocket.TextFrame:
		if err := json.Unmarshal(data, v); err != nil {
			return err
		}
		return nil
	default:
		return errors.New("invalid frame type")
	}
}

func loadTree(file string) error {
	pal := palette.Plan9
	rawPal := make([]byte, 4*256)

	treeFp, err := os.Open(file)
	if err != nil {
		return err
	}
	defer treeFp.Close()

	log.Println("loading octree:", file)
	tree, vpa, err := trace.LoadOctree(treeFp)
	if err != nil {
		return err
	}

	loadedTree.maxDepth = trace.TreeWidthToDepth(vpa)
	loadedTree.tree = tree

	paletteFile := file + ".png"
	paletteFp, err := os.Open(paletteFile)
	if err == nil {
		src, _, err := image.Decode(paletteFp)

		if err == nil {
			size := src.Bounds().Max

			if size.X == 16 && size.Y == 16 {
				log.Println("loading palette:", paletteFile)
				pal = make([]color.Color, 256)

				for y := 0; y < 16; y++ {
					for x := 0; x < 16; x++ {
						r, g, b, _ := src.At(x, y).RGBA()
						pal[y*16+x] = color.RGBA{uint8(r), uint8(g), uint8(b), 0xFF}
					}
				}
			}
		}
	}
	paletteFp.Close()

	for i, c := range pal {
		r, g, b, _ := c.RGBA()
		rawPal[i*4] = byte(r)
		rawPal[i*4+1] = byte(g)
		rawPal[i*4+2] = byte(b)
		rawPal[i*4+3] = 0xFF
	}

	loadedTree.pal = pal
	loadedTree.rawPal = rawPal
	return nil
}

func renderServer(ws *websocket.Conn) {
	addr := ws.RemoteAddr()
	log.Println("new connection:", addr)
	defer func() { log.Println(addr, "was disconnected") }()

	// Setup watchdog.
	shutdownWatch := make(chan struct{}, 1)
	defer func() { shutdownWatch <- struct{}{} }()
	go func() {
		select {
		case <-shutdownWatch:
		case <-time.After(time.Duration(arguments.timeout) * time.Minute):
			log.Println("session timeout")
			ws.Close()
		}
	}()

	var setup setupMessage
	if err := messageCodec.Receive(ws, &setup); err != nil {
		log.Println(err)
		return
	}
	log.Println(setup)

	if setup.Width*setup.Height > 1280*720 || setup.FieldOfView < 45 || setup.FieldOfView > 180 {
		log.Println("invalid setup")
		log.Println(setup)
		return
	}

	rect := image.Rect(0, 0, setup.Width/2, setup.Height)
	backBuffer := image.NewPaletted(rect, loadedTree.pal)
	surfaces := [2]*image.RGBA{
		image.NewRGBA(rect),
		image.NewRGBA(rect),
	}

	cfg := trace.Config{
		FieldOfView:   setup.FieldOfView,
		TreeScale:     1,
		ViewDist:      setup.ViewDist,
		Images:        surfaces,
		Jitter:        true,
		MultiThreaded: true,
		FrameSeed:     1,
	}

	raytracer := trace.NewRaytracer(cfg)
	clear := setup.ClearColor
	raytracer.SetClearColor(color.RGBA{clear[0], clear[1], clear[2], clear[3]})
	updateChan := make(chan updateMessage, 2)

	go func() {
		var update updateMessage
		for {
			if err := messageCodec.Receive(ws, &update); err != nil {
				log.Println(err)
				return
			}

			// TODO Verify message.
			updateChan <- update
		}
	}()

	// Send palette.
	if setup.ColorFormat == "PALETTED" {
		log.Println("sending palette...")
		if err := streamCodec.Send(ws, loadedTree.rawPal); err != nil {
			log.Println(err)
			return
		}
	}

	for {
		update := <-updateChan
		camera := trace.FreeFlightCamera{
			Pos:  update.Camera.Position,
			XRot: update.Camera.XRot,
			YRot: update.Camera.YRot,
		}

		frame := 1 + raytracer.Trace(&camera, loadedTree.tree, loadedTree.maxDepth)
		idx := frame % 2

		var err error
		if setup.ColorFormat == "PALETTED" {
			draw.Draw(backBuffer, rect, raytracer.Image(idx), image.ZP, draw.Src)
			err = streamCodec.Send(ws, backBuffer.Pix)
		} else {
			err = streamCodec.Send(ws, raytracer.Image(idx).Pix)
		}

		if err != nil {
			log.Println(err)
			return
		}
	}
}

var arguments struct {
	web,
	tree string
	pprof bool
	port,
	timeout uint
}

func init() {
	flag.Usage = func() {
		fmt.Printf("Usage: program [options]\n\n")
		flag.PrintDefaults()
	}

	flag.StringVar(&arguments.web, "web", "cmd/web-raytracer/frontend", "web frontend location")
	flag.StringVar(&arguments.tree, "tree", "tree.oct", "octree to serve clients")
	flag.BoolVar(&arguments.pprof, "pprof", false, "enables cpu profiler and pprof over http, port 6060")
	flag.UintVar(&arguments.port, "port", 8080, "server port")
	flag.UintVar(&arguments.timeout, "timeout", 3, "max session length in minutes")
}

func main() {
	flag.Parse()

	if arguments.pprof {
		log.Println("pprof enabled")
		go func() {
			log.Println(http.ListenAndServe("localhost:6060", nil))
		}()

		fp, err := os.Create("backend.pprof")
		if err != nil {
			panic(err)
		}
		defer fp.Close()

		pprof.StartCPUProfile(fp)
		defer pprof.StopCPUProfile()
	}

	if err := loadTree(arguments.tree); err != nil {
		log.Println(err)
		os.Exit(-1)
	}

	http.Handle("/", http.FileServer(http.Dir(arguments.web)))
	http.Handle("/render", websocket.Handler(renderServer))

	log.Println("waiting for connections...")
	if err := http.ListenAndServe(fmt.Sprintf(":%v", arguments.port), nil); err != nil {
		log.Println(err)
		os.Exit(-1)
	}
}
