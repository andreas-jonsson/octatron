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
	"github.com/andreas-jonsson/octatron/pack"
	"github.com/go-gl/gl/v3.2-compatibility/gl"
	"github.com/veandco/go-sdl2/sdl"

	"compress/zlib"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"os"
	"runtime"
)

const (
	winTitle  = "Octatron - Debugger"
	winWidth  = 800
	winHeight = 600

	rotSpeed = 10.0

	cloudScale     = 1
	cloudPointSize = 0.005
	cloudOffsetX   = -788
	cloudOffsetY   = -602
	cloudOffsetZ   = -48
)

type octreeNode struct {
	Color    pack.Color
	Children [8]uint32
}

type point3d struct {
	X, Y, Z float32
}

func (p *point3d) add(x *point3d) point3d {
	return point3d{p.X + x.X, p.Y + x.Y, p.Z + x.Z}
}

func (p *point3d) addn(n float32) point3d {
	return point3d{p.X + n, p.Y + n, p.Z + n}
}

func (p *point3d) scale(n float32) point3d {
	return point3d{p.X * n, p.Y * n, p.Z * n}
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
	gl.ClearColor(0.75, 0.75, 0.75, 1.0)

	gl.Enable(gl.DEPTH_TEST)
	gl.LineWidth(1)

	gl.Viewport(0, 0, winWidth, winHeight)
	gl.Hint(gl.PERSPECTIVE_CORRECTION_HINT, gl.NICEST)
	gluPerspective(45, float64(winWidth)/winHeight, 10, 1000)

	gl.MatrixMode(gl.MODELVIEW)
	gl.LoadIdentity()
}

func renderBox() {
	gl.Begin(gl.QUADS)
	gl.Vertex3f(1, 1, 0)
	gl.Vertex3f(0, 1, 0)
	gl.Vertex3f(0, 1, 1)
	gl.Vertex3f(1, 1, 1)

	gl.Vertex3f(1, 0, 1)
	gl.Vertex3f(0, 0, 1)
	gl.Vertex3f(0, 0, 0)
	gl.Vertex3f(1, 0, 0)

	gl.Vertex3f(1, 1, 1)
	gl.Vertex3f(0, 1, 1)
	gl.Vertex3f(0, 0, 1)
	gl.Vertex3f(1, 0, 1)

	gl.Vertex3f(1, 0, 0)
	gl.Vertex3f(0, 0, 0)
	gl.Vertex3f(0, 1, 0)
	gl.Vertex3f(1, 1, 0)

	gl.Vertex3f(0, 1, 1)
	gl.Vertex3f(0, 1, 0)
	gl.Vertex3f(0, 0, 0)
	gl.Vertex3f(0, 0, 1)

	gl.Vertex3f(1, 1, 0)
	gl.Vertex3f(1, 1, 1)
	gl.Vertex3f(1, 0, 1)
	gl.Vertex3f(1, 0, 0)
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

	ymax := zNear * math.Tan(fovy*math.Pi/360)
	ymin := -ymax
	xmin := ymin * aspect
	xmax := ymax * aspect

	gl.Frustum(xmin, xmax, ymin, ymax, zNear, zFar)
}

type renderData struct {
	yrot, xrot     float64
	zoom           float64
	minNodeSize    float32
	nodes          []octreeNode
	box            uint32
	renderSections bool
}

func drawAxis(data *renderData) {
	/*
		gl.LoadIdentity()

		gl.Translated(0, 0, -3)
		gl.Rotated(data.xrot, 1, 0, 0)
		gl.Rotated(data.yrot, 0, 1, 0)

		gl.LineWidth(2.5)
		gl.Begin(gl.LINES)
		gl.Color3f(255,0,0)
		gl.Vertex3f(10, 0, 0)
		gl.Vertex3f(-10, 0, 0)
		gl.End()
		gl.LineWidth(1)
	*/
}

func windowLoop(window *sdl.Window) {
	data := &renderData{}

	data.renderSections = true
	data.zoom = -250
	data.nodes = loadTree("cmd/packer/test.priv.oct")
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
				switch t.Keysym.Sym {
				case sdl.K_ESCAPE:
					return
				case sdl.K_SPACE:
					data.renderSections = !data.renderSections
				case sdl.K_PLUS:
					data.minNodeSize += 1.0
				case sdl.K_MINUS:
					data.minNodeSize -= 1.0
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

		renderTree(data, &data.nodes[0], point3d{0, 0, 0}, 100)

		gl.Disable(gl.DEPTH_TEST)
		drawAxis(data)
		gl.Enable(gl.DEPTH_TEST)

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

	var header pack.OctreeHeader
	if err := binary.Read(fp, binary.LittleEndian, &header); err != nil {
		panic(err)
	}

	var reader io.Reader
	if header.Compressed() == true {
		zipReader, err := zlib.NewReader(fp)
		if err != nil {
			panic(err)
		}
		zipReader.Close()
		reader = zipReader
	} else {
		reader = fp
	}

	numNodes := header.NumNodes
	nodes := make([]octreeNode, numNodes)

	for i := uint64(0); i < numNodes; i++ {
		node := &nodes[i]
		if err := pack.DecodeNode(reader, header.Format, &node.Color, node.Children[:]); err != nil {
			panic(err)
		}
	}

	return nodes
}

var childPositions = []point3d{
	point3d{0, 0, 0}, point3d{1, 0, 0}, point3d{0, 1, 0}, point3d{1, 1, 0},
	point3d{0, 0, 1}, point3d{1, 0, 1}, point3d{0, 1, 1}, point3d{1, 1, 1},
}

func renderTree(data *renderData, node *octreeNode, pos point3d, size float32) {
	var candidates [8]*octreeNode
	var candidatesPos [8]point3d

	for {
		num := 0
		childSize := size * 0.5
		for i, child := range node.Children {
			if child != 0 {
				candidates[num] = &data.nodes[child]
				childPos := childPositions[i].scale(childSize)
				childPos = pos.add(&childPos)
				candidatesPos[num] = childPos
				num++
			}
		}

		// Is this a leaf
		if num == 0 || size < data.minNodeSize {
			renderNode(data, node.Color, pos, size)
			return
		} else if data.renderSections == true {
			gl.PolygonMode(gl.FRONT_AND_BACK, gl.LINE)
			renderNode(data, pack.Color{0, 1, 0, 1}, pos, size)
			gl.PolygonMode(gl.FRONT_AND_BACK, gl.FILL)
		}

		num--
		for i := 0; i < num; i++ {
			renderTree(data, candidates[i], candidatesPos[i], childSize)
		}

		node = candidates[num]
		pos = candidatesPos[num]
		size = childSize
	}
}

func renderNode(data *renderData, color pack.Color, pos point3d, size float32) {
	gl.LoadIdentity()

	gl.Translated(0, 0, data.zoom)
	gl.Rotated(data.xrot, 1, 0, 0)
	gl.Rotated(data.yrot, 0, 1, 0)

	gl.Translatef(pos.X, pos.Y, pos.Z)
	gl.Scalef(size, size, size)

	gl.Color3f(color.R, color.G, color.B)
	gl.CallList(data.box)
}
