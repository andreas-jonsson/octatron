/*
Copyright (C) 2015-2016 Andreas T Jonsson

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

package pack

import "errors"

var (
	errUnsupportedFormat = errors.New("unsupported octree-format")
	errInvalidFile       = errors.New("invalid file")
	errOctreeOverflow    = errors.New("octree-format overflow")
	errVoxelsPowerOfTwo  = errors.New("voxels must be a power of two")
	errInputIsCompressed = errors.New("input is compressed")
)
