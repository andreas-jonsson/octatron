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

uniform sampler1D oct;
//uniform float octSize;

in vec3 rayDirection;
in vec3 rayOrigin;

out vec4 outputColor;

bool intersect(in vec3 origin, in vec3 direction, in vec3 bmin, in vec3 bmax, out float dist)
{
    vec3 omin = (bmin - origin) / direction;
    vec3 omax = (bmax - origin) / direction;

    vec3 mmax = max(omax, omin);
    vec3 mmin = min(omax, omin);

    float final = min(mmax.x, min(mmax.y, mmax.z));
    float start = max(max(mmin.x, 0.0), max(mmin.y, mmin.z));

	dist = min(final, start);
    return final > start;
}

const int veryBig = 10000;
const float octSize = 1;

void main() {

	//outputColor = vec4(1,0,0,1);
	//return;

	vec3 childPositions[] = {
		vec3(0, 0, 0), vec3(1, 0, 0), vec3(0, 1, 0), vec3(1, 1, 0),
		vec3(0, 0, 1), vec3(1, 0, 1), vec3(0, 1, 1), vec3(1, 1, 1)
	}


	int nodeOffset = 0;


	float halfSize = octSize * 0.5;
	vec3 min = vec3(-halfSize);
	vec3 max = vec3(halfSize);



	float dist;
	if (intersect(rayOrigin, rayDirection, min, max, dist))
		outputColor = vec4(1,dist - 1.5,0,1);
	else
		outputColor = vec4(0,0,0,1);
	return;


	//float dist;
	//float minDist = veryBig;

	//int newOffset = 1;

	/*********/
	/* 1 * 2 */
	/*********/
	/* 3 * 4 */
	/*********/

/*
	vec3 boxOffset = vec3(-halfSize, -halfSize, -halfSize);
	if (intersect(rayOrigin, rayDirection, min + boxOffset, max + boxOffset, dist)) {
		if (minDist < dist) {
			minDist = dist;
			newOffset = texture(oct, nodeOffset + 1);
		}
	}

	boxOffset = vec3(halfSize, -halfSize, halfSize);
	if (intersect(rayOrigin, rayDirection, min + boxOffset, max + boxOffset, dist)) {
		if (minDist < dist) {
			minDist = dist;
			newOffset = texture(oct, nodeOffset + 2);
		}
	}

	boxOffset = vec3(-halfSize, halfSize, halfSize);
	if (intersect(rayOrigin, rayDirection, min + boxOffset, max + boxOffset, dist)) {
		if (minDist < dist) {
			minDist = dist;
			newOffset = texture(oct, nodeOffset + 3);
		}
	}

	boxOffset = vec3(halfSize, halfSize, halfSize);
	if (intersect(rayOrigin, rayDirection, min + boxOffset, max + boxOffset, dist)) {
		if (minDist < dist) {
			minDist = dist;
			newOffset = texture(oct, nodeOffset + 4);
		}
	}*/

	/********************* Layer 2 (-Z) ***********************/
/*
	boxOffset = vec3(-halfSize, -halfSize, -halfSize);
	if (intersect(rayOrigin, rayDirection, min + boxOffset, max + boxOffset, dist)) {
		if (minDist < dist) {
			minDist = dist;
			newOffset = texture(oct, nodeOffset + 5);
		}
	}

	boxOffset = vec3(halfSize, -halfSize, -halfSize);
	if (intersect(rayOrigin, rayDirection, min + boxOffset, max + boxOffset, dist)) {
		if (minDist < dist) {
			minDist = dist;
			newOffset = texture(oct, nodeOffset + 6);
		}
	}

	boxOffset = vec3(-halfSize, halfSize, -halfSize);
	if (intersect(rayOrigin, rayDirection, min + boxOffset, max + boxOffset, dist)) {
		if (minDist < dist) {
			minDist = dist;
			newOffset = texture(oct, nodeOffset + 7);
		}
	}

	boxOffset = vec3(halfSize, halfSize, -halfSize);
	if (intersect(rayOrigin, rayDirection, min + boxOffset, max + boxOffset, dist)) {
		if (minDist < dist) {
			minDist = dist;
			newOffset = texture(oct, nodeOffset + 8);
		}
	}


*/









/*
	float dist;
	if (intersect(rayOrigin, rayDirection, min, max, dist))
		child1offset = texture(oct, nodeOffset + 1);
		doTest(child1offset, halfSize)

		child2offset = texture(oct, nodeOffset + 2);
		child3offset = texture(oct, nodeOffset + 3);
		child4offset = texture(oct, nodeOffset + 4);
		child5offset = texture(oct, nodeOffset + 5);
		child6offset = texture(oct, nodeOffset + 6);
		child7offset = texture(oct, nodeOffset + 7);
		child8offset = texture(oct, nodeOffset + 8);

		outputColor = vec4(1,dist - 1.5,0,1);
	else
		outputColor = vec4(0,0,0,1);
*/



	/*
	for (int i = 0; i < veryBig; i++) {
		if (intersect(rayOrigin, rayDirection, min, max))
			outputColor = texture(oct, nodeOffset);
	}
	discard;*/
}
