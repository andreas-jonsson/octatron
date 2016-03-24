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
	"bufio"
	"fmt"
	"os"
	"testing"
)

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

	var (
		s       Sample
		r, g, b byte
	)

	parser := func(samples chan<- Sample) error {
		scanner := bufio.NewScanner(infile)
		for scanner.Scan() {
			text := scanner.Text()

			var ref float32
			_, err := fmt.Sscan(text, &s.Pos.X, &s.Pos.Y, &s.Pos.Z, &ref, &r, &g, &b)
			if err != nil {
				return err
			}

			s.Col.R = float32(r) / 255
			s.Col.G = float32(g) / 255
			s.Col.B = float32(b) / 255
			s.Col.A = 1

			samples <- s
		}
		return scanner.Err()
	}

	bounds := Box{Point{0, 0, 0}, 80}
	cfg := BuildConfig{parser, outfile, bounds, 8, MipR8G8B8A8UnpackUI32, true, true, 0.25}

	status, err := BuildTree(&cfg)
	if err != nil {
		panic(err)
	}
	fmt.Println(status)
}
