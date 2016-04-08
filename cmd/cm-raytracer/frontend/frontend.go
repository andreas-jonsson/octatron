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
	"strconv"
	"time"

	"github.com/andreas-jonsson/octatron/trace"
	"github.com/gopherjs/gopherjs/js"
	"github.com/gopherjs/webgl"
	"github.com/gopherjs/websocket"
)

const (
	imgWidth        = 320
	imgHeight       = 180
	imgCmResolution = 512
	imgScale        = 2
	cameraSpeed     = 0.1
)

type (
	setupMessage struct {
		Resolution int     `resolution`
		ClearColor [4]byte `clear_color`
	}

	updateMessage struct {
		Position [3]float32 `position`
	}
)

var (
	keys       = make(map[int]bool)
	backBuffer = image.NewRGBA(image.Rect(0, 0, imgWidth, imgHeight))
	imgRect    = image.Rect(0, 0, imgCmResolution, imgCmResolution)
	mapsImages = [6]*image.RGBA{
		image.NewRGBA(imgRect),
		image.NewRGBA(imgRect),
		image.NewRGBA(imgRect),
		image.NewRGBA(imgRect),
		image.NewRGBA(imgRect),
		image.NewRGBA(imgRect),
	}

	frameId, numFrames int
	canvas             *js.Object
	camera             trace.FreeFlightCamera
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
	posArray, _ := buildArrays()

	gl.BindBuffer(gl.ARRAY_BUFFER, gl.CreateBuffer())
	gl.BufferData(gl.ARRAY_BUFFER, posArray, gl.STATIC_DRAW)

	positionLocation := gl.GetAttribLocation(program, "a_position")
	gl.EnableVertexAttribArray(positionLocation)
	gl.VertexAttribPointer(positionLocation, 2, gl.FLOAT, false, 0, 0)

	/*gl.BindBuffer(gl.ARRAY_BUFFER, gl.CreateBuffer())
	gl.BufferData(gl.ARRAY_BUFFER, uvArray, gl.STATIC_DRAW)

	texCoordLocation := gl.GetAttribLocation(program, "a_texCoord")
	gl.EnableVertexAttribArray(texCoordLocation)
	gl.VertexAttribPointer(texCoordLocation, 2, gl.FLOAT, false, 0, 0)*/
}

func setupTextures(gl *webgl.Context, program *js.Object) {
	gl.ActiveTexture(gl.TEXTURE0)
	gl.BindTexture(gl.TEXTURE_CUBE_MAP, gl.CreateTexture())

	for i := 0; i < 6; i++ {
		gl.Call("texImage2D", gl.TEXTURE_CUBE_MAP_POSITIVE_X+i, 0, gl.RGBA, imgCmResolution, imgCmResolution, 0, gl.RGBA, gl.UNSIGNED_BYTE, nil)
	}

	gl.TexParameteri(gl.TEXTURE_CUBE_MAP, gl.TEXTURE_MAG_FILTER, gl.LINEAR)
	gl.TexParameteri(gl.TEXTURE_CUBE_MAP, gl.TEXTURE_MIN_FILTER, gl.LINEAR)
	gl.TexParameteri(gl.TEXTURE_CUBE_MAP, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE)
	gl.TexParameteri(gl.TEXTURE_CUBE_MAP, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE)
	//gl.TexParameteri(gl.TEXTURE_CUBE_MAP, gl.TEXTURE_WRAP_R, gl.CLAMP_TO_EDGE)

	gl.Uniform1i(gl.GetUniformLocation(program, "s_texture"), 0)
}

func setupGL(canvas *js.Object) *webgl.Context {
	gl, err := webgl.NewContext(canvas, webgl.DefaultAttributes())
	assert(err)

	program := setupShaders(gl)
	setupGeometry(gl, program)
	setupTextures(gl, program)

	gl.ClearColor(0, 0, 0, 1)
	gl.Clear(gl.COLOR_BUFFER_BIT)

	return gl
}

func setupConnection(gl *webgl.Context) {
	document := js.Global.Get("document")
	location := document.Get("location")

	ws, err := websocket.New(fmt.Sprintf("ws://%s/render", location.Get("host")))
	assert(err)

	onOpen := func(ev *js.Object) {
		setup := setupMessage{
			Resolution: imgCmResolution,
			ClearColor: [4]byte{127, 127, 127, 255},
		}

		msg, err := json.Marshal(setup)
		assert(err)

		assert(ws.Send(string(msg)))

		go updateCamera(ws, gl)
	}

	onMessage := func(ev *js.Object) {
		data := js.Global.Get("Uint8Array").New(ev.Get("data"))
		gl.Call("texImage2D", gl.TEXTURE_CUBE_MAP_POSITIVE_X+(frameId%6), 0, gl.RGBA, imgCmResolution, imgCmResolution, 0, gl.RGBA, gl.UNSIGNED_BYTE, data)
		frameId++
	}

	ws.BinaryType = "arraybuffer"
	ws.AddEventListener("open", false, onOpen)
	ws.AddEventListener("message", false, onMessage)
}

func updateCamera(ws *websocket.WebSocket, gl *webgl.Context) {
	const tick30hz = (1000 / 30) * time.Millisecond

	var (
		msg    updateMessage
		camera trace.FreeFlightCamera
	)

	for _ = range time.Tick(tick30hz) {
		switch {
		case keys[38]: // Up
			camera.YRot += cameraSpeed
		case keys[40]: // Down
			camera.YRot -= cameraSpeed
		case keys[37]: // Left
			camera.XRot += cameraSpeed
		case keys[39]: // Right
			camera.XRot -= cameraSpeed
		case keys[87]: // W
			camera.Move(cameraSpeed)
		case keys[83]: // S
			camera.Move(-cameraSpeed)
		case keys[65]: // A
			camera.Strafe(cameraSpeed)
		case keys[68]: // D
			camera.Strafe(-cameraSpeed)
		case keys[69]: // E
			camera.Lift(cameraSpeed)
		case keys[81]: // Q
			camera.Lift(-cameraSpeed)
		}

		msg.Position = camera.Pos
		//msg.Camera.XRot = camera.XRot
		//msg.Camera.YRot = camera.YRot

		m, err := json.Marshal(msg)
		assert(err)

		assert(ws.Send(string(m)))

		gl.DrawArrays(gl.TRIANGLES, 0, 6)
		numFrames++
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
	document.Get("body").Call("appendChild", canvas)

	setupConnection(setupGL(canvas))
}

func main() {
	js.Global.Call("addEventListener", "load", func() { go load() })
}

var vsSource = `
	precision highp float;

	attribute vec2 a_position;
	varying vec3 v_position;

	uniform mat4 u_projection;
	uniform mat4 u_view;

	void main() {
		gl_Position = vec4(a_position, -1, 1);
		v_position = projection * view * gl_Position;
	}
`

var psSource = `
	precision highp float;

	varying vec3 v_position;
	uniform samplerCube s_texture;

	void main() {
		gl_FragColor = textureCube(s_texture, v_position);
	}
`
