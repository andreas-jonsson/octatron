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

type accNode struct {
	Color    [5]uint64
	Children [8]uint32
}

func BuildTree2(cfg *BuildConfig, worker func(chan<- Sample) error) error {
	fp, err := ioutil.TempFile("", "")
	if err != nil {
		return err
	}

	fileName := fp.Name()
	defer os.Remove(fileName)
	defer fp.Close()

	var cbErr error
	errPtr := &cbErr
	channel := make(chan Sample, 10)

	go func() {
		*errPtr = worker(channel)
		close(channel)
	}()

	header, err := writeOctreeHeader(cfg, fp)
	if err != nil {
		return err
	}

	header.NumNodes++
	var rootNode accNode
	if err := binary.Write(fp, binary.BigEndian, rootNode); err != nil {
		return err
	}

	for {
		samp, more := <-channel
		if more == false {
			break
		}

		if _, err := fp.Seek(int64(header.Size()), 0); err != nil {
			return err
		}

		if err := insertSample(cfg, header, fp, samp, cfg.Bounds, cfg.VoxelsPerAxis); err != nil {
			return err
		}
	}

	if cbErr != nil {
		return cbErr
	}

	if _, err := fp.Seek(0, 0); err != nil {
		return err
	}

	if err := binary.Write(fp, binary.BigEndian, header); err != nil {
		return err
	}

	if _, err := fp.Seek(0, 0); err != nil {
		return err
	}

	if cfg.Interactive == true {
		status, err := OptimizeTree(fp, cfg.Writer, cfg.Format, 0.0)
		if err != nil {
			return err
		}
		fmt.Println(status)
	} else {
		if err := TranscodeTree(fp, cfg.Writer, cfg.Format); err != nil {
			return err
		}
	}

	return nil
}

func writeOctreeHeader(cfg *BuildConfig, writer io.Writer) (*OctreeHeader, error) {
	var header OctreeHeader
	header.Sign[0] = 0x1b
	header.Sign[1] = 0x6f
	header.Sign[2] = 0x63
	header.Sign[3] = 0x74
	header.Version = binaryVersion
	header.Format = MIP_R64G64B64A64S64_UI32
	header.Unused = 0x0
	header.NumNodes = 0
	header.NumLeafs = 0
	header.VoxelsPerAxis = uint32(cfg.VoxelsPerAxis)
	return &header, binary.Write(writer, binary.BigEndian, header)
}

var childPositions = []Point{
	Point{0, 0, 0}, Point{1, 0, 0}, Point{0, 1, 0}, Point{1, 1, 0},
	Point{0, 0, 1}, Point{1, 0, 1}, Point{0, 1, 1}, Point{1, 1, 1},
}

func insertSample(cfg *BuildConfig, header *OctreeHeader, readWriter io.ReadWriteSeeker, sample Sample, bounds Box, voxelRes int) error {
	var node accNode
	for {
		if err := binary.Read(readWriter, binary.BigEndian, &node); err != nil {
			return err
		}

		if _, err := readWriter.Seek(int64(-MIP_R64G64B64A64S64_UI32.NodeSize()), 1); err != nil {
			return err
		}

		color := sample.Color()
		node.Color[0] += uint64(color.R * 256)
		node.Color[1] += uint64(color.G * 256)
		node.Color[2] += uint64(color.B * 256)
		node.Color[3] += uint64(color.A * 256)
		node.Color[4]++

		if err := binary.Write(readWriter, binary.BigEndian, node.Color); err != nil {
			return err
		}

		if voxelRes == 1 {
			header.NumLeafs++
			return nil
		}

		var (
			childBounds Box
			newVoxelRes = voxelRes
		)

		for i, child := range node.Children {
			childBounds.Size = bounds.Size * 0.5
			childOffset := childPositions[i].scale(childBounds.Size)
			childBounds.Pos = bounds.Pos.add(&childOffset)

			if childBounds.Intersect(sample.Position()) == true {
				if child == 0 {
					currentPos, err := readWriter.Seek(0, 1)
					if err != nil {
						return err
					}

					newPos, err := readWriter.Seek(0, 2)
					if err != nil {
						return err
					}

					if _, err = readWriter.Seek(currentPos, 0); err != nil {
						return err
					}

					node.Children[i] = uint32((newPos - int64(header.Size())) / int64(MIP_R64G64B64A64S64_UI32.NodeSize()))
					if err := binary.Write(readWriter, binary.BigEndian, node.Children); err != nil {
						return err
					}

					if _, err = readWriter.Seek(newPos, 0); err != nil {
						return err
					}

					header.NumNodes++
					var newNode accNode
					if err := binary.Write(readWriter, binary.BigEndian, newNode); err != nil {
						return err
					}

					if _, err := readWriter.Seek(int64(-MIP_R64G64B64A64S64_UI32.NodeSize()), 1); err != nil {
						return err
					}
				} else {
					if _, err := readWriter.Seek(int64(int(child)*MIP_R64G64B64A64S64_UI32.NodeSize()+header.Size()), 0); err != nil {
						return err
					}
				}

				newVoxelRes = voxelRes / 2
				break
			}
		}

		if newVoxelRes == voxelRes {
			return nil
		} else {
			bounds = childBounds
			voxelRes = newVoxelRes
		}
	}
}
