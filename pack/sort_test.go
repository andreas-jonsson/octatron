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
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"sort"
	"testing"
)

func filter(input io.Reader, samples chan<- Sample) error {
	scanner := bufio.NewScanner(input)
	for scanner.Scan() {
		s := new(filterSample)

		//var ref float64
		_, err := fmt.Sscan(scanner.Text(), &s.Pos.X, &s.Pos.Y, &s.Pos.Z, &s.R, &s.G, &s.B)
		if err != nil {
			return err
		}

		samples <- s
	}

	return scanner.Err()
}

func startFilter() {
	in, _ := os.Open("test.xyz")
	defer in.Close()

	out, _ := os.Create("test.bin")
	defer out.Close()

	bounds, err := FilterInput(in, out, filter)
	if err != nil {
		panic(err)
	}

	fmt.Println("Bounding box:", bounds)
}

func startSort() {
	in, _ := os.Open("test.bin")
	defer in.Close()

	out, _ := os.Create("test.ord")
	defer out.Close()

	if err := SortInput(in, out); err != nil {
		panic(err)
	}
}

func startExternalSort() {
	in, _ := os.Open("test.bin")
	defer in.Close()

	out, _ := os.Create("test.external.ord")
	defer out.Close()

	if err := ExternalSortInput(in, out, 5); err != nil {
		panic(err)
	}
}

func verifySort(file string) {
	fp, err := os.Open(file)
	if err != nil {
		panic(err)
	}
	defer fp.Close()

	size, err := fp.Seek(0, 2)
	if err != nil {
		panic(err)
	}

	numNodes := size / defaultNodeSize
	_, err = fp.Seek(0, 0)
	if err != nil {
		panic(err)
	}

	samples := make(sampleSlice, numNodes)
	err = binary.Read(fp, binary.BigEndian, samples)
	if err != nil {
		panic(err)
	}

	if sort.IsSorted(samples) == false {
		panic("data was not sorted")
	}
}

func TestFilter(t *testing.T) {
	startFilter()
	startSort()
	verifySort("test.ord")
	startExternalSort()
	verifySort("test.external.ord")
}
