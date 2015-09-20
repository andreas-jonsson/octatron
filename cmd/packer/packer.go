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
	infile, err := os.Open("test.priv.xyz")
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
		progress := -1

		for scanner.Scan() {
			text := scanner.Text()
			s := new(xyzSample)

			var ref float64
			_, err := fmt.Sscan(text, &s.pos.X, &s.pos.Y, &s.pos.Z, &ref, &s.r, &s.g, &s.b)
			if err != nil {
				return err
			}

			reads += int64(len(text) + 1)
			p := int((float64(reads) / float64(size)) * 100)
			if p > progress {
				progress = p
				fmt.Printf("\rProgress: %v%%", p)
			}

			samples <- s
		}

		return scanner.Err()
	}

	bounds := pack.Box{pack.Point{797, 698, 41.881}, 8.5}
	//bounds := pack.Box{pack.Point{733, 682, 40.4}, 8.1}

	cfg := pack.BuildConfig{parser, outfile, bounds, 256, pack.MipR8G8B8A8PackUI28, true, true, 0.25}

	status, err := pack.BuildTree(&cfg)
	if err != nil {
		panic(err)
	}
	fmt.Println(status)

	fmt.Println("Compressing...")

	zipfile, err := os.Create("test.priv.ocz")
	if err != nil {
		panic(err)
	}
	defer zipfile.Close()

	outfile.Seek(0, 0)
	if pack.CompressTree(outfile, zipfile); err != nil {
		panic(err)
	}
}

func main() {
	Start()
}
