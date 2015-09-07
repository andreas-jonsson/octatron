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
	"encoding/binary"
)

type FilterConfig struct {
	Writer        io.Writer
	Reader        io.Reader
	Function 	  func(io.Reader, chan<- Sample) error
}

func FilterInput(cfg *FilterConfig) error {
	var err error
	errPtr := &err
	channel := make(chan Sample, 10)

	go func() {
		*errPtr = cfg.Function(cfg.Reader, channel)
		close(channel)
	}()

	for {
		samp, more := <-channel
		if more == false {
			break
		}

		err := binary.Write(cfg.Writer, binary.BigEndian, samp.Position())
		if err != nil {
			return err
		}

		err = binary.Write(cfg.Writer, binary.BigEndian, samp.Color())
		if err != nil {
			return err
		}
	}
	return err
}
