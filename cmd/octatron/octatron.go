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
    "flag"
	"runtime"
    "github.com/andreas-t-jonsson/octatron/pack"
)

type Sample struct {
	pos   pack.Point
	color pack.Color
}

func (s *Sample) Color() pack.Color {
	return s.color
}

func (s *Sample) Position() pack.Point {
	return s.pos
}

type Worker struct {
	file *os.File
}

func (w *Worker) Start(bounds pack.Box, samples chan<- pack.Sample) error {
	scanner := bufio.NewScanner(w.file)
	for scanner.Scan() {
		s := new(Sample)

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

func (w *Worker) Stop() {
	w.file.Close()
}

func createWorker(file string) (*Worker, error) {
	var err error
	w := new(Worker)
	w.file, err = os.Open(file)
	return w, err
}

var (
    input string
    output string
)

func init() {
    flag.StringVar(&input, "in", "in.xyz", "file to process")
    flag.StringVar(&output, "out", "out.oct", "file to write")
}

func main() {
    flag.Parse()

    var err error
	workers := make([]pack.Worker, runtime.NumCPU())

	for i := range workers {
		workers[i], err = createWorker(input)
        if err != nil {
            fmt.Println(err)
            os.Exit(-1)
    	}
	}

	file, err := os.Create(output)
	if err != nil {
        fmt.Println(err)
        os.Exit(-2)
	}
	defer file.Close()

	bounds := pack.Box{pack.Point{0.0, 0.0, 0.0}, 1000.0}
	err = pack.BuildTree(workers, &pack.BuildConfig{file, bounds, 1024, pack.Mip_R8G8B8_Branch32, true})
    if err != nil {
        fmt.Println(err)
        os.Exit(-3)
	}
}
