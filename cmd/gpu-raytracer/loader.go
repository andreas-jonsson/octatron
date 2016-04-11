// +build gpu

/*
Copyright (C) 2015 Andreas T Jonsson

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

package main

import (
	"errors"
	"os"

	"github.com/andreas-jonsson/octatron/pack"
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
		if err := binary.Read(fp, binary.LittleEndian, data[start:start+nodeSize]); err != nil {
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

var vertexShader = `
#version 150

uniform mat4 cameraMatrix;

in vec3 inputPosition;

out vec3 rayDirection;
out vec3 rayOrigin;

void main() {
	const vec3 orig = vec3(0.0, 0.0, -2.0);
	vec3 dir = normalize(inputPosition - orig);

	rayDirection = (cameraMatrix * vec4(dir, 1)).xyz;
	rayOrigin = (cameraMatrix * vec4(orig, 1)).xyz;

    gl_Position = vec4(inputPosition, 1);
}
`

var fragmentShader = `
#version 150

uniform usampler2D oct;

in vec3 rayDirection;
in vec3 rayOrigin;

out vec4 outputColor;

const float veryBig = 10000;
const float octSize = 1;

bool intersect(in vec3 origin, in vec3 direction, in float len, in vec3 bmin, in vec3 bmax, out float dist) {
    vec3 omin = (bmin - origin) / direction;
    vec3 omax = (bmax - origin) / direction;

    vec3 mmax = max(omax, omin);
    vec3 mmin = min(omax, omin);

    float final = min(mmax.x, min(mmax.y, mmax.z));
    float start = max(max(mmin.x, 0.0), max(mmin.y, mmin.z));

	dist = min(final, start);
    return final > start && dist < len;
}

ivec2 convertAddress(uint addr) {
    ivec2 size = textureSize(oct, 0);
    return ivec2(addr % uint(size.x), addr / uint(size.x));
}

void decodeColor(in uint color, out vec4 outputColor) {
    outputColor.r = float((color & 0xff000000u) >> 24) / 255.0;
    outputColor.g = float((color & 0xff0000u) >> 16) / 255.0;
    outputColor.b = float((color & 0xff00u) >> 8) / 255.0;
    outputColor.a = float(color & 0xffu) / 255.0;
}

struct workNode {
    vec3 pos;
    float size;
    uint index;
};

const uint nodeSize = 36u;
const vec3[8] childPositions = vec3[](
    vec3(0, 0, 0), vec3(1, 0, 0), vec3(0, 1, 0), vec3(1, 1, 0),
    vec3(0, 0, 1), vec3(1, 0, 1), vec3(0, 1, 1), vec3(1, 1, 1)
);

bool intersectTree(in vec3 origin, in vec3 direction, in float len, in uint nodeIndex, in vec3 nodePos, in float nodeScale, out vec4 outputColor, out float dist) {
    int top = -1;
    workNode work[64];

    float shortestDist = veryBig;
    uint candidateColor;
    float intersectionDist;

	if (intersect(origin, direction, len, nodePos, nodePos + vec3(nodeScale), intersectionDist) == false)
		return false;

    while (true) {
        uint nodeAddress = (nodeIndex * nodeSize) / 4u;
        uint color = texelFetch(oct, convertAddress(nodeAddress), 0).r;
        uint mask = color & 0x000000ffu;

        if (mask != 0u) {
            float childScale = nodeScale * 0.5;

            for (uint i = 0u; i < 8u; i++) {
                if (((0x80u >> i) & mask) != 0u) {
					vec3 pos = nodePos + (childPositions[i] * childScale);
					if (intersect(origin, direction, len, pos, pos + vec3(childScale), intersectionDist) == true) {
                        top++;
                        work[top].pos = pos;
                        work[top].size = childScale;
                        work[top].index = texelFetch(oct, convertAddress(nodeAddress + i + 1u), 0).r;
					}
                }
            }
        } else if (intersectionDist < shortestDist) {
            shortestDist = intersectionDist;
            len = intersectionDist;
            candidateColor = color;
        }

        if (top == -1) {
            if (shortestDist >= veryBig) {
                return false;
            }
            decodeColor(candidateColor, outputColor);
            return true;
        }

        nodeScale = work[top].size;
        nodeIndex = work[top].index;
        nodePos = work[top].pos;
        top--;
    }
}

void main() {
    float halfSize = octSize * 0.5;
    vec3 min = vec3(-halfSize);

    float dist;
    vec4 color;

    if (intersectTree(rayOrigin, rayDirection, veryBig, 0u, min, octSize, color, dist) == false) {
        discard;
        return;
    }

    outputColor = vec4(color.rgb,1);
}
`
