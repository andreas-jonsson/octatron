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
	"encoding/binary"
	"io"
	"sort"
)

type Sample interface {
	Color() Color
	Position() Point
}

type Worker interface {
	Start(bounds Box, samples chan<- Sample) error
	Stop()
}

type sortedWorker struct {
	size   int64
	reader io.ReadSeeker
}

const defaultNodeSize = 8*3 + 4 // x,y,z float64 + r,g,b,a uint8

func (w *sortedWorker) Start(bounds Box, samples chan<- Sample) error {
	f := func(i int) bool {
		var samp filterSample
		_, err := w.reader.Seek(int64(i*defaultNodeSize), 0)
		if err != nil {
			panic(err)
		}

		err = binary.Read(w.reader, binary.BigEndian, &samp)
		if err != nil {
			panic(err)
		}

		return samp.Pos.X >= bounds.Pos.X
	}

	offset := sort.Search(int(w.size/defaultNodeSize), f)

	_, err := w.reader.Seek(int64(offset*defaultNodeSize), 0)
	if err != nil {
		return err
	}

	for {
		var sample filterSample
		err = binary.Read(w.reader, binary.BigEndian, &sample)
		if err == io.EOF {
			return nil
		} else if err != nil {
			return err
		}

		if bounds.Intersect(sample.Pos) == true {
			samples <- &sample
		} else if sample.Pos.X > bounds.Pos.X+bounds.Size {
			return nil
		}
	}
}

func (w *sortedWorker) Stop() {
}

func NewSortedWorker(reader io.ReadSeeker) (Worker, error) {
	var err error
	w := new(sortedWorker)

	w.size, err = fileSize(reader)
	if err != nil {
		return w, err
	}

	w.reader = reader
	return w, nil
}

type unsortedWorker struct {
	reader io.ReadSeeker
}

func (w *unsortedWorker) Start(bounds Box, samples chan<- Sample) error {
	for {
		var sample filterSample
		err := binary.Read(w.reader, binary.BigEndian, &sample)
		if err == io.EOF {
			w.reader.Seek(0, 0)
			return nil
		} else if err != nil {
			return err
		}

		if bounds.Intersect(sample.Pos) == true {
			samples <- &sample
		}
	}
}

func (w *unsortedWorker) Stop() {
}

func NewUnsortedWorker(reader io.ReadSeeker) (Worker, error) {
	return &unsortedWorker{reader}, nil
}
