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
	"errors"
	"fmt"
	"image"
	"image/color/palette"
	"strconv"
	"time"

	"github.com/andreas-jonsson/octatron/trace"
	"github.com/gopherjs/gopherjs/js"
	"github.com/gopherjs/websocket"
)

const (
	imgWidth  = 320
	imgHeight = 180
	imgScale  = 2
	//colorFormat = "RGBA"
	colorFormat = "PALETTE"

	//hostAddress = "localhost"
	hostAddress = "server.andreasjonsson.se"
)

type (
	setupMessage struct {
		Width       int     `width`
		Height      int     `height`
		FieldOfView float32 `field_of_view`
		ViewDist    float32 `view_dist`
		ColorFormat string  `color_format`
	}

	updateMessage struct {
		Camera struct {
			Position [3]float32 `position`
			XRot     float32    `x_rot`
			YRot     float32    `y_rot`
		} "camera"
	}
)

var (
	keys       = make(map[int]bool)
	imgRect    = image.Rect(0, 0, imgWidth/2, imgHeight)
	palImages  = [2]*image.Paletted{image.NewPaletted(imgRect, palette.Plan9), image.NewPaletted(imgRect, palette.Plan9)}
	rgbaImages = [2]*image.RGBA{image.NewRGBA(imgRect), image.NewRGBA(imgRect)}
	finalImage = image.NewRGBA(image.Rect(0, 0, imgWidth, imgHeight))

	frameId, numFrames int
)

func throw(err error) {
	js.Global.Call("alert", err.Error())
	panic(err)
}

func assert(err error) {
	if err != nil {
		throw(err)
	}
}

func setupConnection(canvas *js.Object) {
	ctx := canvas.Call("getContext", "2d")
	img := ctx.Call("getImageData", 0, 0, imgWidth, imgHeight)

	if img.Get("data").Length() != len(finalImage.Pix) {
		throw(errors.New("data size of images do not match"))
	}

	ws, err := websocket.New(fmt.Sprintf("ws://%s:8080/render", hostAddress))
	assert(err)

	onOpen := func(ev *js.Object) {
		setup := setupMessage{
			Width:       imgWidth,
			Height:      imgHeight,
			FieldOfView: 45,
			ViewDist:    20,
			ColorFormat: colorFormat,
		}

		msg, err := json.Marshal(setup)
		assert(err)

		assert(ws.Send(string(msg)))

		go updateCamera(ws)
	}

	onMessage := func(ev *js.Object) {
		idx := frameId % 2
		data := js.Global.Get("Uint8Array").New(ev.Get("data")).Interface().([]uint8)

		var (
			imageA, imageB image.Image
		)

		if colorFormat == "RGBA" {
			rgbaImages[idx].Pix = data
			imageA = rgbaImages[0]
			imageB = rgbaImages[1]
		} else {
			palImages[idx].Pix = data
			imageA = palImages[0]
			imageB = palImages[1]
		}

		// This function could be optimized for this specific senario.
		assert(trace.Reconstruct(imageA, imageB, finalImage))

		arrBuf := js.NewArrayBuffer(finalImage.Pix)
		buf := js.Global.Get("Uint8ClampedArray").New(arrBuf)
		img.Get("data").Call("set", buf)
		ctx.Call("putImageData", img, 0, 0)

		numFrames++
		frameId++
	}

	ws.BinaryType = "arraybuffer"
	ws.AddEventListener("open", false, onOpen)
	ws.AddEventListener("message", false, onMessage)
}

func updateCamera(ws *websocket.WebSocket) {
	const (
		cameraSpeed = 0.1
		tick30hz    = (1000 / 30) * time.Millisecond
	)

	var (
		//pressed bool
		msg updateMessage
	)

	msg.Camera.Position = [3]float32{0.666, 0, 1.131}
	msg.Camera.XRot = 0.46938998
	msg.Camera.YRot = 0.26761

	for _ = range time.Tick(tick30hz) {
		//pressed = true

		switch {
		case keys[38]: // Up
			msg.Camera.Position[2] -= cameraSpeed
		case keys[40]: // Down
			msg.Camera.Position[2] += cameraSpeed
		case keys[37]: // Left
			msg.Camera.Position[0] += cameraSpeed
		case keys[39]: // Right
			msg.Camera.Position[0] -= cameraSpeed
		default:
			//pressed = false
		}

		//if pressed {
		msg, err := json.Marshal(msg)
		assert(err)

		assert(ws.Send(string(msg)))
		//}
	}
}

func updateTitle() {
	title := fmt.Sprintf("AJ's Raytracer - fps: %v", numFrames)
	js.Global.Get("document").Set("title", title)
}

func load() {
	document := js.Global.Get("document")

	go func() {
		for _ = range time.Tick(time.Second) {
			updateTitle()
			numFrames = 0
		}
	}()

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

	setupConnection(canvas)
}

func main() {
	js.Global.Call("addEventListener", "load", func() { go load() })
}
