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

#version 150

uniform usampler2D oct;

in vec3 rayDirection;
in vec3 rayOrigin;

out vec4 outputColor;

const int veryBig = 10000;
const float octSize = 1;

bool intersect(in vec3 origin, in vec3 direction, in vec3 bmin, in vec3 bmax, out float dist) {
    vec3 omin = (bmin - origin) / direction;
    vec3 omax = (bmax - origin) / direction;

    vec3 mmax = max(omax, omin);
    vec3 mmin = min(omax, omin);

    float final = min(mmax.x, min(mmax.y, mmax.z));
    float start = max(max(mmin.x, 0.0), max(mmin.y, mmin.z));

	dist = min(final, start);
    return final > start;
}

ivec2 convertAddress(uint addr) {
    ivec2 size = textureSize(oct, 0);
    return ivec2(addr % uint(size.x), addr / uint(size.x));
}

void decodeColor(in uint nodeAddress, out vec4 outputColor) {
    uint color = texelFetch(oct, convertAddress(nodeAddress), 0).r;
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

bool intersectTree(in vec3 origin, in vec3 direction, in uint nodeIndex, in vec3 nodePos, in float nodeScale, out vec4 outputColor, out float dist) {
    int top = -1;
    workNode work[64];

    float shortestDist = 100000000.0;
    uint candidate = 0xffffffffu;
    float intersectionDist;

    while (true) {
        if (intersect(origin, direction, nodePos, nodePos + vec3(nodeScale), intersectionDist) == true) {
            int numChild = 0;
            uint nodeAddress = (nodeIndex * nodeSize) / 4u;
            float childScale = nodeScale * 0.5;

            for (uint i = 0u; i < 8u; i++) {
                uint child = texelFetch(oct, convertAddress(nodeAddress + i + 1u), 0).r;
                if (child > 0u) {
                    numChild++;

                    top++;
                    work[top].pos = nodePos + (childPositions[i] * childScale);
                    work[top].size = childScale;
                    work[top].index = child;
                }
            }

            if (numChild == 0) {
                if (intersectionDist < shortestDist) {
                    shortestDist = intersectionDist;
                    candidate = nodeAddress;
                }
            }
        }

        if (top == -1) {
            if (candidate == 0xffffffffu) {
                return false;
            }
            decodeColor(candidate, outputColor);
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

    if (intersectTree(rayOrigin, rayDirection, 0u, min, octSize, color, dist) == false) {
        discard;
        return;
    }

    outputColor = vec4(color.rgb,1);
}
