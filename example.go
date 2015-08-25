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

package main

import (
    "github.com/andreas-t-jonsson/octatron"
    "os"
    "fmt"
    "bufio"
)

type Sample struct {
    pos octatron.Point
    color octatron.Color
}

func (s *Sample) Color() octatron.Color {
    return s.color
}

func (s *Sample) Position() octatron.Point {
 	return s.pos
}

type Worker struct {
    file *os.File
}

func (w *Worker) Run(volume octatron.Box, samples chan octatron.Sample) error {
    scanner := bufio.NewScanner(w.file)
    for scanner.Scan() {
        s := new(Sample)

        _, err := fmt.Sscan(scanner.Text(), &s.pos.X, &s.pos.Y, &s.pos.Z, &s.color.R, &s.color.G, &s.color.B)
        if err != nil {
            return err
        }

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

func (w *Worker) Stop() {
 	w.file.Close()
}

func createWorker(file string) *Worker {
    var err error
    w := new(Worker)

    w.file, err = os.Open("small.xyz")
    if err != nil {
        panic(err)
    }

    return w
}

func main() {
    workers := make([]octatron.Worker, 1)

    for i := range workers {
        workers[i] = createWorker("small.xyz")
    }

    file, err := os.Create("small.oct")
    if err != nil {
        panic(err)
    }
    defer file.Close()

    bounds := octatron.Box{octatron.Point{0.0, 0.0, 0.0}, 1000.0}

    _, err = octatron.BuildTree(workers, &octatron.TreeConfig{file, bounds, 10})
    if err != nil {
        panic(err)
    }
}
