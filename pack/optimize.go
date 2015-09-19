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
	"io/ioutil"
	"os"
)

const leafThreshold = 0 // Should perhaps move this to be controlled by the user.

type OptStatus struct {
	NumMerged uint32
	MemMap    []int64
}

func OptimizeTree(reader io.ReadSeeker, writer io.Writer, outputFormat OctreeFormat, colorThreshold float32) (OptStatus, error) {
	var (
		header OctreeHeader
		status OptStatus
	)

	if err := binary.Read(reader, binary.BigEndian, &header); err != nil {
		return status, err
	}

	if header.Compressed() == true {
		return status, errUnsupportedFormat
	}

	maxLevels := 0
	for i := 1; i <= int(header.VoxelsPerAxis); i *= 2 {
		maxLevels++
	}

	status.MemMap = make([]int64, maxLevels)
	tempFiles := make([]*os.File, maxLevels)

	for i := range tempFiles {
		fp, err := ioutil.TempFile("", "")
		if err != nil {
			return status, err
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

	_, err := optNode(reader, tempFiles, &header, outputFormat, 0, 0, colorThreshold, &status)
	if err != nil {
		return status, err
	}

	header.Format = outputFormat
	if err := binary.Write(writer, binary.BigEndian, header); err != nil {
		return status, err
	}

	err = mergeAndPatch(writer, tempFiles, &header, &status)
	if err != nil {
		return status, err
	}

	return status, err
}

func mergeAndPatch(writer io.Writer, files []*os.File, header *OctreeHeader, status *OptStatus) error {
	var numNodes int64
	for lv, fp := range files {
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

		nodeSize := int64(header.Format.NodeSize())
		numNodesInFile := end / nodeSize
		nextLevelStart := numNodes + numNodesInFile

		for i := int64(0); i < numNodesInFile; i++ {
			if err := DecodeNode(fp, header.Format, &color, children[:]); err != nil {
				return err
			}

			for j, child := range children {
				if child == 0xffff {
					children[j] = 0
				} else {
					children[j] = uint32(nextLevelStart) + child
				}
			}

			if err := EncodeNode(writer, header.Format, color, children[:]); err != nil {
				return err
			}
		}

		status.MemMap[lv] = numNodes*nodeSize + int64(header.Size())
		numNodes += numNodesInFile
	}
	return nil
}

func optNode(reader io.ReadSeeker, files []*os.File, header *OctreeHeader, outputFormat OctreeFormat, nodeIndex, level uint32, colorThreshold float32, status *OptStatus) (int64, error) {
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

	merge := true
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

			if color.dist(&childColor) > colorThreshold {
				merge = false
				break
			}

			leafs := 0
			for _, gc := range grandChildren {
				if gc > 0 {
					leafs++
				}
			}

			if leafs > leafThreshold {
				merge = false
				break
			}
		} else {
			merge = false
			break
		}
	}

	numChildren := 0
	if merge == false {
		for i, child := range children {
			if child > 0 {
				p, err := optNode(reader, files, header, outputFormat, child, level+1, colorThreshold, status)
				if err != nil {
					return 0, err
				}
				children[i] = uint32(p)
				numChildren++
			} else {
				children[i] = 0xffff
			}
		}
	} else {
		status.NumMerged++
		for i := range children {
			children[i] = 0xffff
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

	if err := EncodeNode(fp, outputFormat, color, children[:]); err != nil {
		return 0, err
	}

	return pos / int64(nodeSize), nil
}
