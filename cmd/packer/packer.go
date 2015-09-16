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
	"github.com/andreas-t-jonsson/octatron/pack"

	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"io/ioutil"
	"os"
)

type writeSeekerBuffer struct {
	data   []byte
	len    int64
	offset int64
}

func NewWriteSeekerBuffer(data []byte) *writeSeekerBuffer {
	return &writeSeekerBuffer{data, 0, 0}
}

func (writer *writeSeekerBuffer) Write(p []byte) (n int, err error) {
	s := len(p)
	for i := 0; i < s; i++ {
		writer.data[writer.offset+int64(i)] = p[i]
	}
	writer.offset += int64(s)
	if writer.offset > writer.len {
		writer.len = writer.offset
	}
	return s, nil
}

func (writer *writeSeekerBuffer) Seek(offset int64, whence int) (int64, error) {
	switch whence {
	case 0:
		writer.offset = offset
	case 1:
		writer.offset += offset
	case 2:
		writer.offset = writer.len - offset
	}
	return writer.offset, nil
}

type cloudSample struct {
	pos        pack.Point
	r, g, b, a byte
}

func (s *cloudSample) Color() pack.Color {
	return pack.Color{float32(s.r) / 256, float32(s.g) / 256, float32(s.b) / 256, float32(s.a) / 256}
}

func (s *cloudSample) Position() pack.Point {
	return s.pos
}

func filter(input io.Reader, samples chan<- pack.Sample) error {
	scanner := bufio.NewScanner(input)
	for scanner.Scan() {
		s := new(cloudSample)

		var ref float64
		_, err := fmt.Sscan(scanner.Text(), &s.pos.X, &s.pos.Y, &s.pos.Z, &ref, &s.r, &s.g, &s.b)
		if err != nil {
			return err
		}

		samples <- s
	}

	return scanner.Err()
}

func startFilter(input, output string) {
	in, err := os.Open(input)
	if err != nil {
		panic(err)
	}
	defer in.Close()

	out, err := os.Create(output)
	if err != nil {
		panic(err)
	}
	defer out.Close()

	bounds, err := pack.FilterInput(in, out, filter)
	if err != nil {
		panic(err)
	}

	fmt.Println("Bounding box:", bounds)
}

func startSort(input, output string) {
	in, _ := os.Open(input)
	defer in.Close()

	out, _ := os.Create(output)
	defer out.Close()

	// This reads the entire file in to memory!
	// Use ExternalSortInput if the file is to big.
	if err := pack.SortInput(in, out); err != nil {
		panic(err)
	}
}

const (
	inMemoryRead  = true
	inMemoryWrite = true
)

func startBuild(numWorkers int, input, output string) {
	workers := make([]pack.Worker, numWorkers)

	var (
		err       error
		data      []byte
		inputSize int
		reader    io.ReadSeeker
	)

	if inMemoryRead == true {
		// This reads the entire file in to memory.
		data, err = ioutil.ReadFile(input)
		if err != nil {
			panic(err)
		}
		inputSize = len(data)
	}

	for i := range workers {
		if inMemoryRead == true {
			reader = bytes.NewReader(data)
		} else {
			fp, err := os.Open(input)
			if err != nil {
				panic(err)
			}
			defer fp.Close()

			if inputSize <= 0 {
				offset, _ := fp.Seek(0, 2)
				fp.Seek(0, 0)
				inputSize = int(offset)
			}
			reader = fp
		}

		workers[i], err = pack.NewSortedWorker(reader)
		if err != nil {
			panic(err)
		}
	}

	file, err := os.Create(output)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	var (
		wsb    *writeSeekerBuffer
		writer io.WriteSeeker
	)

	if inMemoryWrite == true {
		// Assume input is less or equal in size to output.
		wsb = NewWriteSeekerBuffer(make([]byte, inputSize))
		writer = wsb
	} else {
		writer = file
	}

	//bounds := pack.Box{pack.Point{733, 682, 40.4}, 8.1}
	bounds := pack.Box{pack.Point{797, 698, 41.881}, 8.5}
	err = pack.BuildTree(workers, &pack.BuildConfig{writer, bounds, 128, pack.MIP_R8G8B8A8_UI32, 0, false, true})
	if err != nil {
		panic(err)
	}

	if inMemoryWrite == true {
		err = binary.Write(file, binary.BigEndian, wsb.data[:wsb.len])
		if err != nil {
			panic(err)
		}
	}
}

func Start() {
	startFilter("test.priv.xyz", "test.priv.bin")
	startSort("test.priv.bin", "test.priv.ord")
	startBuild(4, "test.priv.ord", "test.priv.oct")
}

func main() {
	Start()
}
