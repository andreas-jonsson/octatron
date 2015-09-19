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

package pack

import (
	"encoding/binary"
	"io"
	"math"
)

type Color struct {
	R, G, B, A float32
}

func (color *Color) scale(n float32) *Color {
	color.R *= n
	color.G *= n
	color.B *= n
	color.A *= n
	return color
}

func (color *Color) dist(c *Color) float32 {
	return float32(math.Sqrt(math.Pow(float64(c.R-color.R), 2) + math.Pow(float64(c.G-color.G), 2) + math.Pow(float64(c.B-color.B), 2) + math.Pow(float64(c.A-color.A), 2)))
}

func (color *Color) writeColor(writer io.Writer, format OctreeFormat) error {
	c := *color

	switch format {
	case MIP_R8G8B8A8_UI32:
		c.scale(256)
		err := binary.Write(writer, binary.BigEndian, byte(c.R))
		err = binary.Write(writer, binary.BigEndian, byte(c.G))
		err = binary.Write(writer, binary.BigEndian, byte(c.B))
		err = binary.Write(writer, binary.BigEndian, byte(c.A))
		return err
	case MIP_R8G8B8A8_UI16:
		c.scale(256)
		err := binary.Write(writer, binary.BigEndian, byte(c.R))
		err = binary.Write(writer, binary.BigEndian, byte(c.G))
		err = binary.Write(writer, binary.BigEndian, byte(c.B))
		err = binary.Write(writer, binary.BigEndian, byte(c.A))
		return err
	case MIP_R5G5B5A1_UI16:
		a := uint16(c.A) & 0x1
		c.scale(32)
		r := uint16(c.R) & 0x1f
		g := uint16(c.G) & 0x1f
		b := uint16(c.B) & 0x1f
		err := binary.Write(writer, binary.BigEndian, r<<11|g<<6|b<<1|a)
		return err
	default:
		return errUnsupportedFormat
	}
}

type Point struct {
	X, Y, Z float64
}

func (point *Point) scale(n float64) Point {
	return Point{point.X * n, point.Y * n, point.Z * n}
}

func (point *Point) add(p *Point) Point {
	return Point{point.X + p.X, point.Y + p.Y, point.Z + p.Z}
}

type Box struct {
	Pos  Point
	Size float64
}

func (b Box) Intersect(p Point) bool {
	max := Point{b.Pos.X + b.Size, b.Pos.Y + b.Size, b.Pos.Z + b.Size}
	if b.Pos.X < p.X && b.Pos.Y < p.Y && b.Pos.Z < p.Z {
		if max.X > p.X && max.Y > p.Y && max.Z > p.Z {
			return true
		}
	}
	return false
}
