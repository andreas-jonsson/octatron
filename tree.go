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

import "runtime"

type Color struct {
	R, G, B, A float32
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
	color         Color
	bounds        Box
	fileOffset    uint64
	parent        *treeNode
	childIndex    int
	numSamples    int
	voxelsPerAxis int
	children      [8]*treeNode
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

			if !didWork {
				runtime.Gosched()
			}
		}
	}()

	return shutdownChan, inChan, outChan
}
