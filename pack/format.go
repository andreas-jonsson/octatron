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
	"compress/zlib"
	"encoding/binary"
	"io"
	"math"
)

type OctreeFormat byte

const (
	MipR8G8B8A8UnpackUI32 OctreeFormat = iota
	MipR8G8B8A8UnpackUI16
	MipR4G4B4A4UnpackUI16
	MipR5G6B5UnpackUI16

	MipR8G8B8A8PackUI28
	MipR4G4B4A4PackUI30
	MipR5G6B5PackUI30
	MipR3G3B2PackUI31

	// Internal formats
	mipR64G64B64A64S64UnpackUI32
)

const (
	maxUint31 = 1<<31 - 1
	maxUint30 = 1<<30 - 1
	maxUint28 = 1<<28 - 1
)

var (
	formatColorSize = [...]int{4, 4, 2, 2, 0, 0, 0, 0, 40}
	formatIndexSize = [...]int{4, 2, 2, 2, 4, 4, 4, 4, 4}
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

func TranscodeTree(reader io.Reader, writer io.Writer, format OctreeFormat) error {
	var (
		header   OctreeHeader
		color    Color
		children [8]uint32
	)

	if err := binary.Read(reader, binary.BigEndian, &header); err != nil {
		return err
	}

	inputFormat := header.Format
	header.Format = format

	if err := binary.Write(writer, binary.BigEndian, header); err != nil {
		return err
	}

	if header.Compressed() == true {
		readCloser, err := zlib.NewReader(reader)
		if err != nil {
			return err
		}
		defer readCloser.Close()
		reader = readCloser

		writeCloser := zlib.NewWriter(writer)
		defer writeCloser.Close()
		writer = writeCloser
	}

	for i := uint64(0); i < header.NumNodes; i++ {
		if err := DecodeNode(reader, inputFormat, &color, children[:]); err != nil {
			return err
		}

		if err := EncodeNode(writer, format, color, children[:]); err != nil {
			return err
		}
	}

	return nil
}

func DecodeHeader(reader io.Reader, header *OctreeHeader) error {
	return binary.Read(reader, binary.BigEndian, header)
}

func EncodeHeader(writer io.Writer, header OctreeHeader) error {
	return binary.Write(writer, binary.BigEndian, header)
}

func DecodeNode(reader io.Reader, format OctreeFormat, color *Color, children []uint32) error {
	readR8G8B8A8 := func() error {
		var col [4]byte
		if err := binary.Read(reader, binary.BigEndian, &col); err != nil {
			return err
		}

		color.R = float32(col[0]) / 255
		color.G = float32(col[1]) / 255
		color.B = float32(col[2]) / 255
		color.A = float32(col[3]) / 255
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

	if format == MipR8G8B8A8UnpackUI32 {
		if err := readR8G8B8A8(); err != nil {
			return err
		}

		if err := binary.Read(reader, binary.BigEndian, children); err != nil {
			return err
		}
	} else if format == MipR8G8B8A8UnpackUI16 {
		if err := readR8G8B8A8(); err != nil {
			return err
		}

		if err := readChild16(); err != nil {
			return err
		}
	} else if format == MipR4G4B4A4UnpackUI16 {
		var col uint16
		if err := binary.Read(reader, binary.BigEndian, &col); err != nil {
			return err
		}

		color.R = float32((col&0xf000)>>12) / 15
		color.G = float32((col&0xf00)>>8) / 15
		color.B = float32((col&0xf0)>>4) / 15
		color.A = float32(col&0xf) / 15

		if err := readChild16(); err != nil {
			return err
		}
	} else if format == MipR5G6B5UnpackUI16 {
		var col uint16
		if err := binary.Read(reader, binary.BigEndian, &col); err != nil {
			return err
		}

		color.R = float32((col&0xf800)>>11) / 31
		color.G = float32((col&0x7e0)>>5) / 63
		color.B = float32(col&0x1f) / 31
		color.A = 1

		if err := readChild16(); err != nil {
			return err
		}
	} else if format == mipR64G64B64A64S64UnpackUI32 {
		var col [5]uint64
		if err := binary.Read(reader, binary.BigEndian, &col); err != nil {
			return err
		}

		color.R = float32((col[0] / col[4])) / 255
		color.G = float32((col[1] / col[4])) / 255
		color.B = float32((col[2] / col[4])) / 255
		color.A = float32((col[3] / col[4])) / 255

		if err := binary.Read(reader, binary.BigEndian, children); err != nil {
			return err
		}
	} else if format == MipR8G8B8A8PackUI28 {
		if err := binary.Read(reader, binary.BigEndian, children); err != nil {
			return err
		}

		var cbits byte
		for i, component := range children {
			if i%2 == 0 {
				cbits = byte(component >> 24)
			} else {
				cbits |= byte(component >> 28)
				color.setComponent(i/2, float32(cbits)/255)
			}
			children[i] = component & 0xfffffff
		}
	} else if format == MipR4G4B4A4PackUI30 {
		if err := binary.Read(reader, binary.BigEndian, children); err != nil {
			return err
		}

		var cbits uint16
		for i, component := range children {
			cbits |= uint16((component & 0xc0000000) >> (16 + byte(i*2)))
			children[i] = component & 0x3fffffff
		}

		color.R = float32((cbits&0xf000)>>12) / 15
		color.G = float32((cbits&0xf00)>>8) / 15
		color.B = float32((cbits&0xf0)>>4) / 15
		color.A = float32(cbits&0xf) / 15
	} else if format == MipR5G6B5PackUI30 {
		if err := binary.Read(reader, binary.BigEndian, children); err != nil {
			return err
		}

		var cbits uint16
		for i, component := range children {
			cbits |= uint16((component & 0xc0000000) >> (16 + byte(i*2)))
			children[i] = component & 0x3fffffff
		}

		color.R = float32((cbits&0xf800)>>11) / 31
		color.G = float32((cbits&0x7e0)>>5) / 63
		color.B = float32(cbits&0x1f) / 31
		color.A = 1
	} else if format == MipR3G3B2PackUI31 {
		if err := binary.Read(reader, binary.BigEndian, children); err != nil {
			return err
		}

		var cbits byte
		for i, component := range children {
			cbits |= byte((component & 0x80000000) >> (24 + byte(i)))
			children[i] = component & 0x7fffffff
		}

		color.R = float32((cbits&0xe0)>>5) / 7
		color.G = float32((cbits&0x1c)>>2) / 7
		color.B = float32(cbits&0x3) / 3
		color.A = 1
	} else {
		return errUnsupportedFormat
	}
	return nil
}

func EncodeNode(writer io.Writer, format OctreeFormat, color Color, children []uint32) error {
	if format == MipR8G8B8A8UnpackUI32 {
		if err := color.writeColor(writer, format); err != nil {
			return err
		}

		for _, child := range children {
			if child > math.MaxUint32 {
				return errOctreeOverflow
			}
		}

		if err := binary.Write(writer, binary.BigEndian, children); err != nil {
			return err
		}
	} else if format == MipR8G8B8A8PackUI28 {
		var component uint32
		colors := color.bytes()

		for i, child := range children {
			if child > maxUint28 {
				return errOctreeOverflow
			}

			var colorNib uint32
			if i%2 == 0 {
				colorNib = uint32(colors[i/2]&0xf0) << 24
			} else {
				colorNib = uint32(colors[i/2]&0xf) << 28
			}

			component = colorNib | child
			if err := binary.Write(writer, binary.BigEndian, component); err != nil {
				return err
			}
		}
	} else if format == MipR3G3B2PackUI31 {
		var (
			component   uint32
			packedColor byte
		)

		packedColor = byte(color.R*7) << 5
		packedColor |= byte(color.G*7) << 2
		packedColor |= byte(color.B * 3)

		for i, child := range children {
			if child > maxUint31 {
				return errOctreeOverflow
			}

			component = ((uint32(packedColor) << byte(24+i)) & 0x80000000) | child
			if err := binary.Write(writer, binary.BigEndian, component); err != nil {
				return err
			}
		}
	} else if format == MipR5G6B5PackUI30 {
		var (
			component   uint32
			packedColor uint16
		)

		packedColor = uint16(color.R*31) << 11
		packedColor |= uint16(color.G*63) << 5
		packedColor |= uint16(color.B * 31)

		for i, child := range children {
			if child > maxUint30 {
				return errOctreeOverflow
			}

			component = ((uint32(packedColor) << byte(16+i*2)) & 0xc0000000) | child
			if err := binary.Write(writer, binary.BigEndian, component); err != nil {
				return err
			}
		}
	} else if format == MipR4G4B4A4PackUI30 {
		var (
			component   uint32
			packedColor uint16
		)

		packedColor = uint16(color.R*15) << 12
		packedColor |= uint16(color.G*15) << 8
		packedColor |= uint16(color.B*15) << 4
		packedColor |= uint16(color.A*15) & 0xf

		for i, child := range children {
			if child > maxUint30 {
				return errOctreeOverflow
			}

			component = ((uint32(packedColor) << byte(16+i*2)) & 0xc0000000) | child
			if err := binary.Write(writer, binary.BigEndian, component); err != nil {
				return err
			}
		}
	} else {
		if err := color.writeColor(writer, format); err != nil {
			return err
		}

		var ch [8]uint16
		for i, child := range children {
			if child > math.MaxUint16 {
				return errOctreeOverflow
			}
			ch[i] = uint16(child)
		}

		if err := binary.Write(writer, binary.BigEndian, ch); err != nil {
			return err
		}
	}
	return nil
}
