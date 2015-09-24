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

package main

import (
	"errors"
	"os"

	"github.com/andreas-t-jonsson/octatron/pack"
	"github.com/go-gl/gl/v3.2-core/gl"

	"encoding/binary"
	"fmt"
	"strings"
)

func newProgram(vertexShaderSource, fragmentShaderSource string) (uint32, error) {
	vertexShader, err := compileShader(vertexShaderSource, gl.VERTEX_SHADER)
	if err != nil {
		return 0, err
	}

	fragmentShader, err := compileShader(fragmentShaderSource, gl.FRAGMENT_SHADER)
	if err != nil {
		return 0, err
	}

	program := gl.CreateProgram()

	gl.AttachShader(program, vertexShader)
	gl.AttachShader(program, fragmentShader)
	gl.LinkProgram(program)

	var status int32
	gl.GetProgramiv(program, gl.LINK_STATUS, &status)
	if status == gl.FALSE {
		var logLength int32
		gl.GetProgramiv(program, gl.INFO_LOG_LENGTH, &logLength)

		log := strings.Repeat("\x00", int(logLength+1))
		gl.GetProgramInfoLog(program, logLength, nil, gl.Str(log))

		return 0, fmt.Errorf("failed to link program: %v", log)
	}

	gl.DeleteShader(vertexShader)
	gl.DeleteShader(fragmentShader)

	return program, nil
}

func compileShader(source string, shaderType uint32) (uint32, error) {
	shader := gl.CreateShader(shaderType)

	csource := gl.Str(source)
	gl.ShaderSource(shader, 1, &csource, nil)
	gl.CompileShader(shader)

	var status int32
	gl.GetShaderiv(shader, gl.COMPILE_STATUS, &status)
	if status == gl.FALSE {
		var logLength int32
		gl.GetShaderiv(shader, gl.INFO_LOG_LENGTH, &logLength)

		log := strings.Repeat("\x00", int(logLength+1))
		gl.GetShaderInfoLog(shader, logLength, nil, gl.Str(log))

		return 0, fmt.Errorf("failed to compile %v: %v", source, log)
	}

	return shader, nil
}

func newOctree(file string) (uint32, []uint32, error) {
	var (
		header  pack.OctreeHeader
		texture uint32
		data    []uint32
	)

	fp, err := os.Open(file)
	if err != nil {
		return 0, nil, err
	}
	defer fp.Close()

	if err := pack.DecodeHeader(fp, &header); err != nil {
		return 0, nil, err
	}

	if header.Format != pack.MipR8G8B8A8UnpackUI32 {
		return 0, nil, errors.New("invalid octree format")
	}
	nodeSize := uint64(1 + 8)

	var maxSize int32
	gl.GetIntegerv(gl.MAX_TEXTURE_SIZE, &maxSize)

	numInts := header.NumNodes * nodeSize
	if maxSize*maxSize < int32(numInts) {
		panic("octree does not fit on GPU")
	}

	var (
		height int32
		width  = maxSize
	)

	for height < width {
		maxSize = width
		height = int32(numInts/uint64(width)) + 1
		width /= 2
	}

	fmt.Printf("Octree is loaded in an %vx%v, R32UI, 2D texture.\n", maxSize, height)

	textureSize := maxSize * height
	data = make([]uint32, textureSize)

	for i := uint64(0); i < header.NumNodes; i++ {
		start := i * nodeSize
		if err := binary.Read(fp, binary.BigEndian, data[start:start+nodeSize]); err != nil {
			return 0, nil, err
		}

		// Recalculate alpha value.
		data[start] &= 0xffffff00
		for j := uint64(1); j < 9; j++ {
			if data[start+j] > 0 {
				data[start] |= uint32(0x100) >> j
			}
		}
	}

	gl.GenTextures(1, &texture)
	gl.ActiveTexture(gl.TEXTURE0)
	gl.BindTexture(gl.TEXTURE_2D, texture)

	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.NEAREST)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.NEAREST)

	gl.TexImage2D(
		gl.TEXTURE_2D,
		0,
		gl.R32UI,
		maxSize,
		height,
		0,
		gl.RED_INTEGER,
		gl.UNSIGNED_INT,
		gl.Ptr(data))

	if glErr := gl.GetError(); glErr != gl.NO_ERROR {
		err = fmt.Errorf("GL error, loading octree: %x", glErr)
	}

	return texture, data, err
}
