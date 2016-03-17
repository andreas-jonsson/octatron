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
	"image"
	"log"
	"net/http"
	"os"
	"sync"

	"golang.org/x/net/websocket"

	"github.com/andreas-jonsson/octatron/trace"
)

var closeFrameErr = errors.New("close-frame")

var (
	messageCodec = websocket.Codec{Marshal: nil, Unmarshal: unmarshalMessage}
	streamCodec  = websocket.Codec{Marshal: marshalData, Unmarshal: nil}
)

type entry struct {
	count int
	tree  trace.Octree
}

var trees struct {
	sync.Mutex
	data map[string]entry
}

type (
	setupMessage struct {
		Width       int     "width"
		Height      int     "height"
		FieldOfView float32 "field_of_view"
		Tree        string  "tree"
	}

	updateMessage struct {
		Camera struct {
			Position [3]float32 "position"
			LookAt   [3]float32 "look_at"
			Up       [3]float32 "up"
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

func loadTree(file string) (trace.Octree, error) {
	trees.Lock()
	defer trees.Unlock()

	e, ok := trees.data[file]
	if ok {
		e.count++
		return e.tree, nil
	}

	fp, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer fp.Close()

	e.tree, err = trace.LoadOctree(fp)
	if err != nil {
		return nil, err
	}

	log.Println("loading:", file)

	e.count = 1
	trees.data[file] = e
	return e.tree, nil
}

func unloadTree(file string) {
	trees.Lock()
	defer trees.Unlock()

	e, ok := trees.data[file]
	if !ok {
		panic("file not loaded: " + file)
	}

	e.count--
	if e.count == 0 {
		delete(trees.data, file)
		log.Println("unloading:", file)
	}
}

func renderServer(ws *websocket.Conn) {
	log.Println("new connection:", ws.RemoteAddr())

	var setup setupMessage
	if err := messageCodec.Receive(ws, &setup); err != nil {
		log.Println(err)
		return
	}

	// TODO Verify message.

	tree, err := loadTree(setup.Tree)
	if err != nil {
		log.Println(err, setup.Tree)
		return
	}
	defer unloadTree(setup.Tree)

	log.Println("reading from:", setup.Tree)

	cfg := trace.Config{
		FieldOfView:  setup.FieldOfView,
		TreeScale:    1,
		TreePosition: [3]float32{-0.5, -0.5, -3},
		Tree:         tree,
		Image:        image.NewRGBA(image.Rect(0, 0, setup.Width, setup.Height)),
	}

	updateChan := make(chan updateMessage, 1)
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
		camera := trace.Camera{
			Position: update.Camera.Position,
			LookAt:   update.Camera.LookAt,
			Up:       update.Camera.Up,
		}

		trace.Raytrace(&cfg, &camera)

		if err := streamCodec.Send(ws, cfg.Image.Pix); err != nil {
			log.Println(err)
			return
		}
	}
}

func main() {
	trees.data = make(map[string]entry)

	http.Handle("/render", websocket.Handler(renderServer))
	log.Println("waiting for connections...")

	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Println(err)
		os.Exit(-1)
	}
}
