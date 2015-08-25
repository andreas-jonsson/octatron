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

import (
	"testing"
	"bufio"
	"fmt"
	"os"
)

type testSample struct {
	pos   Point
	color Color
}

func (s *testSample) Color() Color {
	return s.color
}

func (s *testSample) Position() Point {
	return s.pos
}

type testWorker struct {
	file *os.File
}

func (w *testWorker) Run(volume Box, samples chan Sample) error {
	scanner := bufio.NewScanner(w.file)
	for scanner.Scan() {
		s := new(testSample)

        var unknown float64
        var r, g, b int

		_, err := fmt.Sscan(scanner.Text(), &s.pos.X, &s.pos.Y, &s.pos.Z, &unknown, &r, &g, &b)
		if err != nil {
			return err
		}

        s.color.R = float32(r)
        s.color.G = float32(g)
        s.color.B = float32(b)

		if volume.Intersect(s.pos) {
			samples <- s
		}
	}

	_, err := w.file.Seek(0, 0)
	if err != nil {
		return err
	}

	return scanner.Err()
}

func (w *testWorker) Stop() {
	w.file.Close()
}

func createWorker(file string) *testWorker {
	var err error
	w := new(testWorker)

	w.file, err = os.Open(file)
	if err != nil {
		panic(err)
	}

	return w
}

func TestOctatron(t *testing.T) {
	workers := make([]Worker, 1)

	for i := range workers {
		workers[i] = createWorker("test.xyz")
	}

	file, err := os.Create("test.oct")
	if err != nil {
		panic(err)
	}
	defer file.Close()

	bounds := Box{Point{0.0, 0.0, 0.0}, 1000.0}

	_, err = BuildTree(workers, &TreeConfig{file, bounds, 10})
	if err != nil {
		panic(err)
	}
}
