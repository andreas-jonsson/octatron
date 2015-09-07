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
	"io"
	"os"
	"fmt"
	"testing"
)

type filterSample struct {
	pos   Point
	color Color
}

func (s *filterSample) Color() Color {
	return s.color
}

func (s *filterSample) Position() Point {
	return s.pos
}

func filter(input io.Reader, samples chan<- Sample) error {
	scanner := bufio.NewScanner(input)
	for scanner.Scan() {
		s := new(filterSample)

		var ref float64
		_, err := fmt.Sscan(scanner.Text(), &s.pos.X, &s.pos.Y, &s.pos.Z, &ref, &s.color.R, &s.color.G, &s.color.B)
		if err != nil {
			return err
		}

		samples <- s
	}

	return scanner.Err()
}

func TestFilter(t *testing.T) {
	in, _ := os.Open("test.xyz")
	defer in.Close()

	out, _ := os.Create("test.bin")
	defer out.Close()

	var cfg FilterConfig
	cfg.Writer = out
	cfg.Reader = in
	cfg.Function = filter

	if err := FilterInput(&cfg); err != nil {
		panic(err)
	}
}
