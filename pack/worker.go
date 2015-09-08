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
	"os"
	"encoding/binary"
)

type Sample interface {
	Color() Color
	Position() Point
}

type Worker interface {
	Start(bounds Box, samples chan<- Sample) error
	Stop()
}

type defaultWorker struct {
	file *os.File
}

const defaultNodeSize = 8 * 3 + 4 * 4 // x,y,z + r,g,b,a

func (w *defaultWorker) Start(bounds Box, samples chan<- Sample) error {
	for {
		sample := new(filterSample)
		err := binary.Read(w.file, binary.BigEndian, samples)
		if err == io.EOF {
			return nil
		} else if err != nil {
			return err
		}
		samples <- sample
	}
}

func (w *defaultWorker) Stop() {
	w.file.Close()
}

func NewDefaultWorker(inputFile string) (Worker, error) {
	var err error
	w := new(defaultWorker)

	w.file, err = os.Open(inputFile)
	if err != nil {
		return w, err
	}

	return w, nil
}
