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
	"fmt"
	"io"
	"io/ioutil"
	"os"
)

func OptimizeTree(reader io.ReadSeeker, writer io.Writer) error {
	var header OctreeHeader
	if err := binary.Read(reader, binary.BigEndian, &header); err != nil {
		return err
	}

	if header.Compressed() == true {
		return errUnsupportedFormat
	}

	if header.Optimized() == true {
		return errInvalidFile
	}

	fmt.Println("aaaa", header.NumNodes)

	maxLevels := 0
	for i := 1; i <= int(header.VoxelsPerAxis); i *= 2 {
		maxLevels++
	}

	tempFiles := make([]*os.File, maxLevels)
	for i := range tempFiles {
		fp, err := ioutil.TempFile("", "")
		if err != nil {
			return nil
		}
		tempFiles[i] = fp
	}
	defer func() {
		for _, fp := range tempFiles {
			name := fp.Name()
			fp.Close()
			os.Remove(name)
		}
	}()

	header.NumLeafs = 0
	header.NumNodes = 0
	header.Flags &= optimizedMask

	_, err := optNode(reader, tempFiles, &header, 0, 0)
	if err != nil {
		return err
	}

	if err := binary.Write(writer, binary.BigEndian, header); err != nil {
		return err
	}

	err = mergeAndPatch(writer, tempFiles, &header)
	if err != nil {
		return err
	}

	return err
}

func mergeAndPatch(writer io.Writer, files []*os.File, header *OctreeHeader) error {
	var numNodes int64
	for _, fp := range files {
		var (
			color    Color
			children [8]uint32
		)

		end, err := fp.Seek(0, 2)
		if err != nil {
			return nil
		}

		_, err = fp.Seek(0, 0)
		if err != nil {
			return nil
		}

		numNodesInFile := end / int64(header.Format.NodeSize())
		nextLevelStart := numNodes + numNodesInFile

		for i := int64(0); i < numNodesInFile; i++ {
			if err := DecodeNode(fp, header.Format, &color, children[:]); err != nil {
				return err
			}

			for j, child := range children {
				if child > 0 {
					children[j] = uint32(nextLevelStart) + child
				}
			}

			if err := EncodeNode(writer, header.Format, color, children[:]); err != nil {
				return err
			}
		}

		numNodes += numNodesInFile
	}
	return nil
}

func optNode(reader io.ReadSeeker, files []*os.File, header *OctreeHeader, nodeIndex, level uint32) (int64, error) {
	var (
		color    Color
		children [8]uint32
	)

	nodeSize := header.Format.NodeSize()
	headerSize := uint32(header.Size())

	if _, err := reader.Seek(int64(nodeIndex*uint32(nodeSize)+headerSize), 0); err != nil {
		return 0, err
	}

	if err := DecodeNode(reader, header.Format, &color, children[:]); err != nil {
		return 0, err
	}

	var (
		leafs int
		merge = true
	)

	for _, child := range children {
		if child > 0 {
			if _, err := reader.Seek(int64(child*uint32(nodeSize)+headerSize), 0); err != nil {
				return 0, err
			}

			var (
				childColor    Color
				grandChildren [8]uint32
			)

			if err := DecodeNode(reader, header.Format, &childColor, grandChildren[:]); err != nil {
				return 0, err
			}

			if color.dist(&childColor) > 0.5 {
				merge = false
				break
			}
		} else {
			if leafs++; leafs > 0 {
				merge = false
				break
			}
		}
	}
	merge = false
	if merge == true {
		for i := range children {
			children[i] = 0
		}
	}

	numChildren := 0
	if merge == false {
		for i, child := range children {
			if child > 0 {
				p, err := optNode(reader, files, header, child, level+1)
				if err != nil {
					return 0, err
				}
				children[i] = uint32(p)
				numChildren++
			}
		}
	}

	fp := files[level]
	pos, err := fp.Seek(0, 1)
	if err != nil {
		return 0, err
	}

	header.NumNodes++
	if numChildren == 0 {
		header.NumLeafs++
	}

	if err := EncodeNode(fp, header.Format, color, children[:]); err != nil {
		return 0, err
	}

	return pos / int64(nodeSize), nil
}
