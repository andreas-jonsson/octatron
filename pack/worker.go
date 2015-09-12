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
	"bytes"
	"encoding/binary"
	"io"
	"io/ioutil"
	"os"
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

type WorkerSharedMemory struct {
	maxSize    int64
	fileName   string
	sharedFile *[]byte
}

func NewWorkerSharedMemory(maxSizeMB int64) *WorkerSharedMemory {
	return &WorkerSharedMemory{maxSizeMB * 1024 * 1024, "", nil}
}

const defaultNodeSize = 8*3 + 4 // x,y,z float64 + r,g,b,a uint8

type sortedWorker struct {
	file   *os.File
	size   int64
	reader io.ReadSeeker
	pool   *WorkerSharedMemory
}

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
	if w.file != nil {
		w.file.Close()
	}
}

func NewSortedWorker(inputFile string, pool *WorkerSharedMemory) (Worker, error) {
	var err error
	w := new(sortedWorker)

	w.size, err = FileSizeByName(inputFile)
	if err != nil {
		return w, err
	}

	if pool != nil && pool.maxSize < w.size {
		pool = nil
	}

	if pool != nil {
		if pool.sharedFile == nil {
			data, err := ioutil.ReadFile(inputFile)
			if err != nil {
				return w, err
			}

			pool.fileName = inputFile
			pool.sharedFile = &data
		} else if pool.fileName != inputFile {
			return w, errInvalidFile
		}

		w.size = int64(len(*pool.sharedFile))
		w.reader = bytes.NewReader(*pool.sharedFile)
		w.pool = pool
	} else {
		w.file, err = os.Open(inputFile)
		if err != nil {
			return w, err
		}
		w.reader = w.file
	}

	return w, nil
}

type unsortedWorker struct {
	file   *os.File
	reader io.ReadSeeker
	pool   *WorkerSharedMemory
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
	if w.file != nil {
		w.file.Close()
	}
}

func NewUnsortedWorker(inputFile string, pool *WorkerSharedMemory) (Worker, error) {
	var err error
	w := new(unsortedWorker)

	if pool != nil {
		size, err := FileSizeByName(inputFile)
		if err != nil {
			return w, err
		}

		if pool.maxSize < size {
			pool = nil
		}
	}

	if pool != nil {
		w.pool = pool
		if pool.sharedFile == nil {
			data, err := ioutil.ReadFile(inputFile)
			if err != nil {
				return w, err
			}

			pool.fileName = inputFile
			pool.sharedFile = &data
		} else if pool.fileName != inputFile {
			return w, errInvalidFile
		}
	} else {
		w.file, err = os.Open(inputFile)
		if err != nil {
			return w, err
		}
		w.reader = w.file
	}

	return w, nil
}
