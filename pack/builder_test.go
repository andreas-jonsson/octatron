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
	"bufio"
	"fmt"
	"os"
	"testing"
)

type xyzSample struct {
	pos     Point
	r, g, b byte
}

func (s *xyzSample) Color() Color {
	return Color{float32(s.r) / 256, float32(s.g) / 256, float32(s.b) / 256, 1}
}

func (s *xyzSample) Position() Point {
	return s.pos
}

func TestBuildTree(t *testing.T) {
	infile, err := os.Open("test.xyz")
	if err != nil {
		panic(err)
	}
	defer infile.Close()

	outfile, err := os.Create("test.oct")
	if err != nil {
		panic(err)
	}
	defer outfile.Close()

	parser := func(samples chan<- Sample) error {
		scanner := bufio.NewScanner(infile)
		for scanner.Scan() {
			text := scanner.Text()
			s := new(xyzSample)

			_, err := fmt.Sscan(text, &s.pos.X, &s.pos.Y, &s.pos.Z, &s.r, &s.g, &s.b)
			if err != nil {
				return err
			}

			samples <- s
		}
		return scanner.Err()
	}

	bounds := Box{Point{0, 0, 0}, 80}
	cfg := BuildConfig{parser, outfile, bounds, 8, MIP_R8G8B8A8_UI32, true, true, 0.25}

	status, err := BuildTree(&cfg)
	if err != nil {
		panic(err)
	}
	fmt.Println(status)
}
