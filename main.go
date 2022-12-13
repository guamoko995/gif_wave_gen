package main

import (
	"fmt"
	"image"
	"image/color"
	"image/gif"
	"math"
	"os"
	"sync"
	"time"
)

type cell struct {
	height    float64
	speed     float64
	mass      float64
	gain      float64
	neighbors []*cell
}

func (c *cell) calcHeight() {
	c.height += c.speed
}

func (c *cell) calcSpeed() {
	var force float64
	for _, n := range c.neighbors {
		force += n.height - c.height
	}
	force /= float64(len(c.neighbors))
	acceleration := force / c.mass
	c.speed *= c.gain
	c.speed += acceleration
}

func (c *cell) getColor() color.Color {
	for _, n := range c.neighbors {
		if n.mass > c.mass {
			return color.Gray{
				Y: 255,
			}
		}
		if n.mass < c.mass {
			return color.Gray{
				Y: 0,
			}
		}
	}
	return color.Gray{
		Y: uint8(10*c.height + 128),
	}
}

type space struct {
	field        [][]*cell
	sizeX        int
	sizeY        int
	extInfluence func(step int)
	paletted     *image.Paletted
}

type dot struct {
	x int
	y int
}

func (s *space) speedCalcer(dots <-chan dot, wg *sync.WaitGroup) {
	for dot := range dots {
		s.field[dot.x][dot.y].calcSpeed()
		wg.Done()
	}
}

func (s *space) heightSetter(index <-chan dot, wg *sync.WaitGroup) {
	for in := range index {
		s.field[in.x][in.y].calcHeight()
		wg.Done()
	}
}

func (s *space) pixSetter(index <-chan dot, wg *sync.WaitGroup) {
	for in := range index {
		color := s.field[in.x][in.y].getColor()
		s.paletted.Set(in.x, in.y, color)
		wg.Done()
	}
}

func (s *space) simulate(frames, stepsPerFrame, speedCalcers, heightCalcers, pixSetters int) *gif.GIF {

	wg := &sync.WaitGroup{}
	count := s.sizeX * s.sizeY

	// speed calcs pool
	cellsSpeed := make(chan dot, speedCalcers)
	defer close(cellsSpeed)
	for i := 0; i < speedCalcers; i++ {
		go s.speedCalcer(cellsSpeed, wg)
	}

	// height calcers pool
	cellsHeight := make(chan dot, heightCalcers)
	defer close(cellsHeight)
	for i := 0; i < heightCalcers; i++ {
		go s.heightSetter(cellsHeight, wg)
	}

	// pix setters pool
	setPix := make(chan dot, pixSetters)
	defer close(setPix)
	for i := 0; i < pixSetters; i++ {
		go s.pixSetter(setPix, wg)
	}

	pallette := make([]color.Color, 0, 256)
	for i := 0; i < 256; i++ {
		pallette = append(pallette, color.RGBA{0, uint8(i / 2), uint8(i), 255})
	}
	images := make([]*image.Paletted, 0, frames)

	// wave generation
	steps := stepsPerFrame * frames
	for i := 0; i < steps; i++ {
		// recalc all speeds
		wg.Add(count)
		for x := 0; x < s.sizeX; x++ {
			for y := 0; y < s.sizeY; y++ {
				cellsSpeed <- dot{x: x, y: y}
			}
		}
		wg.Wait()

		// speeds adjustment by external influence
		s.extInfluence(i)

		// recalc all height
		wg.Add(count)
		for x := 0; x < s.sizeX; x++ {
			for y := 0; y < s.sizeY; y++ {
				cellsHeight <- dot{x: x, y: y}
			}
		}
		wg.Wait()

		// heights snapshot generation
		if i%stepsPerFrame == 0 {
			s.paletted = image.NewPaletted(image.Rect(0, 0, s.sizeX, s.sizeY), pallette)
			wg.Add(count)
			for x := 0; x < s.sizeX; x++ {
				for y := 0; y < s.sizeY; y++ {
					setPix <- dot{x: x, y: y}
				}
			}
			wg.Wait()
			images = append(images, s.paletted)
		}
	}

	return &gif.GIF{
		Image: images,
		Delay: make([]int, frames),
	}
}

func newSpace(sizeX int, sizeY int, cellMass float64) *space {
	s := &space{
		sizeX: sizeX,
		sizeY: sizeY,
	}

	s.field = make([][]*cell, sizeX)
	for x := range s.field {
		s.field[x] = make([]*cell, sizeY)
		for y := range s.field[x] {
			s.field[x][y] = &cell{}
			s.field[x][y].mass = cellMass
			s.field[x][y].gain = 1
		}
	}

	for x := 0; x < sizeX; x++ {
		for y := 0; y < sizeY; y++ {
			yNorth := y - 1
			if yNorth >= 0 && yNorth < sizeY {
				s.field[x][y].neighbors = append(s.field[x][y].neighbors, s.field[x][yNorth])
			}

			ySouth := y + 1
			if ySouth >= 0 && ySouth < sizeY {
				s.field[x][y].neighbors = append(s.field[x][y].neighbors, s.field[x][ySouth])
			}

			xWest := x - 1
			if xWest >= 0 && xWest < sizeX {
				s.field[x][y].neighbors = append(s.field[x][y].neighbors, s.field[xWest][y])
			}

			xEast := x + 1
			if xEast >= 0 && xEast < sizeX {
				s.field[x][y].neighbors = append(s.field[x][y].neighbors, s.field[xEast][y])
			}
		}
	}
	return s
}

func main() {
	t := time.Now()

	s := newSpace(501, 501, 10)

	// линза
	for x := 0; x < s.sizeX; x++ {
		for y := 0; y < s.sizeY; y++ {
			if (x-330)*(x-330)+(y-250)*(y-250) < 100*100 && (x-170)*(x-170)+(y-250)*(y-250) < 100*100 {
				s.field[x][y].mass *= 2
			}

			if y < 100 || 500-y < 100 {
				s.field[x][y].mass *= 0.99
				s.field[x][y].gain *= 0.99
			}

			/*w := 20
			if y < int(10*math.Abs(float64((x%w)-w/2))) || 500-y < int(10*math.Abs(float64((x%w)-w/2))) {
				s.field[x][y].mass /= 5
			}*/
		}
	}

	// внешнее воздействие
	s.extInfluence = func(step int) {
		if step < 300 {
			s.field[0][250].speed += math.Sin(float64(step) * (math.Pi / 100))
			s.field[0][251].speed += math.Sin(float64(step) * (math.Pi / 100))
		}
	}

	g := s.simulate(500, 20, 60, 60, 60)

	file, err := os.Create("image.gif")
	if err != nil {
		return
	}
	defer file.Close()

	gif.EncodeAll(file, g)

	fmt.Println(time.Since(t))
}
