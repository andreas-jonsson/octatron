/*************************************************************************/
/* Octatron                                                              */
/* Copyright (C) 2015 Andreas T Jonsson <mail@andreasjonsson.se>         */
/*                                                                       */
/* This program is free software: you can redistribute it and/or modify  */
/* it under the terms of the GNU General Public License as published by  */
/* the Free Software Foundation, either version 3 of the License, or     */
/* (at your option) any later version.                                   */
/*                                                                       */
/* This program is distributed in the hope that it will be useful,       */
/* but WITHOUT ANY WARRANTY; without even the implied warranty of        */
/* MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the         */
/* GNU General Public License for more details.                          */
/*                                                                       */
/* You should have received a copy of the GNU General Public License     */
/* along with this program.  If not, see <http://www.gnu.org/licenses/>. */
/*************************************************************************/

package main

import (
	"io/ioutil"

	"github.com/go-gl/gl/v3.2-core/gl"
	"github.com/go-gl/mathgl/mgl32"
	"github.com/veandco/go-sdl2/sdl"

	"fmt"
	"runtime"
)

const (
	winTitle  = "Octatron"
	winWidth  = 800
	winHeight = 600
)

func init() {
	runtime.LockOSThread()
}

func main() {
	var (
		err     error
		window  *sdl.Window
		context sdl.GLContext
	)

	if err = sdl.Init(sdl.INIT_EVERYTHING); err != nil {
		panic(err)
	}
	defer sdl.Quit()

	sdl.GL_SetAttribute(sdl.GL_CONTEXT_MAJOR_VERSION, 3)
	sdl.GL_SetAttribute(sdl.GL_CONTEXT_MINOR_VERSION, 2)
	sdl.GL_SetAttribute(sdl.GL_CONTEXT_PROFILE_MASK, sdl.GL_CONTEXT_PROFILE_CORE)

	window, err = sdl.CreateWindow(winTitle, sdl.WINDOWPOS_UNDEFINED, sdl.WINDOWPOS_UNDEFINED, winWidth, winHeight, sdl.WINDOW_OPENGL)
	if err != nil {
		panic(err)
	}
	defer window.Destroy()

	context, err = sdl.GL_CreateContext(window)
	if err != nil {
		panic(err)
	}
	defer sdl.GL_DeleteContext(context)

	if err = sdl.GL_MakeCurrent(window, context); err != nil {
		panic(err)
	}

	if err = gl.Init(); err != nil {
		panic(err)
	}

	printGLInfo()
	cameraMatrixUniform := setupGL()
	windowLoop(window, cameraMatrixUniform)
}

func printGLInfo() {
	fmt.Println("OpenGL version:", gl.GoStr(gl.GetString(gl.VERSION)))
	fmt.Println("OpenGL vendor:", gl.GoStr(gl.GetString(gl.VENDOR)))
	fmt.Println("OpenGL renderer:", gl.GoStr(gl.GetString(gl.RENDERER)))
}

func loadResources() (uint32, uint32) {
	texture, _, err := newOctree("cmd/packer/test.priv.oct")
	//texture, _, err := newOctree("pack/test.oct")
	if err != nil {
		panic(err)
	}

	vertexShader, err := ioutil.ReadFile("cmd/raytracer/vp.glsl")
	if err != nil {
		panic(err)
	}

	fragmentShader, err := ioutil.ReadFile("cmd/raytracer/fp.glsl")
	if err != nil {
		panic(err)
	}

	program, err := newProgram(string(vertexShader)+"\x00", string(fragmentShader)+"\x00")
	if err != nil {
		panic(err)
	}

	gl.UseProgram(program)
	return program, texture
}

func setupGL() int32 {
	gl.Disable(gl.DEPTH_TEST)
	gl.ClearColor(0.7, 0.7, 0.7, 1)

	program, _ := loadResources()

	gl.BindFragDataLocation(program, 0, gl.Str("outputColor\x00"))
	cameraMatrixUniform := gl.GetUniformLocation(program, gl.Str("cameraMatrix\x00"))
	octUniform := gl.GetUniformLocation(program, gl.Str("oct\x00"))
	gl.Uniform1i(octUniform, 0)

	gl.Viewport(0, 0, winWidth, winHeight)

	var vao uint32
	gl.GenVertexArrays(1, &vao)
	gl.BindVertexArray(vao)

	quadVertexBufferData := []float32{
		-1, -1, 0,
		1, -1, 0,
		-1, 1, 0,
		-1, 1, 0,
		1, -1, 0,
		1, 1, 0,
	}

	var vbo uint32
	gl.GenBuffers(1, &vbo)
	gl.BindBuffer(gl.ARRAY_BUFFER, vbo)
	gl.BufferData(gl.ARRAY_BUFFER, len(quadVertexBufferData)*4, gl.Ptr(quadVertexBufferData), gl.STATIC_DRAW)

	vertAttrib := uint32(gl.GetAttribLocation(program, gl.Str("inputPosition\x00")))
	gl.EnableVertexAttribArray(vertAttrib)
	gl.VertexAttribPointer(vertAttrib, 3, gl.FLOAT, false, 0, gl.PtrOffset(0))

	return cameraMatrixUniform
}

func windowLoop(window *sdl.Window, cameraMatrixUniform int32) {
	var (
		yrot       float32
		xrot       float32
		buttonDown bool
	)

	for {
		for event := sdl.PollEvent(); event != nil; event = sdl.PollEvent() {
			switch t := event.(type) {
			case *sdl.QuitEvent:
				return
			case *sdl.MouseButtonEvent:
				if t.State == 1 {
					buttonDown = true
				} else {
					buttonDown = false
				}
			case *sdl.MouseMotionEvent:
				if buttonDown {
					xrot += float32(t.YRel) * 0.001
					yrot += float32(t.XRel) * 0.001
				}
			}
		}

		gl.Clear(gl.COLOR_BUFFER_BIT | gl.DEPTH_BUFFER_BIT)

		xq := mgl32.QuatRotate(xrot, mgl32.Vec3{1, 0, 0})
		yq := mgl32.QuatRotate(yrot, mgl32.Vec3{0, 1, 0})

		mat := xq.Mul(yq).Mat4()
		//mat = mat.Mul4(mgl32.Translate3D(0,0,-0.5))

		gl.UniformMatrix4fv(cameraMatrixUniform, 1, false, &mat[0])

		gl.DrawArrays(gl.TRIANGLES, 0, 6)

		sdl.GL_SwapWindow(window)
		if glErr := gl.GetError(); glErr != gl.NO_ERROR {
			panic(fmt.Errorf("GL error, swap: %x", glErr))
		}
	}
}
