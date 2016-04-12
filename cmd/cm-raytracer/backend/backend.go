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

var cameraMatrix = [6]lookAtCamera{
	// pos, at, up
	{trace.Vec3{0, 0, 0}, trace.Vec3{-1, 0, 0}, trace.Vec3{0, 1, 0}}, // Right
	{trace.Vec3{0, 0, 0}, trace.Vec3{1, 0, 0}, trace.Vec3{0, 1, 0}},  // Left

	{trace.Vec3{0, 0, 0}, trace.Vec3{0, 1, 0}, trace.Vec3{0, 0, -1}}, // Up
	{trace.Vec3{0, 0, 0}, trace.Vec3{0, -1, 0}, trace.Vec3{0, 0, 1}}, // Down

	{trace.Vec3{0, 0, 0}, trace.Vec3{0, 0, 1}, trace.Vec3{0, 1, 0}},  // Backward
	{trace.Vec3{0, 0, 0}, trace.Vec3{0, 0, -1}, trace.Vec3{0, 1, 0}}, // Forward
}

var loadedTree struct {
	maxDepth int
	tree     trace.Octree
}

type (
	setupMessage struct {
		Resolution int     `resolution`
		ClearColor [4]byte `clear_color`
	}

	updateMessage struct {
		Position [3]float32 `position`
	}

	lookAtCamera struct {
		pos, at, up trace.Vec3
	}
)

func (c *lookAtCamera) Up() trace.Vec3 {
	return c.up
}

func (c *lookAtCamera) Position() trace.Vec3 {
	return c.pos
}

func (c *lookAtCamera) LookAt() trace.Vec3 {
	return c.at
}

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

	if setup.Resolution < 16 || setup.Resolution > 4096 {
		log.Println("invalid setup")
		return
	}

	var (
		surfaces   [6]*image.RGBA
		raytracers [6]*trace.Raytracer
		cameras    [6]lookAtCamera

		rect = image.Rect(0, 0, setup.Resolution, setup.Resolution)
	)

	for i, _ := range surfaces {
		surfaces[i] = image.NewRGBA(rect)

		cfg := trace.Config{
			FieldOfView:   1.55,
			TreeScale:     1,
			ViewDist:      float32(arguments.viewDistance),
			Images:        [2]*image.RGBA{surfaces[i], nil},
			Jitter:        false,
			MultiThreaded: true,
		}

		raytracers[i] = trace.NewRaytracer(cfg)
		clear := setup.ClearColor
		raytracers[i].SetClearColor(color.RGBA{clear[0], clear[1], clear[2], clear[3]})

		cameras[i] = cameraMatrix[i]
	}

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

	for {
		update := <-updateChan
		for i := 0; i < 6; i++ {
			camera := cameras[i]
			camera.pos = update.Position
			camera.at[0] += camera.pos[0]
			camera.at[1] += camera.pos[1]
			camera.at[2] += camera.pos[2]

			raytracers[i].Trace(&camera, loadedTree.tree, loadedTree.maxDepth)
		}

		for _, rt := range raytracers {
			if err := streamCodec.Send(ws, rt.Image(0).Pix); err != nil {
				log.Println(err)
				return
			}
		}
	}
}

var arguments struct {
	web,
	tree string
	pprof bool
	port,
	timeout uint
	viewDistance float64
}

func init() {
	flag.Usage = func() {
		fmt.Printf("Usage: program [options]\n\n")
		flag.PrintDefaults()
	}

	flag.StringVar(&arguments.web, "web", "cmd/cm-raytracer/frontend", "web frontend location")
	flag.StringVar(&arguments.tree, "tree", "tree.oct", "octree to serve clients")
	flag.BoolVar(&arguments.pprof, "pprof", false, "enables cpu profiler and pprof over http, port 6060")
	flag.UintVar(&arguments.port, "port", 8080, "server port")
	flag.UintVar(&arguments.timeout, "timeout", 30, "max session length in minutes")
	flag.Float64Var(&arguments.viewDistance, "dist", 1, "max view-distance")
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
