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

package octatron

import (
	"bufio"
	"fmt"
	"os"
	"runtime"
	"testing"
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

func (w *testWorker) Start(bounds Box, samples chan<- Sample) error {
	scanner := bufio.NewScanner(w.file)
	for scanner.Scan() {
		s := new(testSample)

		var ref float64
		_, err := fmt.Sscan(scanner.Text(), &s.pos.X, &s.pos.Y, &s.pos.Z, &ref, &s.color.R, &s.color.G, &s.color.B)
		if err != nil {
			return err
		}

		if bounds.Intersect(s.pos) == true {
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

func start(numWorkers int) {
	workers := make([]Worker, numWorkers)

	for i := range workers {
		workers[i] = createWorker("test.xyz")
	}

	file, err := os.Create("test.oct")
	if err != nil {
		panic(err)
	}
	defer file.Close()

	bounds := Box{Point{0.0, 0.0, 0.0}, 1000.0}

	err = BuildTree(workers, &BuildConfig{file, bounds, 8, Mip_R8G8B8_Branch32})
	if err != nil {
		panic(err)
	}
}

func TestSingleWorker(t *testing.T) {
	start(1)
}

func TestMultiWorker(t *testing.T) {
	start(100)
}

func BenchmarkWorker(b *testing.B) {
	num := runtime.NumCPU()
	for i := 0; i < b.N; i++ {
		start(num)
	}
}
