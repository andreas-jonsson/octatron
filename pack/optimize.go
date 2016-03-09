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
	"io/ioutil"
	"math"
	"os"
)

const leafThreshold = 0 // Should perhaps move this to be controlled by the user.

type OptStatus struct {
	NumMerged uint32
	MemMap    []int64
}

type optInput struct {
	reader         io.ReadSeeker
	files          []*os.File
	header         *OctreeHeader
	colorThreshold float32
	colorFilter    bool
	status         *OptStatus
}

func CompressTree(reader io.Reader, writer io.Writer) error {
	var header OctreeHeader
	err := binary.Read(reader, binary.LittleEndian, &header)
	if err != nil {
		return err
	}

	if header.Compressed() == true {
		return errInputIsCompressed
	}
	header.Flags = header.Flags & compressedMask

	err = binary.Write(writer, binary.LittleEndian, header)
	if err != nil {
		return err
	}

	zip := zlib.NewWriter(writer)
	defer zip.Close()

	var (
		color    Color
		children [8]uint32
	)

	for i := uint64(0); i < header.NumNodes; i++ {
		if err := DecodeNode(reader, header.Format, &color, children[:]); err != nil {
			return err
		}

		if err := EncodeNode(zip, header.Format, color, children[:]); err != nil {
			return err
		}
	}

	return nil
}

func OptimizeTree(reader io.ReadSeeker, writer io.Writer, outputFormat OctreeFormat, colorThreshold float32, colorFilter bool) (OptStatus, error) {
	var (
		header OctreeHeader
		status OptStatus
	)

	if err := binary.Read(reader, binary.LittleEndian, &header); err != nil {
		return status, err
	}

	if header.Compressed() == true {
		return status, errInputIsCompressed
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

	args := optInput{reader, tempFiles, &header, colorThreshold, colorFilter, &status}
	_, err := optNode(&args, 0, 0, Color{})
	if err != nil {
		return status, err
	}

	header.Format = outputFormat
	if err := binary.Write(writer, binary.LittleEndian, header); err != nil {
		return status, err
	}

	header.Format = MipR8G8B8A8UnpackUI32
	err = mergeAndPatch(writer, tempFiles, &header, outputFormat, &status)
	if err != nil {
		return status, err
	}

	return status, err
}

func mergeAndPatch(writer io.Writer, files []*os.File, header *OctreeHeader, outputFormat OctreeFormat, status *OptStatus) error {
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
				if child == math.MaxUint32 {
					children[j] = 0
				} else {
					children[j] = uint32(nextLevelStart) + child
				}
			}

			if err := EncodeNode(writer, outputFormat, color, children[:]); err != nil {
				return err
			}
		}

		status.MemMap[lv] = numNodes*nodeSize + int64(header.Size())
		numNodes += numNodesInFile
	}
	return nil
}

func optNode(in *optInput, nodeIndex, level uint32, parentColor Color) (int64, error) {
	var (
		color    Color
		children [8]uint32
	)

	nodeSize := in.header.Format.NodeSize()
	headerSize := uint32(in.header.Size())

	if _, err := in.reader.Seek(int64(nodeIndex*uint32(nodeSize)+headerSize), 0); err != nil {
		return 0, err
	}

	if err := DecodeNode(in.reader, in.header.Format, &color, children[:]); err != nil {
		return 0, err
	}

	merge := true
	for _, child := range children {
		if child > 0 {
			if _, err := in.reader.Seek(int64(child*uint32(nodeSize)+headerSize), 0); err != nil {
				return 0, err
			}

			var (
				childColor    Color
				grandChildren [8]uint32
			)

			if err := DecodeNode(in.reader, in.header.Format, &childColor, grandChildren[:]); err != nil {
				return 0, err
			}

			if color.dist(&childColor) > in.colorThreshold {
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
				p, err := optNode(in, child, level+1, color)
				if err != nil {
					return 0, err
				}
				children[i] = uint32(p)
				numChildren++
			} else {
				children[i] = math.MaxUint32
			}
		}
	} else {
		in.status.NumMerged++
		for i := range children {
			children[i] = math.MaxUint32
		}
	}

	fp := in.files[level]
	pos, err := fp.Seek(0, 1)
	if err != nil {
		return 0, err
	}

	newColor := color
	in.header.NumNodes++
	if numChildren == 0 {
		in.header.NumLeafs++
		if in.colorFilter == true {
			newColor = parentColor
		}
	}

	if err := EncodeNode(fp, MipR8G8B8A8UnpackUI32, newColor, children[:]); err != nil {
		return 0, err
	}

	return pos / int64(MipR8G8B8A8UnpackUI32.NodeSize()), nil
}
