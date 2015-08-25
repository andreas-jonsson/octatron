/************************************************************************/
/* Octatron                                                             */
/* Copyright (c) 2015 Andreas T Jonsson <mail@andreasjonsson.se>        */
/*                                                                      */
/* Octatron is free software: you can redistribute it and/or modify     */
/* it under the terms of the GNU General Public License as published by */
/* the Free Software Foundation, either version 3 of the License, or    */
/* (at your option) any later version.                                  */
/*                                                                      */
/* Octatron is distributed in the hope that it will be useful,          */
/* but WITHOUT ANY WARRANTY; without even the implied warranty of       */
/* MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the        */
/* GNU General Public License for more details.                         */
/*                                                                      */
/* You should have received a copy of the GNU General Public License    */
/* along with Octatron.  If not, see <http://www.gnu.org/licenses/>.    */
/************************************************************************/

package octatron

type Color struct {
    R, G, B, A float32
}

type Point struct {
    X, Y, Z float64
}

type Box struct {
    Pos Point
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

type Sample interface {
    Color() Color
    Position() Point
}

type Worker interface {
    Run(volume Box, samples chan Sample) error
    Stop()
}
