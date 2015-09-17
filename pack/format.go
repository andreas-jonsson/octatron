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
)

type OctreeFormat byte

const (
	MIP_R8G8B8A8_UI32 OctreeFormat = iota
	MIP_R8G8B8A8_UI16
	MIP_R5G5B5A1_UI16
)

var (
	formatIndexSize = [3]int{4, 2, 2}
	formatColorSize = [3]int{4, 4, 2}
)

func (f OctreeFormat) IndexSize() int {
	return formatIndexSize[f]
}

func (f OctreeFormat) ColorSize() int {
	return formatColorSize[f]
}

func (f OctreeFormat) NodeSize() int {
	return formatColorSize[f] + formatIndexSize[f]*8
}

const (
	binaryVersion  byte = 0x0
	endianMask     byte = 0x1
	compressedMask byte = 0x2
	optimizedMask  byte = 0x4
)

type OctreeHeader struct {
	Sign          [4]byte
	Version       byte
	Format        OctreeFormat
	Flags         byte
	Unused        byte
	NumNodes      uint64
	NumLeafs      uint64
	VoxelsPerAxis uint32
}

func (h *OctreeHeader) Size() int {
	return 28
}

func (h *OctreeHeader) BigEndian() bool {
	return h.Flags&endianMask == endianMask
}

func (h *OctreeHeader) Compressed() bool {
	return h.Flags&compressedMask == compressedMask
}

func (h *OctreeHeader) Optimized() bool {
	return h.Flags&optimizedMask == optimizedMask
}

func DecodeNode(reader io.Reader, fmt OctreeFormat, color *Color, children []uint32) error {
	readR8G8B8A8 := func() error {
		var col [4]byte
		if err := binary.Read(reader, binary.BigEndian, &col); err != nil {
			return err
		}

		color.R = float32(col[0]) / 256
		color.G = float32(col[1]) / 256
		color.B = float32(col[2]) / 256
		color.A = float32(col[3]) / 256
		return nil
	}

	readChild16 := func() error {
		var ch [8]uint16
		if err := binary.Read(reader, binary.BigEndian, &ch); err != nil {
			return err
		}

		for i := 0; i < 8; i++ {
			children[i] = uint32(ch[i])
		}
		return nil
	}

	if fmt == MIP_R8G8B8A8_UI32 {
		if err := readR8G8B8A8(); err != nil {
			return err
		}

		if err := binary.Read(reader, binary.BigEndian, children); err != nil {
			return err
		}
	} else if fmt == MIP_R8G8B8A8_UI16 {
		if err := readR8G8B8A8(); err != nil {
			return err
		}

		if err := readChild16(); err != nil {
			return err
		}
	} else if fmt == MIP_R5G5B5A1_UI16 {
		var col uint16
		if err := binary.Read(reader, binary.BigEndian, &col); err != nil {
			return err
		}

		color.R = float32(col & 0xf800)
		color.G = float32(col & 0x7c0)
		color.B = float32(col & 0x3e)
		color.A = float32(col & 0x1)

		if err := readChild16(); err != nil {
			return err
		}
	} else {
		return errUnsupportedFormat
	}
	return nil
}

func EncodeNode(writer io.Writer, fmt OctreeFormat, color Color, children []uint32) error {
	if err := color.writeColor(writer, fmt); err != nil {
		return err
	}

	if fmt == MIP_R8G8B8A8_UI32 {
		if err := binary.Write(writer, binary.BigEndian, children); err != nil {
			return err
		}
	} else {
		var ch [8]uint16
		for i := 0; i < 8; i++ {
			ch[i] = uint16(children[i])
		}

		if err := binary.Write(writer, binary.BigEndian, ch); err != nil {
			return err
		}
	}
	return nil
}
