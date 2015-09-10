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
	"os"
	"testing"
)

func start(numWorkers int, input string, constructor func(string) (Worker, error)) {
	workers := make([]Worker, numWorkers)
	for i := range workers {
		var err error
		workers[i], err = constructor(input)
		if err != nil {
			panic(err)
		}
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

	_, err = file.Seek(0, 0)
	if err != nil {
		panic(err)
	}

	zip, err := os.Create("test.ocz")
	if err != nil {
		panic(err)
	}
	defer zip.Close()

	err = CompressTree(file, zip)
	if err != nil {
		panic(err)
	}
}

func TestXSortedWorker(t *testing.T) {
	startFilter()
	startSort()
	start(4, "test.ord", NewXSortedWorker)
}

func TestUnsortedWorker(t *testing.T) {
	startFilter()
	start(4, "test.bin", NewUnsortedWorker)
}
