/*
Copyright (C) 2015-2016 Andreas T Jonsson

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

func (color *Color) setComponent(comp int, val float32) {
	switch comp {
	case 0:
		color.R = val
	case 1:
		color.G = val
	case 2:
		color.B = val
	case 3:
		color.A = val
	default:
		panic("invalid color component")
	}
}

func (color *Color) bytes() [4]byte {
	return [4]byte{byte(color.R * 255), byte(color.G * 255), byte(color.B * 255), byte(color.A * 255)}
}

func (color *Color) dist(c *Color) float32 {
	return float32(math.Sqrt(math.Pow(float64(c.R-color.R), 2) + math.Pow(float64(c.G-color.G), 2) + math.Pow(float64(c.B-color.B), 2) + math.Pow(float64(c.A-color.A), 2)))
}

func (color *Color) writeColor(writer io.Writer, format OctreeFormat) error {
	c := *color

	switch format {
	case MipR8G8B8A8UnpackUI32:
		c.scale(255)
		err := binary.Write(writer, binary.LittleEndian, byte(c.R))
		err = binary.Write(writer, binary.LittleEndian, byte(c.G))
		err = binary.Write(writer, binary.LittleEndian, byte(c.B))
		err = binary.Write(writer, binary.LittleEndian, byte(c.A))
		return err
	case MipR8G8B8A8UnpackUI16:
		c.scale(255)
		err := binary.Write(writer, binary.LittleEndian, byte(c.R))
		err = binary.Write(writer, binary.LittleEndian, byte(c.G))
		err = binary.Write(writer, binary.LittleEndian, byte(c.B))
		err = binary.Write(writer, binary.LittleEndian, byte(c.A))
		return err
	case MipR4G4B4A4UnpackUI16:
		c.scale(15)
		r := uint16(c.R) & 0xf
		g := uint16(c.G) & 0xf
		b := uint16(c.B) & 0xf
		a := uint16(c.A) & 0xf
		err := binary.Write(writer, binary.LittleEndian, r<<12|g<<8|b<<4|a)
		return err
	case MipR5G6B5UnpackUI16:
		r := uint16(c.R*31) & 0x1f
		g := uint16(c.G*63) & 0x3f
		b := uint16(c.B*31) & 0x1f
		err := binary.Write(writer, binary.LittleEndian, r<<11|g<<5|b)
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
