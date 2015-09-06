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
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"runtime"
	"testing"
)

const memLimitMB = 16

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
	file   *os.File
	mem    []byte
	reader io.Reader
}

func (w *testWorker) Start(bounds Box, samples chan<- Sample) error {
	scanner := bufio.NewScanner(w.reader)
	for scanner.Scan() {
		s := new(testSample)

		var ref float64
		_, err := fmt.Sscan(scanner.Text(), &s.pos.X, &s.pos.Y, &s.pos.Z, &ref, &s.color.R, &s.color.G, &s.color.B)
		if err != nil {
			return err
		}

		if bounds.Intersect(s.pos) == true {
			s.color.Scale(256.0)
			samples <- s
		}
	}

	if w.file != nil {
		_, err := w.file.Seek(0, 0)
		if err != nil {
			return err
		}
	} else {
		w.reader = bytes.NewReader(w.mem)
	}

	return scanner.Err()
}

func (w *testWorker) Stop() {
	if w.file != nil {
		w.file.Close()
	}
}

func createWorker(file string) *testWorker {
	var err error
	w := new(testWorker)

	w.file, err = os.Open(file)
	if err != nil {
		panic(err)
	}

	size, err := w.file.Seek(0, 2)
	if err != nil {
		panic(err)
	}

	size = size / 1024 / 1024
	if size < memLimitMB {
		w.file.Close()
		w.file = nil

		fmt.Printf("Worker [%p], caching file in memory! (%vMB < %vMB)\n", w, size, memLimitMB)

		w.mem, err = ioutil.ReadFile(file)
		if err != nil {
			panic(err)
		}
		w.reader = bytes.NewReader(w.mem)
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

	bounds := Box{Point{0, 0, 0}, 80}

	err = BuildTree(workers, &BuildConfig{file, bounds, 8, MIP_R8G8B8A8_UI32, true})
	if err != nil {
		panic(err)
	}
}

func TestWorker(t *testing.T) {
	start(4)
}

func BenchmarkWorker(b *testing.B) {
	num := runtime.NumCPU()
	for i := 0; i < b.N; i++ {
		start(num)
	}
}
