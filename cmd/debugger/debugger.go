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
	"github.com/go-gl/gl/v3.2-compatibility/gl"
	"github.com/veandco/go-sdl2/sdl"

	"os"
	"fmt"
	"math"
	"runtime"
	"encoding/binary"
)

const (
	winTitle  = "Octatron - Debugger"
	winWidth  = 800
	winHeight = 600

	nodeSize = 4 + 8 * 4
	rotSpeed float64 = 10
)

type octreeNode struct {
	Color [4]byte
	Children [8]uint32
}

type point3d struct {
	X, Y, Z float32
}

func (p *point3d) add(x *point3d) point3d {
	return point3d{p.X + x.X, p.Y + x.Y, p.Z + x.Z}
}

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

	sdl.GL_SetAttribute(sdl.GL_CONTEXT_PROFILE_MASK, sdl.GL_CONTEXT_PROFILE_COMPATIBILITY)

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

	sdl.GL_SetSwapInterval(1)

	printGLInfo()
	setupGL()
	windowLoop(window)
}

func printGLInfo() {
	fmt.Println("OpenGL version:", gl.GoStr(gl.GetString(gl.VERSION)))
	fmt.Println("OpenGL vendor:", gl.GoStr(gl.GetString(gl.VENDOR)))
	fmt.Println("OpenGL renderer:", gl.GoStr(gl.GetString(gl.RENDERER)))
}

func setupGL() {
	gl.ShadeModel(gl.FLAT)
    gl.ClearColor(0.75,0.75,0.75,1.0)

    gl.Enable(gl.DEPTH_TEST)

	gl.Viewport(0, 0, winWidth, winHeight)
	gl.Hint(gl.PERSPECTIVE_CORRECTION_HINT, gl.NICEST)
	gluPerspective(45, float64(winWidth) / winHeight, 0.1, 1000.0)

	gl.MatrixMode(gl.MODELVIEW)
	gl.LoadIdentity()
}

func renderBox() {
	gl.Begin(gl.QUADS)
	gl.Vertex3f( 1,  1, -1)
	gl.Vertex3f(-1,  1, -1)
	gl.Vertex3f(-1,  1,  1)
	gl.Vertex3f( 1,  1,  1)

	gl.Vertex3f( 1, -1,  1)
	gl.Vertex3f(-1, -1,  1)
	gl.Vertex3f(-1, -1, -1)
	gl.Vertex3f( 1, -1, -1)

	gl.Vertex3f( 1,  1,  1)
	gl.Vertex3f(-1,  1,  1)
	gl.Vertex3f(-1, -1,  1)
	gl.Vertex3f( 1, -1,  1)

	gl.Vertex3f( 1, -1, -1)
	gl.Vertex3f(-1, -1, -1)
	gl.Vertex3f(-1,  1, -1)
	gl.Vertex3f( 1,  1, -1)

	gl.Vertex3f(-1,  1,  1)
	gl.Vertex3f(-1,  1, -1)
	gl.Vertex3f(-1, -1, -1)
	gl.Vertex3f(-1, -1,  1)

	gl.Vertex3f( 1,  1, -1)
	gl.Vertex3f( 1,  1,  1)
	gl.Vertex3f( 1, -1,  1)
	gl.Vertex3f( 1, -1, -1)
	gl.End()
}

func genBox() uint32 {
	box := gl.GenLists(1)
	gl.NewList(box, gl.COMPILE)
	renderBox()
	gl.EndList()
	return box
}

func gluPerspective(fovy float64, aspect float64, zNear float64, zFar float64) {
    gl.MatrixMode(gl.PROJECTION)
    gl.LoadIdentity()

    ymax := zNear * math.Tan(fovy * math.Pi / 360)
    ymin := -ymax
    xmin := ymin * aspect
    xmax := ymax * aspect

    gl.Frustum(xmin, xmax, ymin, ymax, zNear, zFar)
}

type renderData struct {
	yrot, xrot float64
	zoom float64
	nodes []octreeNode
	box uint32
	renderSections bool
}

func windowLoop(window *sdl.Window) {
	data := &renderData{}

	data.renderSections = true
	data.zoom = -7
	data.nodes = loadTree("pack/test.oct")
	data.box = genBox()
	defer gl.DeleteLists(data.box, 1)

	buttonDown := false
	t := float64(sdl.GetTicks())
	for {
		ticks := float64(sdl.GetTicks()) * 0.001
		dt := ticks - t
		t = ticks

		for event := sdl.PollEvent(); event != nil; event = sdl.PollEvent() {
			switch t := event.(type) {
			case *sdl.QuitEvent:
				return
			case *sdl.KeyDownEvent:
				if t.Keysym.Sym == sdl.K_ESCAPE {
					return
				} else if t.Keysym.Sym == sdl.K_SPACE {
					data.renderSections = !data.renderSections
				}
			case *sdl.MouseButtonEvent:
				if t.State == 1 {
					buttonDown = true
				} else {
					buttonDown = false
				}
			case *sdl.MouseMotionEvent:
				if buttonDown {
					data.xrot += dt * float64(t.YRel) * rotSpeed
					data.yrot += dt * float64(t.XRel) * rotSpeed
				}
			case *sdl.MouseWheelEvent:
				data.zoom += dt * float64(t.Y) * rotSpeed
			}
		}

		gl.Clear(gl.COLOR_BUFFER_BIT | gl.DEPTH_BUFFER_BIT)
		renderTree(data, &data.nodes[0], point3d{0,0,0}, 100)
		sdl.GL_SwapWindow(window)

		if glErr := gl.GetError(); glErr != gl.NO_ERROR {
			panic(fmt.Errorf("GL error: %x", glErr))
		}
	}
}

func loadTree(file string) []octreeNode {
	fp, err := os.Open(file)
	if err != nil {
		panic(err)
	}
	defer fp.Close()

	size, _ := fp.Seek(0, 2)
	fp.Seek(0, 0)

	numNodes := int(size) / nodeSize
	nodes := make([]octreeNode, numNodes)

	prog := -1
	for i := 0; i < numNodes; i++ {
		err := binary.Read(fp, binary.BigEndian, &nodes[i])
		if err != nil {
			panic(err)
		}

		p := int((float32(i + 1) / float32(numNodes)) * 100)
		if p > prog {
			fmt.Printf("\rLoading: %v%%", p)
			prog = p
		}
	}

	fmt.Println("")
	return nodes
}

var childPositions = []point3d{
	point3d{-1,-1,-1},point3d{1,-1,-1},point3d{-1,1,-1},point3d{1,1,-1},
	point3d{-1,-1,1},point3d{1,-1,1},point3d{-1,1,1},point3d{1,1,1},
}

func renderTree(data *renderData, node *octreeNode, pos point3d, size float32) {
	var candidates [8]*octreeNode
	var candidatesPos [8]*point3d

	for {
		num := 0
		for i, child := range node.Children {
			childPos := &childPositions[i]
			if child != 0 {
				candidates[num] = &data.nodes[child / nodeSize]
				candidatesPos[num] = childPos
				num++
			}

			if data.renderSections == true {
				gl.PolygonMode(gl.FRONT_AND_BACK, gl.LINE)
				renderNode(data, [4]byte{0,255,0,255}, pos.add(childPos), size * 0.5)
				gl.PolygonMode(gl.FRONT_AND_BACK, gl.FILL)
			}
		}

		// Is this a leaf
		if num == 0 {
			renderNode(data, node.Color, pos, size)
			return
		}

		num--
		size *= 0.5

		for i := 0; i < num; i++ {
			renderTree(data, candidates[i], pos.add(candidatesPos[i]), size)
		}

		node = candidates[num]
		pos = pos.add(candidatesPos[num])
	}
}

func renderNode(data *renderData, color [4]byte, pos point3d, size float32) {
	gl.LoadIdentity()

	gl.Translated(0,0,data.zoom)
	gl.Rotated(data.xrot, 1,0,0)
	gl.Rotated(data.yrot, 0,1,0)

	gl.Translatef(pos.X * size, pos.Y * size, pos.Z * size)
	gl.Scalef(size, size, size)

	gl.Color3f(float32(color[0]) / 256, float32(color[1]) / 256, float32(color[2]) / 256)

	gl.CallList(data.box)
	//renderBox()
}
