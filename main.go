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

func (s *space) simulate(frames, stepsPerFrame, speedCalcers, hightSetters, pixSetters int) *gif.GIF {

	wg := &sync.WaitGroup{}
	count := s.sizeX * s.sizeY

	cellsSpeed := make(chan dot, speedCalcers)
	defer close(cellsSpeed)
	for i := 0; i < speedCalcers; i++ {
		go s.speedCalcer(cellsSpeed, wg)
	}

	cellsHight := make(chan dot, hightSetters)
	defer close(cellsHight)
	for i := 0; i < hightSetters; i++ {
		go s.heightSetter(cellsHight, wg)
	}

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
	steps := stepsPerFrame * frames
	for i := 0; i < steps; i++ {
		wg.Add(count)
		for x := 0; x < s.sizeX; x++ {
			for y := 0; y < s.sizeY; y++ {
				cellsSpeed <- dot{x: x, y: y}
			}
		}
		wg.Wait()

		s.extInfluence(i)

		wg.Add(count)
		for x := 0; x < s.sizeX; x++ {
			for y := 0; y < s.sizeY; y++ {
				cellsHight <- dot{x: x, y: y}
			}
		}
		wg.Wait()

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

	// создаем линзу с двойной массой в центре пространства
	for x := 0; x < s.sizeX; x++ {
		for y := 0; y < s.sizeY; y++ {
			if (x-330)*(x-330)+(y-250)*(y-250) < 100*100 && (x-170)*(x-170)+(y-250)*(y-250) < 100*100 {
				s.field[x][y].mass *= 2
			}
		}
	}

	s.extInfluence = func(step int) {
		if step < 250 {
			s.field[100][250].speed += 1 * math.Sin(float64(step)*(math.Pi/50))
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
