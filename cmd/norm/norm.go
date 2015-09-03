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
	"os"
	"fmt"
	"bufio"
)

const (
	xOffset = -788
	yOffset = -602
	zOffset = -48
	scale = 100.0
)

func main() {
	fp, err := os.Open("pack/test.xyz")
	if err != nil {
		panic(err)
	}
	defer fp.Close()

	wfp, err := os.Create("pack/test_norm.xyz")
	if err != nil {
		panic(err)
	}
	defer wfp.Close()

	scanner := bufio.NewScanner(fp)
	for scanner.Scan() {
		var (
			x, y, z, f float64
			r, g, b byte
		)

		_, err := fmt.Sscan(scanner.Text(), &x, &y, &z, &f, &r, &g, &b)
		if err != nil {
			panic(err)
		}

		fmt.Fprintf(wfp, "%v %v %v %v %v %v %v\n", (x + xOffset) * scale, (y + yOffset) * scale, (z + zOffset) * scale, f, r, g, b)
	}

	if err := scanner.Err(); err != nil {
		panic(err)
	}
}
