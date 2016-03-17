// +build js

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
	"fmt"
	"strconv"
	"time"

	"github.com/flimzy/jsblob"
	"github.com/gopherjs/gopherjs/js"
	"github.com/gopherjs/websocket"
)

const (
	imgWidth  = 320
	imgHeight = 200
	imgScale  = 3
)

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

var (
	numFrames = 0
	keys      = make(map[int]bool)
)

func handleError(err error) {
	js.Global.Call("alert", err.Error())
}

func updateScreen(ctx, buf, img *js.Object, dest, src []byte) {
	for i, b := range src {
		dest[i] = b
	}

	img.Get("data").Call("set", buf)
	ctx.Call("putImageData", img, 0, 0)
	numFrames++
}

func setupConnection(ctx, buf, img *js.Object, dest []byte) *websocket.WebSocket {
	ws, err := websocket.New("ws://localhost:8080/render")
	if err != nil {
		handleError(err)
	}

	onOpen := func(ev *js.Object) {
		setup := setupMessage{
			Width:       imgWidth,
			Height:      imgHeight,
			FieldOfView: 45,
			Tree:        "pack/test.oct",
		}

		msg, err := json.Marshal(setup)
		if err != nil {
			handleError(err)
		}

		if err := ws.Send(string(msg)); err != nil {
			handleError(err)
		}
	}

	onMessage := func(ev *js.Object) {
		blob := jsblob.Blob{*ev.Get("data")}
		go func() {
			updateScreen(ctx, buf, img, dest, blob.Bytes())
		}()
	}

	ws.AddEventListener("open", false, onOpen)
	ws.AddEventListener("message", false, onMessage)

	return ws
}

func updateCamera(ws *websocket.WebSocket) {
	const (
		cameraSpeed = 0.01
		tick30hz    = (1000 / 30) * time.Millisecond
	)

	var (
		pressed bool
		msg     updateMessage
	)

	msg.Camera.LookAt = [3]float32{0, 0, -1}
	msg.Camera.Up = [3]float32{0, 1, 0}

	for _ = range time.Tick(tick30hz) {
		pressed = false

		switch {
		case keys[38]: // Up
			msg.Camera.Position[2] -= cameraSpeed
			msg.Camera.LookAt[2] -= cameraSpeed
			pressed = true
		case keys[40]: // Down
			msg.Camera.Position[2] += cameraSpeed
			msg.Camera.LookAt[2] += cameraSpeed
			pressed = true
		case keys[37]: // Left
			msg.Camera.Position[0] += cameraSpeed
			msg.Camera.LookAt[0] += cameraSpeed
			pressed = true
		case keys[39]: // Right
			msg.Camera.Position[0] -= cameraSpeed
			msg.Camera.LookAt[0] -= cameraSpeed
			pressed = true
		}

		if pressed {
			msg, err := json.Marshal(msg)
			if err != nil {
				handleError(err)
			}

			if err := ws.Send(string(msg)); err != nil {
				handleError(err)
			}
		}
	}
}

func updateTitle() {
	title := fmt.Sprintf("AJ's Raytracer - fps: %v", numFrames)
	js.Global.Get("document").Set("title", title)
}

func start() {
	document := js.Global.Get("document")

	document.Set("onkeydown", func(e *js.Object) {
		keys[e.Get("keyCode").Int()] = true
	})

	document.Set("onkeyup", func(e *js.Object) {
		keys[e.Get("keyCode").Int()] = false
	})

	canvas := document.Call("createElement", "canvas")
	canvas.Call("setAttribute", "width", strconv.Itoa(imgWidth))
	canvas.Call("setAttribute", "height", strconv.Itoa(imgHeight))
	canvas.Get("style").Set("width", strconv.Itoa(imgWidth*imgScale)+"px")
	canvas.Get("style").Set("height", strconv.Itoa(imgHeight*imgScale)+"px")
	document.Get("body").Call("appendChild", canvas)

	go func() {
		for _ = range time.Tick(time.Second) {
			updateTitle()
			numFrames = 0
		}
	}()

	ctx := canvas.Call("getContext", "2d")
	img := ctx.Call("getImageData", 0, 0, imgWidth, imgHeight)
	data := img.Get("data")
	arrBuf := js.Global.Get("ArrayBuffer").New(data.Length())
	buf := js.Global.Get("Uint8ClampedArray").New(arrBuf)
	dest := js.Global.Get("Uint8Array").New(arrBuf).Interface().([]byte)

	ws := setupConnection(ctx, buf, img, dest)
	go updateCamera(ws)
}

func main() {
	js.Global.Call("addEventListener", "load", func() { go start() })
}
