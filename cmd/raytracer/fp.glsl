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

#version 410

uniform usampler1D oct;
//uniform float octSize;

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

void decodeColor(in uint nodeAddress, out vec4 outputColor) {
    uint color = texelFetch(oct, int(nodeAddress), 0).r;
    outputColor.r = float((color & 0xff000000u) >> 24) / 255.0;
    outputColor.g = float((color & 0xff0000u) >> 16) / 255.0;
    outputColor.b = float((color & 0xff00u) >> 8) / 255.0;
    outputColor.a = float(color & 0xffu) / 255.0;
}

void intersectTree(in vec3 origin, in vec3 direction, in uint nodeIndex, in vec3 nodePos, in float nodeScale, out vec4 outputColor, out float dist) {
    const uint nodeSize = 36u;
    const vec3[8] childPositions = vec3[](
        vec3(0, 0, 0), vec3(1, 0, 0), vec3(0, 1, 0), vec3(1, 1, 0),
        vec3(0, 0, 1), vec3(1, 0, 1), vec3(0, 1, 1), vec3(1, 1, 1)
    );

    for (;;) {
        uint nodeAddress = (nodeIndex * nodeSize) / 4u;
        float childScale = nodeScale * 0.5;

        float shortestDist = 100000000.0;
        uint candidate = 0xffffffffu;
        vec3 candidatePos;

        int numChild = 0;
        for (int i = 1; i < 9; i++) {
            uint child = texelFetch(oct, int(nodeAddress) + i, 0).r;



            //uint child = texture(oct, float((int(nodeAddress) + i) / textureSize(oct, 0))).r;


            //decodeColor(0, outputColor);
            //return;








            if (child > 0u) {
                numChild++;




                vec3 bmin = nodePos + (childPositions[0] * childScale);
                vec3 bmax = bmin + vec3(childScale);

                float intersectionDist;
                //intersect(origin, direction, bmin, bmax, intersectionDist);
                if (intersect(origin, direction, bmin, bmax, intersectionDist) == true) {

                    //outputColor = vec4(0,0,intersectionDist-1.5,1);
                    //return;


                    if (intersectionDist < shortestDist) {
                        shortestDist = intersectionDist;
                        candidate = child;
                        candidatePos = bmin;


                        //outputColor = vec4(1,0,0,1);
                        //return;
                    }
                }
            }
        }

        if (numChild == 0) {

            //decodeColor(nodeAddress, outputColor);

            outputColor = vec4(0,1,0,1);
            return;
        }

        // This should be avoided.
        if (candidate == 0xffffffffu) {
            outputColor = vec4(1,0,0,1);
            return;
        }

        nodeScale = childScale;
        nodeIndex = candidate;
        nodePos = candidatePos;
    }
}

void main() {
    float halfSize = octSize * 0.5;
	vec3 min = vec3(-halfSize);

    float dist;
    vec4 color;

    intersectTree(rayOrigin, rayDirection, 0u, min, octSize, color, dist);

    outputColor = vec4(color.rgb,1);
/*
    uint r = texelFetch(oct, 0,0).r;
    uint g = texelFetch(oct, 1,0).r;
    uint b = texelFetch(oct, 2,0).r;
    uint a = texelFetch(oct, 3,0).r;
    outputColor = vec4(float(r)/255,float(g)/255,float(b)/255,float(a)/255);*/

    //decodeColor(0u, outputColor);

    //outputColor = vec4(float((u & 0xff000000u) >> 24) / 255,0,0,1);
    //outputColor = vec4(float(u) / 255,0,0,1);


	//outputColor = vec4(1,0,0,1);
	//return;

/*
	int nodeOffset = 0;






	float dist;
	if (intersect(rayOrigin, rayDirection, min, max, dist))
		outputColor = vec4(1,dist - 1.5,0,1);
	else
		outputColor = vec4(0,0,0,1);
*/
}
