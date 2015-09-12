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
)

func FileSize(seeker io.Seeker) (int64, error) {
	var (
		offset int64
		size   int64
		err    error
	)

	offset, err = seeker.Seek(0, 1)
	if err != nil {
		return 0, err
	}

	size, err = seeker.Seek(0, 2)
	if err != nil {
		return 0, err
	}

	_, err = seeker.Seek(offset, 0)
	if err != nil {
		return 0, err
	}

	return size, err
}

func FileSizeByName(file string) (int64, error) {
	fp, err := os.Open(file)
	if err != nil {
		return 0, err
	}
	defer fp.Close()
	return FileSize(fp)
}
