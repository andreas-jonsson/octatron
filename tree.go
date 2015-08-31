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

package octatron

import (
	"encoding/binary"
	"io"
	"runtime"
	"sync"
)

type Color struct {
	R, G, B, A float32
}

func (color *Color) writeColor(writer io.Writer, format OctreeFormat) (int, error) {
	switch format {
	case Mip_R8G8B8_Branch32:
		err := binary.Write(writer, binary.BigEndian, byte(color.R))
		err = binary.Write(writer, binary.BigEndian, byte(color.G))
		err = binary.Write(writer, binary.BigEndian, byte(color.B))
		return 3, err
	default:
		return 0, unsupportedFormatError
	}
}

type Point struct {
	X, Y, Z float64
}

type Box struct {
	Pos  Point
	Size float64
}

func (b Box) Intersect(p Point) bool {
	max := Point{b.Pos.X + b.Size, b.Pos.Y + b.Size, b.Pos.Z + b.Size}
	if b.Pos.X < p.X && b.Pos.Y < p.Y && b.Pos.Z < p.Z {
		if max.X > p.X && max.Y > p.Y && max.Z > p.Z {
			return true
		}
	}
	return false
}

type treeNode struct {
	color      Color
	bounds     Box
	parent     *treeNode
	fileOffset int64

	childIndex,
	voxelsPerAxis,
	numSamples,
	colorSize int
}

func newRootNode(bounds Box, vpa int) *treeNode {
	return &treeNode{bounds: bounds, voxelsPerAxis: vpa}
}

func startNodeCache(channelSize int) (shutdown chan<- struct{}, in chan<- *treeNode, out <-chan *treeNode) {
	shutdownChan := make(chan struct{}, 1)
	inChan := make(chan *treeNode, channelSize)
	outChan := make(chan *treeNode, channelSize)

	go func() {
		nodeMap := make(map[*treeNode]struct{})
		for {
			didWork := false

			select {
			case <-shutdownChan:
				close(outChan)
				return
			case n := <-inChan:
				nodeMap[n] = struct{}{}
				didWork = true
			default:
			}

			for n := range nodeMap {
				select {
				case outChan <- n:
					delete(nodeMap, n)
					didWork = true
				default:
				}
				break
			}

			if didWork == false {
				runtime.Gosched()
			}
		}
	}()

	return shutdownChan, inChan, outChan
}

func indexSize(format OctreeFormat) (int, error) {
	switch format {
	case Mip_R8G8B8_Branch32:
		return 4, nil
	default:
		return 0, unsupportedFormatError
	}
}

func writeTail(seeker io.Seeker, format OctreeFormat) error {
	size, err := indexSize(format)
	if err != nil {
		return err
	}

	size *= 8
	switch format {
	case Mip_R8G8B8_Branch32:
		_, err = seeker.Seek(int64(size), 1)
	default:
		return unsupportedFormatError
	}
	return err
}

func (node *treeNode) spawnChildren(zOffset float64, nodeInChan chan<- *treeNode) {
	childSize := node.bounds.Size / 2
	npv := node.voxelsPerAxis / 2

	childIndexStart := 0
	if zOffset > 0.0 {
		childIndexStart += 4
	}

	child := &treeNode{parent: node}
	child.bounds.Size = childSize
	child.bounds.Pos = node.bounds.Pos
	child.bounds.Pos.Z += zOffset
	child.voxelsPerAxis = npv
	child.childIndex = childIndexStart

	nodeInChan <- child

	child = &treeNode{parent: node}
	child.bounds.Size = childSize
	child.bounds.Pos = node.bounds.Pos
	child.bounds.Pos.X += childSize
	child.bounds.Pos.Z += zOffset
	child.voxelsPerAxis = npv
	child.childIndex = childIndexStart + 1

	nodeInChan <- child

	child = &treeNode{parent: node}
	child.bounds.Size = childSize
	child.bounds.Pos = node.bounds.Pos
	child.bounds.Pos.Y += childSize
	child.bounds.Pos.Z += zOffset
	child.voxelsPerAxis = npv
	child.childIndex = childIndexStart + 2

	nodeInChan <- child

	child = &treeNode{parent: node}
	child.bounds.Size = childSize
	child.bounds.Pos = node.bounds.Pos
	child.bounds.Pos.X += childSize
	child.bounds.Pos.Y += childSize
	child.bounds.Pos.Z += zOffset
	child.voxelsPerAxis = npv
	child.childIndex = childIndexStart + 3

	nodeInChan <- child
}

func (node *treeNode) patchParent(writer io.WriteSeeker, mutex *sync.Mutex, format OctreeFormat) error {
	mutex.Lock()
	defer mutex.Unlock()

	parent := node.parent
	size, err := indexSize(format)

	_, err = writer.Seek(parent.fileOffset+int64(parent.colorSize+node.childIndex*size), 0)
	if err != nil {
		return err
	}

	// TODO Should write proper child-index format
	return binary.Write(writer, binary.BigEndian, uint32(node.fileOffset))
}

func (node *treeNode) serialize(writer io.WriteSeeker, mutex *sync.Mutex, format OctreeFormat, nodeInChan chan<- *treeNode) (bool, error) {
	var (
		size        int
		err         error
		hasChildren bool = true
	)

	mutex.Lock()
	node.fileOffset, err = writer.Seek(0, 1)

	size, err = node.color.writeColor(writer, format)
	if err != nil {
		mutex.Unlock()
		return hasChildren, err
	}
	node.colorSize += size

	err = writeTail(writer, format)
	if err != nil {
		mutex.Unlock()
		return hasChildren, err
	}

	mutex.Unlock()

	if node.voxelsPerAxis > 1 {
		node.spawnChildren(0.0, nodeInChan)
		node.spawnChildren(node.bounds.Size/2, nodeInChan)
	} else {
		hasChildren = false
	}

	if node.parent != nil {
		err := node.patchParent(writer, mutex, format)
		if err != nil {
			return hasChildren, err
		}
	}

	return hasChildren, nil
}
