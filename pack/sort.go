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
	"io/ioutil"
	"math"
	"os"
	"sort"
)

type FilterConfig struct {
	Reader   io.Reader
	Writer   io.Writer
	Function func(io.Reader, chan<- Sample) error
}

type filterSample struct {
	Pos        Point
	R, G, B, A byte
}

func (s *filterSample) Color() Color {
	return Color{float32(s.R) / 256, float32(s.G) / 256, float32(s.B) / 256, float32(s.A) / 256}
}

func (s *filterSample) setColor(c Color) {
	s.R = byte(c.R * 256)
	s.G = byte(c.G * 256)
	s.B = byte(c.B * 256)
	s.A = byte(c.A * 256)
}

func (s *filterSample) Position() Point {
	return s.Pos
}

type sampleSlice []filterSample

func (s sampleSlice) Len() int {
	return len(s)
}

func (s sampleSlice) Less(i, j int) bool {
	return s[i].Pos.X < s[j].Pos.X
}

func (s sampleSlice) Swap(i, j int) {
	tmp := s[i]
	s[i] = s[j]
	s[j] = tmp
}

func FilterInput(cfg *FilterConfig) (Box, error) {
	var (
		err   error
		fsamp filterSample
		ret   Box
	)

	min := math.MaxFloat64
	max := -math.MaxFloat64
	minPos := Point{min, min, min}
	maxPos := Point{max, max, max}

	errPtr := &err
	channel := make(chan Sample, 10)

	go func() {
		*errPtr = cfg.Function(cfg.Reader, channel)
		close(channel)
	}()

	for {
		samp, more := <-channel
		if more == false {
			break
		}

		fsamp.Pos = samp.Position()
		fsamp.setColor(samp.Color())
		minMax(&minPos, &maxPos, &fsamp.Pos)

		err := binary.Write(cfg.Writer, binary.BigEndian, fsamp)
		if err != nil {
			return ret, err
		}
	}

	len := Point{maxPos.X - minPos.X, maxPos.Y - minPos.Y, maxPos.Z - minPos.Z}
	ret.Size = math.Max(math.Max(len.X, len.Y), len.Z)
	ret.Pos = minPos

	return ret, err
}

func minMax(min *Point, max *Point, pos *Point) {
	min.X = math.Min(min.X, pos.X)
	min.Y = math.Min(min.Y, pos.Y)
	min.Z = math.Min(min.Z, pos.Z)

	max.X = math.Max(max.X, pos.X)
	max.Y = math.Max(max.Y, pos.Y)
	max.Z = math.Max(max.Z, pos.Z)
}

func XSortInput(reader io.ReadSeeker, writer io.Writer, numSlices int) error {
	files, err := sortData(reader, writer, numSlices)
	if err != nil {
		return err
	}

	defer func() {
		for _, f := range files {
			os.Remove(f)
		}
	}()

	if err = mergeData(files, writer); err != nil {
		return err
	}

	return nil
}

func sortData(reader io.ReadSeeker, writer io.Writer, numSlices int) ([]string, error) {
	size, err := reader.Seek(0, 2)
	if err != nil {
		return nil, err
	}

	numNodes := size / defaultNodeSize
	for numSlices == 0 || numNodes%int64(numSlices) != 0 {
		numSlices++
	}

	numNodesInBuffer := numNodes / int64(numSlices)
	files := make([]string, numSlices)

	_, err = reader.Seek(0, 0)
	if err != nil {
		return files, err
	}

	samples := make(sampleSlice, numNodesInBuffer)

	for i, _ := range files {
		err = binary.Read(reader, binary.BigEndian, samples)
		if err != nil {
			return files, err
		}

		sort.Sort(samples)

		var file *os.File
		file, err = ioutil.TempFile("", "")
		if err != nil {
			return files, err
		}
		files[i] = file.Name()

		err = binary.Write(file, binary.BigEndian, samples)
		if err != nil {
			return files, err
		}
		file.Close()
	}

	return files, nil
}

func mergeData(files []string, writer io.Writer) error {
	var err error

	numFiles := len(files)
	fps := make([]io.ReadCloser, numFiles)
	unsorted := make([]*filterSample, numFiles)
	hasData := make([]bool, numFiles)

	for i, f := range files {
		unsorted[i] = new(filterSample)
		fps[i], err = os.Open(f)
		if err != nil {
			return err
		}
	}

	for {
		numUnsorted := 0
		for i := 0; i < numFiles; i++ {
			if unsorted[i] == nil {
				continue
			}

			numUnsorted++
			if hasData[i] == true {
				continue
			}

			err = binary.Read(fps[i], binary.BigEndian, unsorted[i])
			if err == io.EOF {
				unsorted[i] = nil
				numUnsorted--
				continue
			} else if err != nil {
				return err
			}
			hasData[i] = true
		}

		if numUnsorted == 0 {
			break
		}

		idx, err := stepMerge(writer, unsorted)
		if err != nil {
			return err
		}
		hasData[idx] = false
	}

	for _, fp := range fps {
		fp.Close()
	}
	return nil
}

func stepMerge(writer io.Writer, unsorted []*filterSample) (int, error) {
	var (
		minIdx  int
		minSamp *filterSample
	)

	for idx, sample := range unsorted {
		if sample == nil {
			continue
		}

		if minSamp == nil || minSamp.Pos.X > sample.Pos.X {
			minIdx = idx
			minSamp = sample
		}
	}

	if err := binary.Write(writer, binary.BigEndian, minSamp); err != nil {
		return minIdx, err
	}

	return minIdx, nil
}
