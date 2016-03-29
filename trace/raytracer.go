/*
Copyright (C) 2016 Andreas T Jonsson

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

package trace

import (
	"errors"
	"image"
	"image/color"
	"image/draw"
	"io"
	"math"
	"runtime"
	"sync"
	"sync/atomic"

	"github.com/andreas-jonsson/octatron/pack"
	"github.com/ungerik/go3d/quaternion"
	"github.com/ungerik/go3d/vec3"
)

type (
	Vec3   [3]float32
	Octree []octreeNode

	Camera interface {
		Position() Vec3
		LookAt() Vec3
		Up() Vec3
	}

	Config struct {
		FieldOfView  float32
		TreeScale    float32
		TreePosition Vec3

		ViewDist      float32
		Jitter, Depth bool
		MultiThreaded bool
		Images        [2]*image.RGBA
	}

	Raytracer struct {
		cfg        Config
		frame      uint32
		numThreads int
		clear      color.RGBA
		depth      [2]*image.Gray16
		wg         [2]sync.WaitGroup
		work       chan rtJob
	}
)

var InvalidSizeError = errors.New("invalid size")

type (
	infiniteRay [2]vec3.T

	rtJob struct {
		camera   Camera
		tree     Octree
		maxDepth float32

		from, to, idx int
	}

	octreeNode struct {
		color    color.RGBA
		children [8]uint32
	}
)

func (n *octreeNode) setColor(color *pack.Color) {
	n.color.R = uint8(color.R * 255)
	n.color.G = uint8(color.G * 255)
	n.color.B = uint8(color.B * 255)
	n.color.A = uint8(color.A * 255)
}

type LookAtCamera struct {
	Pos  Vec3
	Look Vec3
}

func (c *LookAtCamera) Up() Vec3 {
	return Vec3{0, 1, 0}
}

func (c *LookAtCamera) Position() Vec3 {
	return c.Pos
}

func (c *LookAtCamera) LookAt() Vec3 {
	return c.Look
}

type FreeFlightCamera struct {
	Pos        Vec3
	XRot, YRot float32
}

func (c *FreeFlightCamera) Up() Vec3 {
	return Vec3{0, 1, 0}
}

func (c *FreeFlightCamera) Forward() Vec3 {
	lookAt := vec3.T(c.LookAt())
	position := vec3.T(c.Pos)
	return Vec3(vec3.Sub(&lookAt, &position))
}

func (c *FreeFlightCamera) Right() Vec3 {
	up := vec3.T(c.Up())
	forward := vec3.T(c.Forward())
	return Vec3(vec3.Cross(&up, &forward))
}

func (c *FreeFlightCamera) Position() Vec3 {
	return c.Pos
}

func (c *FreeFlightCamera) LookAt() Vec3 {
	forward := vec3.T{0, 0, -1}
	position := vec3.T(c.Pos)

	quat := quaternion.FromEulerAngles(c.XRot, c.YRot, 0)
	quat.RotateVec3(&forward)

	return Vec3(vec3.Add(&position, &forward))
}

func (c *FreeFlightCamera) Move(dist float32) {
	position := vec3.T(c.Pos)
	forward := vec3.T(c.Forward())

	forward.Scale(dist)
	c.Pos = Vec3(vec3.Add(&position, &forward))
}

func (c *FreeFlightCamera) Lift(dist float32) {
	position := vec3.T(c.Pos)
	forward := vec3.T(c.Forward())
	right := vec3.T(c.Right())
	up := vec3.Cross(&forward, &right)

	up.Scale(dist)
	c.Pos = Vec3(vec3.Add(&position, &up))
}

func (c *FreeFlightCamera) Strafe(dist float32) {
	position := vec3.T(c.Pos)
	right := vec3.T(c.Right())

	right.Scale(dist)
	c.Pos = Vec3(vec3.Add(&position, &right))
}

func TreeWidthToDepth(width int) int {
	n, d := width, 0
	for ; n > 0; d++ {
		n >>= 1
	}
	return d
}

func LoadOctree(reader io.Reader) (Octree, int, error) {
	var (
		color  pack.Color
		header pack.OctreeHeader
	)

	if err := pack.DecodeHeader(reader, &header); err != nil {
		return nil, 0, err
	}

	data := make([]octreeNode, header.NumNodes)
	for i := range data {
		n := &data[i]
		if err := pack.DecodeNode(reader, header.Format, &color, n.children[:]); err != nil {
			return nil, 0, err
		}
		n.setColor(&color)
	}

	return data, int(header.VoxelsPerAxis), nil
}

func Reconstruct(a, b image.Image, out draw.Image) error {
	outputSize := out.Bounds().Max
	inputSize := a.Bounds().Max

	if inputSize != b.Bounds().Max {
		return InvalidSizeError
	}

	if inputSize.X != outputSize.X/2 || inputSize.Y != outputSize.Y {
		return InvalidSizeError
	}

	img := [2]image.Image{a, b}
	for y := 0; y < inputSize.Y; y++ {
		for x := 0; x < inputSize.X; x++ {
			left, right := img[y%2], img[(y+1)%2]
			out.Set(x*2, y, left.At(x, y))
			out.Set(x*2+1, y, right.At(x, y))
		}
	}

	return nil
}

func intersectBox(ray *infiniteRay, lenght float32, box *vec3.Box) float32 {
	origin := ray[0]
	direction := ray[1]

	oMin := vec3.Sub(&box.Min, &origin)
	oMax := vec3.Sub(&box.Max, &origin)

	oMin[0] /= direction[0]
	oMin[1] /= direction[1]
	oMin[2] /= direction[2]

	oMax[0] /= direction[0]
	oMax[1] /= direction[1]
	oMax[2] /= direction[2]

	mMax := vec3.Max(&oMax, &oMin)
	mMin := vec3.Min(&oMax, &oMin)

	final := math.Min(float64(mMax[0]), math.Min(float64(mMax[1]), float64(mMax[2])))
	start := math.Max(math.Max(float64(mMin[0]), 0.0), math.Max(float64(mMin[1]), float64(mMin[2])))

	dist := float32(math.Min(final, start))
	if final > start && dist < lenght {
		return dist
	}
	return lenght
}

var childPositions = []vec3.T{
	vec3.T{0, 0, 0}, vec3.T{1, 0, 0}, vec3.T{0, 1, 0}, vec3.T{1, 1, 0},
	vec3.T{0, 0, 1}, vec3.T{1, 0, 1}, vec3.T{0, 1, 1}, vec3.T{1, 1, 1},
}

func (rt *Raytracer) Wait(frame int) {
	rt.wg[frame].Wait()
}

func (rt *Raytracer) Depth(frame int) *image.Gray16 {
	rt.Wait(frame)
	return rt.depth[frame]
}

func (rt *Raytracer) ClearDepth(frame int) {
	rt.Wait(frame)
	img := rt.depth[frame]
	clear := image.Uniform{color.Gray16{math.MaxUint16}}
	draw.Draw(img, img.Bounds(), &clear, image.ZP, draw.Src)
}

func (rt *Raytracer) SetClearColor(c color.RGBA) {
	rt.clear = c
}

func (rt *Raytracer) Frame() int {
	return int(atomic.LoadUint32(&rt.frame) % 2)
}

func (rt *Raytracer) intersectTree(tree []octreeNode, ray *infiniteRay, nodePos *vec3.T, nodeScale, length, maxDepth float32, nodeIndex, treeDepth uint32) (float32, color.RGBA) {
	var (
		color = rt.clear
		node  = tree[nodeIndex]

		// Declare this here to avoid runtime allocation.
		pos vec3.T
	)

	box := vec3.Box{*nodePos, vec3.T{nodePos[0] + nodeScale, nodePos[1] + nodeScale, nodePos[2] + nodeScale}}
	boxDist := intersectBox(ray, length, &box)

	if boxDist == length {
		return length, color
	}

	{
		d := (boxDist / rt.cfg.ViewDist)
		if treeDepth > uint32(maxDepth*(1-d*d)) {
			return boxDist, node.color
		}
	}

	numChild := 0
	childScale := nodeScale * 0.5
	childDepth := treeDepth + 1

	for i, childIndex := range node.children {
		if childIndex != 0 {
			numChild++
			scaled := childPositions[i].Scaled(childScale)
			pos = vec3.Add(nodePos, &scaled)

			if ln, col := rt.intersectTree(tree, ray, &pos, childScale, length, maxDepth, childIndex, childDepth); ln < length {
				length = ln
				color = col
			}
		}
	}

	if numChild == 0 {
		return boxDist, node.color
	}

	return length, color
}

func (rt *Raytracer) calcIncVectors(camera Camera, size image.Point) (vec3.T, vec3.T, vec3.T) {
	width := float32(size.X)
	height := float32(size.Y)

	lookAtPoint := vec3.T(camera.LookAt())
	eyePoint := vec3.T(camera.Position())
	up := vec3.T(camera.Up())

	viewDirection := vec3.Sub(&lookAtPoint, &eyePoint)
	u := vec3.Cross(&viewDirection, &up)
	v := vec3.Cross(&u, &viewDirection)
	u.Normalize()
	v.Normalize()

	viewPlaneHalfWidth := float32(math.Tan(float64(rt.cfg.FieldOfView / 2)))
	aspectRatio := height / width
	viewPlaneHalfHeight := aspectRatio * viewPlaneHalfWidth

	sV := v.Scaled(viewPlaneHalfHeight)
	sU := u.Scaled(viewPlaneHalfWidth)

	lookV := vec3.Sub(&lookAtPoint, &sV)
	viewPlaneBottomLeftPoint := vec3.Sub(&lookV, &sU)

	xIncVector := u.Scaled(2 * viewPlaneHalfWidth)
	yIncVector := v.Scaled(2 * viewPlaneHalfHeight)

	xIncVector[0] /= width
	xIncVector[1] /= width
	xIncVector[2] /= width

	yIncVector[0] /= height
	yIncVector[1] /= height
	yIncVector[2] /= height

	return xIncVector, yIncVector, viewPlaneBottomLeftPoint
}

func (rt *Raytracer) traceScanLines(job *rtJob) {
	cfg := &rt.cfg
	idx := job.idx
	img := cfg.Images[idx]
	depth := rt.depth[idx]
	size := img.Bounds().Max

	testDepth := cfg.Depth
	nodeScale := cfg.TreeScale
	nodePos := vec3.T(cfg.TreePosition)
	viewDist := cfg.ViewDist

	jitter, step := 0, 1
	if cfg.Jitter {
		jitter = 1
		step = 2
		size.X *= 2
	}

	xInc, yInc, bottomLeft := rt.calcIncVectors(job.camera, size)
	eyePoint := vec3.T(job.camera.Position())

	var (
		col  color.RGBA
		dist float32
	)

	for h := job.from; h < job.to; h++ {
		start := ((h + idx) % 2) * jitter

		for w := start; w < size.X; w += step {
			x := xInc.Scaled(float32(w))
			y := yInc.Scaled(float32(h))

			x = vec3.Add(&x, &y)
			viewPlanePoint := vec3.Add(&bottomLeft, &x)

			dir := vec3.Sub(&viewPlanePoint, &eyePoint)
			dir.Normalize()

			ray := infiniteRay{eyePoint, dir}
			dx, dy := w/step, size.Y-h

			if testDepth {
				max := (float32(depth.Gray16At(dx, dy).Y) / math.MaxUint16) * viewDist
				dist, col = rt.intersectTree(job.tree, &ray, &nodePos, nodeScale, max, job.maxDepth, 0, 0)
				d := color.Gray16{uint16(math.MaxUint16 * (dist / viewDist))}
				depth.SetGray16(dx, dy, d)
			} else {
				_, col = rt.intersectTree(job.tree, &ray, &nodePos, nodeScale, viewDist, job.maxDepth, 0, 0)
			}
			img.SetRGBA(dx, dy, col)
		}
	}
}

func (rt *Raytracer) workerLoop() {
	for {
		job, ok := <-rt.work
		if !ok {
			return
		}

		rt.traceScanLines(&job)
		rt.wg[job.idx].Done()
	}
}

func (rt *Raytracer) Trace(camera Camera, tree Octree, maxDepth int) int {
	cfg := &rt.cfg
	idx := int(atomic.LoadUint32(&rt.frame) % 2)
	size := cfg.Images[0].Bounds().Max // We assume this call is thread-safe.

	rt.Wait(idx)

	if cfg.Jitter {
		atomic.AddUint32(&rt.frame, 1)
	}

	height := size.Y
	batchSize := height / rt.numThreads
	rt.wg[idx].Add(rt.numThreads)

	for y := 0; y < height; y += batchSize {
		rt.work <- rtJob{camera: camera,
			tree:     tree,
			maxDepth: float32(maxDepth),
			from:     y,
			to:       y + batchSize,
			idx:      idx,
		}
	}

	return idx
}

func (rt *Raytracer) Close() {
	close(rt.work)
}

func NewRaytracer(cfg Config) *Raytracer {
	rect := cfg.Images[0].Bounds()
	numCPU := 1

	if cfg.MultiThreaded {
		numCPU = runtime.NumCPU()
		for numCPU%rect.Max.Y == 0 {
			numCPU++
		}
	}

	rt := &Raytracer{cfg: cfg, clear: color.RGBA{0, 0, 0, 255}, numThreads: numCPU, work: make(chan rtJob, numCPU*2)}

	if cfg.Depth {
		rect := cfg.Images[0].Bounds()
		rt.depth = [2]*image.Gray16{image.NewGray16(rect), image.NewGray16(rect)}
	}

	for i := 0; i < numCPU; i++ {
		go rt.workerLoop()
	}

	return rt
}
