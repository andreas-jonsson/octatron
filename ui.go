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
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

func startUI(data []workerPrivateData, totalVolume uint64, volumeTraversed *uint64) *sync.WaitGroup {
	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer fmt.Println("")
		for {
			var numSamples uint64
			for _, w := range data {
				numSamples += atomic.LoadUint64(&w.numSamples)
			}

			traversed := atomic.LoadUint64(volumeTraversed)
			fmt.Printf("\rProgress %d%%, (%v samples)", int((float32(traversed)/float32(totalVolume))*100.0), numSamples)

			if traversed >= totalVolume {
				wg.Done()
				return
			}

			time.Sleep(500 * time.Millisecond)
		}
	}()

	return &wg
}
