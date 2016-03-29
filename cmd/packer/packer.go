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

package main

import (
	"bufio"
	"flag"
	"fmt"
	"io/ioutil"
	"math"
	"os"
	"path"

	"github.com/andreas-jonsson/octatron/pack"
	"github.com/ungerik/go3d/float64/mat4"
	"github.com/ungerik/go3d/float64/vec3"
)

type xyzSample struct {
	pos     pack.Point
	r, g, b byte
}

func (s *xyzSample) Color() pack.Color {
	return pack.Color{float32(s.r) / 255, float32(s.g) / 255, float32(s.b) / 255, 1}
}

func (s *xyzSample) Position() pack.Point {
	return s.pos
}

func assert(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(-1)
	}
}

var formatLookup = map[string]pack.OctreeFormat{
	"MipR8G8B8A8UnpackUI32": pack.MipR8G8B8A8UnpackUI32,
	"MipR8G8B8A8UnpackUI16": pack.MipR8G8B8A8UnpackUI16,
	"MipR4G4B4A4UnpackUI16": pack.MipR4G4B4A4UnpackUI16,
	"MipR5G6B5UnpackUI16":   pack.MipR5G6B5UnpackUI16,

	"MipR8G8B8A8PackUI28": pack.MipR8G8B8A8PackUI28,
	"MipR4G4B4A4PackUI30": pack.MipR4G4B4A4PackUI30,
	"MipR5G6B5PackUI30":   pack.MipR5G6B5PackUI30,
	"MipR3G3B2PackUI31":   pack.MipR3G3B2PackUI31,
}

var arguments struct {
	format, input, output     string
	rotate, translate, bounds string

	vpa       int
	threshold float64

	reflectComponent, compress bool
	optimize, filter, dryRun   bool
}

func init() {
	flag.Usage = func() {
		fmt.Printf("Usage: packer [options]\n\n")
		flag.PrintDefaults()
	}

	flag.StringVar(&arguments.format, "format", "MipR8G8B8A8PackUI28", "octree packing format")
	flag.StringVar(&arguments.bounds, "bounds", "0,0,0,1", "octree bounding-box X,Y,Z,SIZE")
	flag.StringVar(&arguments.input, "input", "cloud.xyz", "")
	flag.StringVar(&arguments.output, "output", "tree.oct", "")

	flag.StringVar(&arguments.rotate, "rotate", "0,0,0", "YAW,PITCH,ROLL")
	flag.StringVar(&arguments.translate, "translate", "0,0,0", "X,Y,Z")

	flag.IntVar(&arguments.vpa, "vpa", 64, "voxels per axis")
	flag.Float64Var(&arguments.threshold, "threshold", 0.25, "color-filter threshold")

	flag.BoolVar(&arguments.compress, "compress", false, "use data compression")
	flag.BoolVar(&arguments.optimize, "optimize", true, "optimize tree")
	flag.BoolVar(&arguments.filter, "filter", true, "apply color-filter")
	flag.BoolVar(&arguments.reflectComponent, "reflect", true, "reflection component")
	flag.BoolVar(&arguments.dryRun, "dry", false, "dry-run, parses and transform cloud")
}

func main() {
	flag.Parse()

	infile, err := os.Open(arguments.input)
	assert(err)
	defer infile.Close()

	var reads int64
	size, _ := infile.Seek(0, 2)
	infile.Seek(0, 0)

	outfile, err := os.Create(arguments.output)
	assert(err)

	var (
		yaw, pitch, roll float64
		trans            vec3.T
	)

	mat := mat4.Ident
	fmt.Sscanf(arguments.rotate, "%f,%f,%f", &yaw, &pitch, &roll)
	fmt.Sscanf(arguments.translate, "%f,%f,%f", &trans[0], &trans[1], &trans[2])
	mat.AssignEulerRotation(yaw, pitch, roll)
	mat.Translate(&trans)

	parser := func(samples chan<- pack.Sample) error {
		scanner := bufio.NewScanner(infile)
		progress := -1
		box := pack.Box{pack.Point{math.MaxFloat64, math.MaxFloat64, math.MaxFloat64}, -math.MaxFloat64}

		var (
			s       pack.Sample
			r, g, b byte
		)

		for scanner.Scan() {
			text := scanner.Text()

			var ref float64
			if arguments.reflectComponent {
				_, err := fmt.Sscan(text, &s.Pos.X, &s.Pos.Y, &s.Pos.Z, &ref, &r, &g, &b)
				assert(err)
			} else {
				_, err := fmt.Sscan(text, &s.Pos.X, &s.Pos.Y, &s.Pos.Z, &r, &g, &b)
				assert(err)
			}

			s.Col.R = float32(r) / 255
			s.Col.G = float32(g) / 255
			s.Col.B = float32(b) / 255
			s.Col.A = 1

			v := vec3.T{s.Pos.X, s.Pos.Y, s.Pos.Z}
			mat.TransformVec3(&v)
			s.Pos = pack.Point{v[0], v[1], v[2]}

			box.Pos.X = math.Min(box.Pos.X, s.Pos.X)
			box.Pos.Y = math.Min(box.Pos.Y, s.Pos.Y)
			box.Pos.Z = math.Min(box.Pos.Z, s.Pos.Z)
			box.Size = math.Max(math.Max(math.Max(s.Pos.X, s.Pos.Y), s.Pos.Z), box.Size) - math.Max(math.Max(box.Pos.X, box.Pos.Y), box.Pos.Z)

			reads += int64(len(text) + 1)
			p := int((float64(reads) / float64(size)) * 100)
			if p > progress {
				progress = p
				fmt.Printf("\rProgress: %v%%", p)
			}

			if !arguments.dryRun {
				samples <- s
			}
		}

		fmt.Println("\rProgress: 100%")
		fmt.Println("Bounds:", box)
		return scanner.Err()
	}

	if arguments.dryRun {
		parser(nil)
		return
	}

	var bounds pack.Box
	fmt.Sscanf(arguments.bounds, "%f,%f,%f,%f", &bounds.Pos.X, &bounds.Pos.Y, &bounds.Pos.Z, &bounds.Size)

	cfg := pack.BuildConfig{
		Worker:         parser,
		Writer:         outfile,
		Bounds:         bounds,
		VoxelsPerAxis:  arguments.vpa,
		Format:         formatLookup[arguments.format],
		Optimize:       arguments.optimize,
		ColorFilter:    arguments.filter,
		ColorThreshold: float32(arguments.threshold),
	}

	status, err := pack.BuildTree(&cfg)
	assert(err)
	fmt.Println("Status:", status)

	if arguments.compress {
		fmt.Println("Compressing...")

		zipfile, err := ioutil.TempFile(path.Dir(arguments.output), "")
		assert(err)

		_, err = outfile.Seek(0, 0)
		assert(err)
		assert(pack.CompressTree(outfile, zipfile))

		outfile.Close()
		zipfilePath := zipfile.Name()
		zipfile.Close()

		assert(os.Remove(arguments.output))
		assert(os.Rename(zipfilePath, arguments.output))
	} else {
		outfile.Close()
	}
}
