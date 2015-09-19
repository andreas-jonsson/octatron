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
	"bufio"
	"fmt"
	"os"

	"github.com/andreas-t-jonsson/octatron/pack"
)

type xyzSample struct {
	pos     pack.Point
	r, g, b byte
}

func (s *xyzSample) Color() pack.Color {
	return pack.Color{float32(s.r) / 256, float32(s.g) / 256, float32(s.b) / 256, 1}
}

func (s *xyzSample) Position() pack.Point {
	return s.pos
}

func Start() {
	infile, err := os.Open("test2.priv.xyz")
	if err != nil {
		panic(err)
	}
	defer infile.Close()

	var reads int64
	size, _ := infile.Seek(0, 2)
	infile.Seek(0, 0)

	outfile, err := os.Create("test.priv.oct")
	if err != nil {
		panic(err)
	}
	defer outfile.Close()

	parser := func(samples chan<- pack.Sample) error {
		defer fmt.Println("\rProgress: 100%")
		scanner := bufio.NewScanner(infile)

		for scanner.Scan() {
			text := scanner.Text()
			s := new(xyzSample)

			var ref float64
			_, err := fmt.Sscan(text, &s.pos.X, &s.pos.Y, &s.pos.Z, &ref, &s.r, &s.g, &s.b)
			if err != nil {
				return err
			}

			reads += int64(len(text) + 1)
			fmt.Printf("\rProgress: %v%%", int(float64(reads)/float64(size)*100))

			samples <- s
		}

		return scanner.Err()
	}

	//bounds := Box{Point{0, 0, 0}, 80}
	bounds := pack.Box{pack.Point{733, 682, 40.4}, 8.1}
	//bounds := Box{Point{797, 698, 41.881}, 8.5}

	status, err := pack.BuildTree(&pack.BuildConfig{outfile, bounds, 512, pack.MIP_R8G8B8A8_UI32, true, 0.25}, parser)
	if err != nil {
		panic(err)
	}
	fmt.Println(status)
}

func main() {
	Start()
}
