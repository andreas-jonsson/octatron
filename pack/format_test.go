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
	"fmt"
	"testing"
)

func testDecode(format OctreeFormat, colorDiff float32) {
	var (
		colorIn           Color
		childIn, childOut [8]uint32
		buffer            bytes.Buffer
	)

	colorOut := Color{0.5, 0.3, 0.1, 0.5}
	for i := range childOut {
		childOut[i] = uint32(100*i - 10*i)
	}

	if err := EncodeNode(&buffer, format, colorOut, childOut[:]); err != nil {
		panic(err)
	}

	if err := DecodeNode(bytes.NewReader(buffer.Bytes()), format, &colorIn, childIn[:]); err != nil {
		panic(err)
	}

	if colorIn.dist(&colorOut) > colorDiff {
		panic(fmt.Errorf("%v ~= %v, %v", colorIn, colorOut, colorDiff))
	}

	for i, child := range childIn {
		if child != childOut[i] {
			panic("child != childOut[i]")
		}
	}
}

func TestDecodeNode(t *testing.T) {
	testDecode(MIP_R8G8B8A8_UI32, 0.01)
	testDecode(MIP_R8G8B8A8_UI16, 0.01)
	//testDecode(MIP_R5G5B5A1_UI16, 0.1)
}
