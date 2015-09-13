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

type OctreeFormat byte

const (
	MIP_R8G8B8A8_UI32 OctreeFormat = iota
	MIP_R8G8B8A8_UI16
	MIP_R5G5B5A1_UI16
)

func (f OctreeFormat) IndexSize() int {
	return 4
}

func (f OctreeFormat) ColorSize() int {
	return 4
}

func (f OctreeFormat) NodeSize() int {
	return f.IndexSize()*8 + f.ColorSize()
}

const (
	binaryVersion  byte = 0x0
	endianMask     byte = 0x1
	compressedMask byte = 0x2
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

func (h *OctreeHeader) BigEndian() bool {
	return h.Flags&endianMask == endianMask
}

func (h *OctreeHeader) Compressed() bool {
	return h.Flags&compressedMask == compressedMask
}
