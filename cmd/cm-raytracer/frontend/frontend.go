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

	"github.com/andreas-jonsson/octatron/go3d/mat4"
	"github.com/andreas-jonsson/octatron/trace"
	"github.com/gopherjs/gopherjs/js"
	"github.com/gopherjs/webgl"
	"github.com/gopherjs/websocket"
)

const (
	imgCmResolution = 256
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
	imgRect    = image.Rect(0, 0, imgCmResolution, imgCmResolution)
	mapsImages = [6]*image.RGBA{
		image.NewRGBA(imgRect),
		image.NewRGBA(imgRect),
		image.NewRGBA(imgRect),
		image.NewRGBA(imgRect),
		image.NewRGBA(imgRect),
		image.NewRGBA(imgRect),
	}

	frameId int
	canvas,
	uniformViewLocation *js.Object
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

func buildArray() *js.Object {
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

	return jsPosArray
}

func setupGeometry(gl *webgl.Context, program *js.Object) {
	gl.BindBuffer(gl.ARRAY_BUFFER, gl.CreateBuffer())
	gl.BufferData(gl.ARRAY_BUFFER, buildArray(), gl.STATIC_DRAW)

	positionLocation := gl.GetAttribLocation(program, "a_position")
	gl.EnableVertexAttribArray(positionLocation)
	gl.VertexAttribPointer(positionLocation, 2, gl.FLOAT, false, 0, 0)
}

func setupTextures(gl *webgl.Context, program *js.Object) {
	gl.ActiveTexture(gl.TEXTURE0)
	gl.BindTexture(gl.TEXTURE_CUBE_MAP, gl.CreateTexture())

	for i := 0; i < 6; i++ {
		gl.Call("texImage2D", gl.TEXTURE_CUBE_MAP_POSITIVE_X+i, 0, gl.RGBA, imgCmResolution, imgCmResolution, 0, gl.RGBA, gl.UNSIGNED_BYTE, nil)
	}

	gl.TexParameteri(gl.TEXTURE_CUBE_MAP, gl.TEXTURE_MAG_FILTER, gl.LINEAR)
	gl.TexParameteri(gl.TEXTURE_CUBE_MAP, gl.TEXTURE_MIN_FILTER, gl.LINEAR)

	/*
		gl.TexParameteri(gl.TEXTURE_CUBE_MAP, gl.TEXTURE_MAG_FILTER, gl.NEAREST)
		gl.TexParameteri(gl.TEXTURE_CUBE_MAP, gl.TEXTURE_MIN_FILTER, gl.NEAREST)
	*/

	gl.TexParameteri(gl.TEXTURE_CUBE_MAP, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE)
	gl.TexParameteri(gl.TEXTURE_CUBE_MAP, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE)

	gl.Uniform1i(gl.GetUniformLocation(program, "s_texture"), 0)
}

func setupGL(canvas *js.Object) *webgl.Context {
	gl, err := webgl.NewContext(canvas, webgl.DefaultAttributes())
	assert(err)

	program := setupShaders(gl)
	setupGeometry(gl, program)
	setupTextures(gl, program)

	uniformViewLocation = gl.GetUniformLocation(program, "u_view")

	gl.ClearColor(0, 0, 0, 1)
	gl.Clear(gl.COLOR_BUFFER_BIT)

	return gl
}

func setupConnection(gl *webgl.Context) {
	document := js.Global.Get("document")
	location := document.Get("location")

	ws, err := websocket.New(fmt.Sprintf("ws://%s/render", location.Get("host")))
	assert(err)

	renderer := make(chan struct{})

	onOpen := func(ev *js.Object) {
		setup := setupMessage{
			Resolution: imgCmResolution,
			ClearColor: [4]byte{127, 127, 127, 255},
		}

		msg, err := json.Marshal(setup)
		assert(err)

		assert(ws.Send(string(msg)))

		go updateCamera(ws, gl, renderer)
	}

	onMessage := func(ev *js.Object) {
		face := frameId % 6
		fmt.Println("Received face:", face)

		data := js.Global.Get("Uint8Array").New(ev.Get("data"))
		gl.Call("texImage2D", gl.TEXTURE_CUBE_MAP_POSITIVE_X+face, 0, gl.RGBA, imgCmResolution, imgCmResolution, 0, gl.RGBA, gl.UNSIGNED_BYTE, data)
		frameId++

		select {
		case renderer <- struct{}{}:
		default:
		}
	}

	ws.BinaryType = "arraybuffer"
	ws.AddEventListener("open", false, onOpen)
	ws.AddEventListener("message", false, onMessage)
}

func updateCamera(ws *websocket.WebSocket, gl *webgl.Context, renderer <-chan struct{}) {
	const tick30hz = (1000 / 30) * time.Millisecond

	var (
		camera    trace.FreeFlightCamera
		oldPos    [3]float32
		positions = make(chan [3]float32, 1)
	)

	positions <- oldPos

	go func() {
		for {
			pos := <-positions
			fmt.Println("New position:", pos)

			m, err := json.Marshal(updateMessage{Position: pos})
			assert(err)
			assert(ws.Send(string(m)))
			<-renderer
		}
	}()

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

		if oldPos != camera.Pos {
			select {
			case positions <- camera.Pos:
				oldPos = camera.Pos
			default:
			}
		}

		mat := mat4.Ident
		mat.AssignEulerRotation(camera.XRot, camera.YRot, 0)
		mat.Transpose()

		gl.UniformMatrix4fv(uniformViewLocation, false, mat.Slice())
		gl.DrawArrays(gl.TRIANGLES, 0, 6)
	}
}

func load() {
	document := js.Global.Get("document")
	document.Set("title", "AJ's Cubemap Raytracer")

	document.Set("onkeydown", func(e *js.Object) {
		keys[e.Get("keyCode").Int()] = true
	})

	document.Set("onkeyup", func(e *js.Object) {
		keys[e.Get("keyCode").Int()] = false
	})

	canvas := document.Call("createElement", "canvas")
	canvas.Call("setAttribute", "width", strconv.Itoa(imgCmResolution))
	canvas.Call("setAttribute", "height", strconv.Itoa(imgCmResolution))
	canvas.Get("style").Set("width", strconv.Itoa(imgCmResolution*imgScale)+"px")
	canvas.Get("style").Set("height", strconv.Itoa(imgCmResolution*imgScale)+"px")
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
	uniform mat4 u_view;

	const float field_of_view = -1.2;

	void main() {
		vec4 pos = vec4(a_position, -1, 1);
		gl_Position = pos;

		pos.z = field_of_view;
		v_position = (u_view * pos).xyz;
	}
`

var psSource = `
	precision highp float;

	varying vec3 v_position;
	uniform samplerCube s_texture;

	void main() {
		gl_FragColor = vec4(textureCube(s_texture, v_position.xyz).rgb, 1);
	}
`
