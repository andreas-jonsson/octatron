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
	"image"
	"image/color"
	"io"
	"math"

	"github.com/andreas-jonsson/octatron/pack"
	"github.com/ungerik/go3d/vec3"
)

type (
	infiniteRay [2]vec3.T

	octreeNode struct {
		color    color.RGBA
		children [8]uint32
	}
)

func (n *octreeNode) setColor(color *pack.Color) {
	n.color.R = uint8(color.R * 255)
	n.color.G = uint8(color.G * 255)
	n.color.B = uint8(color.B * 255)
	n.color.A = uint8(color.A * 255)
}

func loadOctree(reader io.Reader) ([]octreeNode, error) {
	var (
		color  pack.Color
		header pack.OctreeHeader
	)

	if err := pack.DecodeHeader(reader, &header); err != nil {
		return nil, err
	}

	data := make([]octreeNode, header.NumNodes)
	for i := range data {
		n := &data[i]
		if err := pack.DecodeNode(reader, header.Format, &color, n.children[:]); err != nil {
			return nil, err
		}
		n.setColor(&color)
	}

	return data, nil
}

func intersectBox(ray *infiniteRay, lenght float32, box *vec3.Box) float32 {
	origin := ray[0]
	direction := ray[1]

	oMin := vec3.Sub(&box.Min, &origin)
	oMax := vec3.Sub(&box.Max, &origin)

	oMin[0] /= direction[0]
	oMin[1] /= direction[1]
	oMin[2] /= direction[2]

	oMax[0] /= direction[0]
	oMax[1] /= direction[1]
	oMax[2] /= direction[2]

	mMax := vec3.Max(&oMax, &oMin)
	mMin := vec3.Min(&oMax, &oMin)

	final := math.Min(float64(mMax[0]), math.Min(float64(mMax[1]), float64(mMax[2])))
	start := math.Max(math.Max(float64(mMin[0]), 0.0), math.Max(float64(mMin[1]), float64(mMin[2])))

	dist := float32(math.Min(final, start))
	if final > start && dist < lenght {
		return dist
	}
	return lenght
}

var (
	clearColor     = color.RGBA{0, 0, 0, 255}
	childPositions = []vec3.T{
		vec3.T{0, 0, 0}, vec3.T{1, 0, 0}, vec3.T{0, 1, 0}, vec3.T{1, 1, 0},
		vec3.T{0, 0, 1}, vec3.T{1, 0, 1}, vec3.T{0, 1, 1}, vec3.T{1, 1, 1},
	}
)

func intersectTree(tree []octreeNode, ray *infiniteRay, nodePos vec3.T, nodeScale, length float32, nodeIndex uint32) (float32, color.RGBA) {
	var (
		color = clearColor
		node  = tree[nodeIndex]
	)

	box := vec3.Box{nodePos, vec3.T{nodePos[0] + nodeScale, nodePos[1] + nodeScale, nodePos[2] + nodeScale}}
	boxDist := intersectBox(ray, length, &box)

	if boxDist == length {
		return math.MaxFloat32, color
	}

	childScale := nodeScale * 0.5
	for i, childIndex := range node.children {
		if childIndex != 0 {
			scaled := childPositions[i].Scaled(childScale)
			pos := vec3.Add(&nodePos, &scaled)

			if dist, col := intersectTree(tree, ray, pos, childScale, length, childIndex); dist < math.MaxFloat32 {
				color = col
				boxDist = dist
			}
		}
	}

	return boxDist, node.color
}

func calcIncVectors(lookAtPoint, eyePoint, up vec3.T, width, height, fieldOfView float32) (vec3.T, vec3.T, vec3.T) {
	viewDirection := vec3.Sub(&lookAtPoint, &eyePoint)
	u := vec3.Cross(&viewDirection, &up)
	v := vec3.Cross(&u, &viewDirection)
	u.Normalize()
	v.Normalize()

	viewPlaneHalfWidth := float32(math.Tan(float64(fieldOfView / 2)))
	aspectRatio := height / width
	viewPlaneHalfHeight := aspectRatio * viewPlaneHalfWidth

	sV := v.Scaled(viewPlaneHalfHeight)
	sU := u.Scaled(viewPlaneHalfWidth)

	lookV := vec3.Sub(&lookAtPoint, &sV)
	viewPlaneBottomLeftPoint := vec3.Sub(&lookV, &sU)

	xIncVector := u.Scaled(2 * viewPlaneHalfWidth)
	yIncVector := v.Scaled(2 * viewPlaneHalfHeight)

	xIncVector[0] /= width
	xIncVector[1] /= width
	xIncVector[2] /= width

	yIncVector[0] /= height
	yIncVector[1] /= height
	yIncVector[2] /= height

	return xIncVector, yIncVector, viewPlaneBottomLeftPoint
}

func startTrace(tree []octreeNode, img *image.RGBA) {
	var (
		width  float32 = 640
		height float32 = 360
	)

	const fieldOfView = 45

	eyePoint := vec3.Zero
	xInc, yInc, bottomLeft := calcIncVectors(vec3.T{0, 0, -1}, eyePoint, vec3.T{0, 1, 0}, width, height, fieldOfView)

	for h := 0; h < int(height); h++ {
		for w := 0; w < int(width); w++ {
			x := xInc.Scaled(float32(w))
			y := yInc.Scaled(float32(h))

			x = vec3.Add(&x, &y)
			viewPlanePoint := vec3.Add(&bottomLeft, &x)

			dir := vec3.Sub(&viewPlanePoint, &eyePoint)
			dir.Normalize()

			ray := infiniteRay{vec3.Zero, dir}

			nodePos := vec3.T{-0.5, -0.5, -3}

			dist, _ := intersectTree(tree, &ray, nodePos, 1, math.MaxFloat32, 0)
			if dist != math.MaxFloat32 {
				img.SetRGBA(w, h, color.RGBA{byte(dist * 180), 0, 0, 255})
				//img.SetRGBA(w, h, col)
			}
			//img.SetRGBA(w, h, color.RGBA{byte(dir[0] * 127), byte(dir[1] * 127), byte(dir[2] * 127), 255})
		}
	}
}
