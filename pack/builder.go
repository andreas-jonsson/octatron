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
	"io"
	"sync"
	"sync/atomic"
)

type BuildConfig struct {
	Writer        io.WriteSeeker
	Bounds        Box
	VoxelsPerAxis int
	Format        OctreeFormat
	Interactive   bool
}

type workerPrivateData struct {
	err        error
	numSamples uint64
	worker     Worker
}

func processData(data *workerPrivateData, node *treeNode, sampleChan <-chan Sample) error {
	for {
		sample, more := <-sampleChan
		if more == false {
			err := data.err
			if err != nil {
				return err
			}
			node.color.div(float32(node.numSamples))
			return nil
		}

		node.numSamples++
		atomic.AddUint64(&data.numSamples, 1)

		// Average voxels color value
		col := sample.Color()
		avg := col.sub(&node.color)
		avg = avg.div(float32(node.numSamples))
		node.color.add(avg)
	}
}

func collectData(workerData *workerPrivateData, node *treeNode, sampleChan chan<- Sample) {
	err := workerData.worker.Start(node.bounds, sampleChan)
	if err != nil {
		workerData.err = err
	}
	close(sampleChan)
}

func incVolume(volumeTraversed *uint64, voxelsPerAxis int) uint64 {
	vpa := uint64(voxelsPerAxis)
	volume := vpa * vpa * vpa
	return atomic.AddUint64(volumeTraversed, volume)
}

func BuildTree(workers []Worker, cfg *BuildConfig) error {
	var (
		volumeTraversed uint64
		wgWorkers       sync.WaitGroup
		wgUI            *sync.WaitGroup
	)

	vpa := uint64(cfg.VoxelsPerAxis)
	if vpa == 0 || (vpa&(vpa-1)) != 0 {
		return voxelsPowerOfTwoError
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

	if cfg.Interactive == true {
		wgUI = startUI(workerData, totalVolume, &volumeTraversed)
		defer wgUI.Wait()
	}

	for idx, worker := range workers {
		data := &workerData[idx]
		data.worker = worker

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
				if node.numSamples == 0 {
					// Are we done with the octree
					if incVolume(&volumeTraversed, node.voxelsPerAxis) == totalVolume {
						nodeMapShutdownChan <- struct{}{}
					}
				} else {
					hasChildren, err := node.serialize(cfg.Writer, writeMutex, cfg.Format, nodeMapInChan)
					if err != nil {
						incVolume(&volumeTraversed, node.voxelsPerAxis)
						data.err = err
						return
					} else if hasChildren == false {
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
	return nil
}
