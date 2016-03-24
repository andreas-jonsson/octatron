/*
Copyright (C) 2015-2016 Andreas T Jonsson

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

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

	colorOut := Color{0.5, 0.3, 0.7, 1.0}
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
	testDecode(MipR8G8B8A8UnpackUI32, 0.01)
	testDecode(MipR8G8B8A8UnpackUI16, 0.01)
	testDecode(MipR4G4B4A4UnpackUI16, 0.1)
	testDecode(MipR5G6B5UnpackUI16, 0.1)

	testDecode(MipR8G8B8A8PackUI28, 0.01)
	testDecode(MipR4G4B4A4PackUI30, 0.1)
	testDecode(MipR5G6B5PackUI30, 0.1)
	testDecode(MipR3G3B2PackUI31, 0.1)
}
