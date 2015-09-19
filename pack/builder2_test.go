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
	"fmt"
	"os"
	"testing"
)

func TestBuildTree2(t *testing.T) {
	infile, err := os.Open("test.xyz")
	if err != nil {
		panic(err)
	}
	defer infile.Close()

	outfile, err := os.Create("test.oct")
	if err != nil {
		panic(err)
	}
	defer outfile.Close()

	parser := func(samples chan<- Sample) error {
		scanner := bufio.NewScanner(infile)
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

	bounds := Box{Point{0, 0, 0}, 80}
	//bounds := Box{Point{733, 682, 40.4}, 8.1}
	//bounds := Box{Point{797, 698, 41.881}, 8.5}

	err = BuildTree2(&BuildConfig{outfile, bounds, 8, MIP_R8G8B8A8_UI32, 0, false, false}, parser)
	if err != nil {
		panic(err)
	}
}
