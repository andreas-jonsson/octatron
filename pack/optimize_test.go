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

func TestTranscode(t *testing.T) {
	TestBuildTree(t)

	in, _ := os.Open("test.oct")
	defer in.Close()

	out, _ := os.Create("test.opt")
	defer out.Close()

	if err := TranscodeTree(in, out, MIP_R8G8B8A8_UI32); err != nil {
		panic(err)
	}
}

func TestCompress(t *testing.T) {
	TestBuildTree(t)

	in, _ := os.Open("test.oct")
	defer in.Close()

	out, _ := os.Create("test.ocz")
	defer out.Close()

	if err := CompressTree(in, out); err != nil {
		panic(err)
	}
}
