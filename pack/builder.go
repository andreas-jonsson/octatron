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
	"compress/gzip"
	"encoding/binary"
	"errors"
	"io"
	"sync"
	"sync/atomic"
)

const (
	binaryVersion  byte = 0
	endianMask     byte = 0x1
	compressedMask byte = 0x2
)

type Header struct {
	Sign          [4]byte
	Version       byte
	Format        OctreeFormat
	Flags         byte
	Unused        byte
	NumNodes      uint64
	NumLeafs      uint64
	VoxelsPerAxis uint32
}

func (h *Header) BigEndian() bool {
	return h.Flags&endianMask == endianMask
}

func (h *Header) Compressed() bool {
	return h.Flags&compressedMask == compressedMask
}

type BuildConfig struct {
	Writer        io.WriteSeeker
	Bounds        Box
	VoxelsPerAxis int
	Format        OctreeFormat
	Interactive   bool
}

type workerPrivateData struct {
	err         error
	numSamples  uint64
	numRequests uint64
	worker      Worker
}

func processData(data *workerPrivateData, node *treeNode, sampleChan <-chan Sample) error {
	for {
		sample, more := <-sampleChan
		if more == false {
			err := data.err
			if err != nil {
				return err
			}
			node.color.div(float32(node.numSamplesInNode))
			return nil
		}

		node.numSamplesInNode++
		atomic.AddUint64(&data.numSamples, 1)

		// Average voxels color value
		col := sample.Color()
		avg := col.sub(&node.color)
		avg.div(float32(node.numSamplesInNode))
		node.color.add(avg)
	}
}

func collectData(workerData *workerPrivateData, node *treeNode, sampleChan chan<- Sample) {
	err := workerData.worker.Start(node.bounds, sampleChan)
	if err != nil {
		workerData.err = err
	}
	atomic.AddUint64(&workerData.numRequests, 1)
	close(sampleChan)
}

func incVolume(volumeTraversed *uint64, voxelsPerAxis int) uint64 {
	vpa := uint64(voxelsPerAxis)
	volume := vpa * vpa * vpa
	return atomic.AddUint64(volumeTraversed, volume)
}

func writeHeader(writer io.Writer, header *Header) error {
	err := binary.Write(writer, binary.BigEndian, header)
	if err != nil {
		return nil
	}
	return err
}

func CompressTree(oct io.Reader, ocz io.Writer) error {
	var header Header
	err := binary.Read(oct, binary.BigEndian, &header)
	if err != nil {
		return err
	}

	if header.Compressed() == true {
		return errors.New("input is compressed")
	}
	header.Flags = header.Flags & compressedMask

	err = binary.Write(ocz, binary.BigEndian, header)
	if err != nil {
		return err
	}

	// We expect format to be 4 byte aligned
	var buffer [4]byte
	zip := gzip.NewWriter(ocz)

	for {
		err = binary.Read(oct, binary.BigEndian, &buffer)
		if err == io.EOF {
			zip.Close()
			return nil
		} else if err != nil {
			return err
		}

		err = binary.Write(zip, binary.BigEndian, buffer)
		if err != nil {
			return err
		}
	}
	return nil
}

func BuildTree(workers []Worker, cfg *BuildConfig) error {
	var (
		volumeTraversed, numLeafs, numNodes uint64
		wgWorkers                           sync.WaitGroup
		wgUI                                *sync.WaitGroup
	)

	vpa := uint64(cfg.VoxelsPerAxis)
	if vpa == 0 || (vpa&(vpa-1)) != 0 {
		return errVoxelsPowerOfTwo
	}

	totalVolume := vpa * vpa * vpa

	numWorkers := len(workers)
	workerData := make([]workerPrivateData, numWorkers)
	wgWorkers.Add(numWorkers)

	writeMutex := &sync.Mutex{}

	nodeMapShutdownChan, nodeMapInChan, nodeMapOutChan := startNodeCache(numWorkers)
	nodeMapInChan <- newRootNode(cfg.Bounds, cfg.VoxelsPerAxis)

	defer func() {
		nodeMapShutdownChan <- struct{}{}
	}()

	for idx, worker := range workers {
		data := &workerData[idx]
		data.worker = worker
	}

	if cfg.Interactive == true {
		wgUI = startUI(workerData, totalVolume, &volumeTraversed, &numLeafs)
		defer wgUI.Wait()
	}

	var header Header
	header.Sign[0] = 0x1b
	header.Sign[1] = 0x6f
	header.Sign[2] = 0x63
	header.Sign[3] = 0x74
	header.Version = binaryVersion
	header.Format = cfg.Format
	header.Unused = 0x0
	header.NumNodes = 0
	header.NumLeafs = 0
	header.VoxelsPerAxis = uint32(cfg.VoxelsPerAxis)

	writeHeader(cfg.Writer, &header)

	for idx, _ := range workers {
		data := &workerData[idx]

		// Spawn worker
		go func() {
			defer wgWorkers.Done()

			// Process jobs
			for {
				node, more := <-nodeMapOutChan
				if more == false {
					return
				}

				sampleChan := make(chan Sample, 10)
				go collectData(data, node, sampleChan)
				if processData(data, node, sampleChan) != nil {
					incVolume(&volumeTraversed, node.voxelsPerAxis)
					return
				}

				// This is a leaf
				if node.numSamplesInNode == 0 {
					// Are we done with the octree
					if incVolume(&volumeTraversed, node.voxelsPerAxis) == totalVolume {
						nodeMapShutdownChan <- struct{}{}
					}
				} else {
					atomic.AddUint64(&numNodes, 1)
					hasChildren, err := node.serialize(cfg.Writer, writeMutex, cfg.Format, nodeMapInChan)
					if err != nil {
						incVolume(&volumeTraversed, node.voxelsPerAxis)
						data.err = err
						return
					} else if hasChildren == false {
						atomic.AddUint64(&numLeafs, 1)
						if incVolume(&volumeTraversed, node.voxelsPerAxis) == totalVolume {
							nodeMapShutdownChan <- struct{}{}
						}
					}
				}
			}
		}()
	}

	wgWorkers.Wait()
	for _, data := range workerData {
		if data.err != nil {
			return data.err
		}
	}

	header.NumNodes = numNodes
	header.NumLeafs = numLeafs
	if _, err := cfg.Writer.Seek(0, 0); err != nil {
		return err
	}

	writeHeader(cfg.Writer, &header)
	if _, err := cfg.Writer.Seek(0, 2); err != nil {
		return err
	}
	return nil
}
