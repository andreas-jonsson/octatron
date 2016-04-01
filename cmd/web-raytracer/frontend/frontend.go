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
	"image/color/palette"
	"time"

	"github.com/gopherjs/gopherjs/js"
	"github.com/gopherjs/webgl"
	"github.com/gopherjs/websocket"
)

const (
	imgWidth  = 512
	imgHeight = 256
	imgScale  = 1

	hostAddress = "localhost"
	//hostAddress = "server.andreasjonsson.se"
)

type (
	setupMessage struct {
		Width       int     "width"
		Height      int     "height"
		FieldOfView float32 "field_of_view"
		ViewDist    float32 "view_dist"
	}

	updateMessage struct {
		Camera struct {
			Position [3]float32 "position"
			XRot     float32    "x_rot"
			YRot     float32    "y_rot"
		} "camera"
	}
)

var (
	keys = make(map[int]bool)

	sizeLocation       *js.Object
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

func glAssert(gl *webgl.Context) {
	if err := gl.GetError(); err != gl.NO_ERROR {
		throw(fmt.Errorf("GL Error: %v", err))
	}
}

func setupConnection(gl *webgl.Context) {
	ws, err := websocket.New(fmt.Sprintf("ws://%s:8080/render", hostAddress))
	assert(err)

	onOpen := func(ev *js.Object) {
		setup := setupMessage{
			Width:       imgWidth,
			Height:      imgHeight,
			FieldOfView: 45,
			ViewDist:    20,
		}

		msg, err := json.Marshal(setup)
		assert(err)

		assert(ws.Send(string(msg)))

		go updateCamera(ws)
	}

	onMessage := func(ev *js.Object) {
		data := js.Global.Get("Uint8Array").New(ev.Get("data"))

		gl.Call("texImage2D", gl.TEXTURE_2D, 0, gl.LUMINANCE, imgWidth/2, imgHeight, 0, gl.LUMINANCE, gl.UNSIGNED_BYTE, data)
		gl.Uniform3i(sizeLocation, imgWidth/2, imgHeight, frameId%2)
		gl.DrawArrays(gl.TRIANGLES, 0, 6)

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

func setupShaders(gl *webgl.Context) *js.Object {
	vs := gl.CreateShader(gl.VERTEX_SHADER)
	gl.ShaderSource(vs, vsSource)
	gl.CompileShader(vs)

	if gl.GetShaderParameter(vs, gl.COMPILE_STATUS).Bool() == false {
		throw(errors.New(gl.GetShaderInfoLog(vs)))
	}

	ps := gl.CreateShader(gl.FRAGMENT_SHADER)
	gl.ShaderSource(ps, psSource)
	gl.CompileShader(ps)

	if gl.GetShaderParameter(ps, gl.COMPILE_STATUS).Bool() == false {
		throw(errors.New(gl.GetShaderInfoLog(ps)))
	}

	program := gl.CreateProgram()
	gl.AttachShader(program, vs)
	gl.AttachShader(program, ps)

	gl.LinkProgram(program)
	gl.UseProgram(program)
	return program
}

func buildArrays() (*js.Object, *js.Object) {
	jsPosArray := js.Global.Get("Float32Array").New(6 * 2)
	posArray := jsPosArray.Interface().([]float32)

	posData := []float32{
		-1, -1,
		1, -1,
		-1, 1,
		-1, 1,
		1, -1,
		1, 1,
	}

	for i, v := range posData {
		posArray[i] = v
	}

	jsUVArray := js.Global.Get("Float32Array").New(6 * 2)
	uvArray := jsUVArray.Interface().([]float32)

	uvData := []float32{
		0, 0,
		1, 0,
		0, 1,
		0, 1,
		1, 0,
		1, 1,
	}

	for i, v := range uvData {
		uvArray[i] = v
	}

	return jsPosArray, jsUVArray
}

func setupGeometry(gl *webgl.Context, program *js.Object) {
	posArray, uvArray := buildArrays()

	gl.BindBuffer(gl.ARRAY_BUFFER, gl.CreateBuffer())
	gl.BufferData(gl.ARRAY_BUFFER, posArray, gl.STATIC_DRAW)

	positionLocation := gl.GetAttribLocation(program, "a_position")
	gl.EnableVertexAttribArray(positionLocation)
	gl.VertexAttribPointer(positionLocation, 2, gl.FLOAT, false, 0, 0)

	gl.BindBuffer(gl.ARRAY_BUFFER, gl.CreateBuffer())
	gl.BufferData(gl.ARRAY_BUFFER, uvArray, gl.STATIC_DRAW)

	texCoordLocation := gl.GetAttribLocation(program, "a_texCoord")
	gl.EnableVertexAttribArray(texCoordLocation)
	gl.VertexAttribPointer(texCoordLocation, 2, gl.FLOAT, false, 0, 0)

	glAssert(gl)
}

func setupTextures(gl *webgl.Context, program *js.Object) {
	gl.ActiveTexture(gl.TEXTURE1)
	gl.BindTexture(gl.TEXTURE_2D, gl.CreateTexture())

	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.NEAREST)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.NEAREST)

	rawPalette := make([]byte, 256*4)
	for i, c := range palette.Plan9 {
		r, g, b, a := c.RGBA()
		offset := i * 4

		rawPalette[offset] = byte(r)
		rawPalette[offset+1] = byte(g)
		rawPalette[offset+2] = byte(b)
		rawPalette[offset+3] = byte(a)
	}

	data := js.Global.Get("Uint8Array").New(js.NewArrayBuffer(rawPalette))
	gl.Call("texImage2D", gl.TEXTURE_2D, 0, gl.RGBA, 16, 16, 0, gl.RGBA, gl.UNSIGNED_BYTE, data)

	gl.ActiveTexture(gl.TEXTURE0)
	gl.BindTexture(gl.TEXTURE_2D, gl.CreateTexture())

	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.NEAREST)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.NEAREST)

	gl.Uniform1i(gl.GetUniformLocation(program, "s_texture"), 0)
	gl.Uniform1i(gl.GetUniformLocation(program, "s_palette"), 1)

	sizeLocation = gl.GetUniformLocation(program, "u_imageSize")

	glAssert(gl)
}

func setupGL(canvas *js.Object) *webgl.Context {
	gl, err := webgl.NewContext(canvas, webgl.DefaultAttributes())
	assert(err)

	program := setupShaders(gl)
	setupGeometry(gl, program)
	setupTextures(gl, program)

	glAssert(gl)

	gl.ClearColor(0, 0, 0, 1)
	gl.Clear(gl.COLOR_BUFFER_BIT)

	glAssert(gl)
	return gl
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
	document.Get("body").Call("appendChild", canvas)

	setupConnection(setupGL(canvas))
}

func main() {
	js.Global.Call("addEventListener", "load", func() { go load() })
}

var vsSource = `
	attribute vec2 a_position;
	attribute vec2 a_texCoord;

	varying vec2 v_texCoord;

	void main() {
		gl_Position = vec4(a_position, 0, 1);
		v_texCoord = a_texCoord;
	}
`

var psSource = `
	precision highp float;

	varying vec2 v_texCoord;

	uniform ivec3 u_imageSize;
	uniform sampler2D s_texture;
	uniform sampler2D s_palette;

	void main() {
		if ((u_imageSize.z == 0 && mod(v_texCoord.x, 2.0) == 0.0) || (u_imageSize.z == 1 && mod(v_texCoord.x, 2.0) > 0.0)) {
			float index = texture2D(s_texture, v_texCoord).r * 255.0;
			vec2 uv = vec2(mod(index, 16.0) / 16.0, (index / 16.0) / 16.0);
			gl_FragColor = texture2D(s_palette, uv);
		} else {
			gl_FragColor = vec4(0,0,0,0);
		}
	}
`
