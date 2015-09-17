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

func optimize(input, output string) {
	fin, err := os.Open(input)
	if err != nil {
		panic(err)
	}
	defer fin.Close()

	fout, err := os.Create(output)
	if err != nil {
		panic(err)
	}
	defer fout.Close()

	if err := OptimizeTree(fin, fout); err != nil {
		panic(err)
	}
}

func TestOptimize(t *testing.T) {
	startFilter()
	start(1, "test.bin", NewUnsortedWorker)
	optimize("test.oct", "test.tmp")
}
