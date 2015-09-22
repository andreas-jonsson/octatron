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

uniform mat4 cameraMatrix;

in vec3 inputPosition;
//in vec3 cameraPosition;

out vec3 rayDirection;
out vec3 rayOrigin;

void main() {
	const vec3 orig = vec3(0.0, 0.0, -2.0);
	vec3 dir = normalize(inputPosition - orig);

	rayDirection = (cameraMatrix * vec4(dir, 1)).xyz;
	rayOrigin = (cameraMatrix * vec4(orig, 1)).xyz;

    gl_Position = vec4(inputPosition, 1);
}
