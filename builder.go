/************************************************************************/
/* Octatron                                                             */
/* Copyright (c) 2015 Andreas T Jonsson <mail@andreasjonsson.se>        */
/*                                                                      */
/* Octatron is free software: you can redistribute it and/or modify     */
/* it under the terms of the GNU General Public License as published by */
/* the Free Software Foundation, either version 3 of the License, or    */
/* (at your option) any later version.                                  */
/*                                                                      */
/* Octatron is distributed in the hope that it will be useful,          */
/* but WITHOUT ANY WARRANTY; without even the implied warranty of       */
/* MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the        */
/* GNU General Public License for more details.                         */
/*                                                                      */
/* You should have received a copy of the GNU General Public License    */
/* along with Octatron.  If not, see <http://www.gnu.org/licenses/>.    */
/************************************************************************/

package octatron

import (
    "io"
    "sync"
)

type TreeResult struct {
}

type TreeConfig struct {
    Writer io.Writer
    Bounds Box
    VoxelsPerAxis int
}

type workerData struct {

}

func processSample(data *workerData, sample *Sample) {

}

func BuildTree(workers []Worker, cfg *TreeConfig) (*TreeResult, error) {
    errorChan := make(chan error)
    data := make([]workerData, len(workers))

    voxelSize := cfg.Bounds.Size / float64(cfg.VoxelsPerAxis)

    var wgWorkers sync.WaitGroup
    for idx, worker := range workers {
        wgWorkers.Add(1)

        go func() {
            defer wgWorkers.Done()

            sampleBox := Box{cfg.Bounds.Pos, voxelSize}
            sampleChan := make(chan Sample, 10)

            go func() {
                err := worker.Run(sampleBox, sampleChan)
                if err != nil {
                    errorChan <- err
                }
                close(sampleChan)
            }()

            for {
                sample, more := <-sampleChan
                processSample(&data[idx], &sample)
                if !more {
                    break
                }
            }
        }()
    }

    wgWorkers.Wait()

    select {
    case err := <-errorChan:
            return nil, err
        default:
            return &TreeResult{}, nil
    }
}
