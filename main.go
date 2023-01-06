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

type vector []float64

func dotProductOfVectors(a, b vector) (vector, error) {
	if len(a) != len(b) {
		return vector{}, fmt.Errorf("dimension does not match")
	}
	c := make(vector, len(a))
	for i := range c {
		c[i] = a[i] * b[i]
	}
	return c, nil
}

func lorSummSpeed(Ax, Ay, Az, Bx, By, Bz, cc float64) (Sx, Sy, Sz float64) {
	absSpeedA := math.Sqrt(Ax*Ax + Ay*Ay + Az*Az)
	absSpeedB := math.Sqrt(Bx*Bx + By*By + Bz*Bz)
	relativisticFactor := 1.0 + absSpeedA*absSpeedB/cc
	Sx = (Ax + Bx) / relativisticFactor
	Sy = (Ay + By) / relativisticFactor
	Sz = (Az + Bz) / relativisticFactor
	return
}

type matter struct {
	mass         float64
	location     *cell
	lastLocation *cell

	biasX  float64
	speedX float64

	biasY  float64
	speedY float64
}

type cell struct {
	space             *space
	perturbationBias  float64
	perturbationSpeed float64
	weight            *matter

	lastX *cell
	nextX *cell

	lastY *cell
	nextY *cell
}

func (m *matter) calcSpeed() {
	incrX := m.lastLocation.nextX.perturbationBias - m.lastLocation.lastX.perturbationBias
	incrY := m.lastLocation.nextY.perturbationBias - m.lastLocation.lastY.perturbationBias

	m.speedX, m.speedY, m.lastLocation.perturbationSpeed = lorSummSpeed(m.speedX, m.speedY, m.lastLocation.perturbationSpeed, incrX, incrY, 0, m.lastLocation.space.speedOfLightSquared)
}

func (m *matter) calcBias() {
	m.biasX += m.speedX
	m.biasY += m.speedY
}

func (m *matter) leap() {
	distanation := m.location
	switch {
	case m.biasX > 0.5:
		m.biasX -= 1
		distanation = distanation.nextX
	case m.biasX < -0.5:
		m.biasX += 1
		distanation = distanation.lastX
	}
	switch {
	case m.biasY > 0.5:
		m.biasY -= 1
		distanation = distanation.nextY
	case m.biasY < -0.5:
		m.biasY += 1
		distanation = distanation.lastY
	}

	m.lastLocation = m.location

	if distanation != m.location {
		m.location.weight = nil
		m.location = distanation
		m.location.weight = m
	}
}

func (c *cell) calcPerturbationBias() {
	c.perturbationBias += c.perturbationSpeed
}

func (c *cell) calcPerturbationSpeed() {
	var force float64

	force += c.lastX.perturbationBias - c.perturbationBias
	force += c.nextX.perturbationBias - c.perturbationBias

	force += c.lastY.perturbationBias - c.perturbationBias
	force += c.nextY.perturbationBias - c.perturbationBias

	acseleration := force * c.space.speedOfLightSquared

	if c.weight == nil {
		_, _, c.perturbationSpeed = lorSummSpeed(0, 0, c.perturbationSpeed, 0, 0, acseleration, c.space.speedOfLightSquared)
	} else {
		c.weight.speedX, c.weight.speedY, c.perturbationSpeed = lorSummSpeed(c.weight.speedX, c.weight.speedY, c.perturbationSpeed, 0, 0, acseleration, c.space.speedOfLightSquared)
		c.weight.speedX, c.weight.speedY, c.perturbationSpeed = lorSummSpeed(c.weight.speedX, c.weight.speedY, c.perturbationSpeed, 0, 0, c.weight.mass, c.space.speedOfLightSquared)
	}
}

func (c *cell) getColor() color.Color {
	if c.weight != nil {
		return color.Gray{Y: 255}
	}
	return color.Gray{Y: uint8(c.perturbationBias)}
}

type space struct {
	field               [][]*cell
	sizeX               int
	sizeY               int
	speedOfLightSquared float64
	extInfluence        func(step int)
	paletted            *image.Paletted
}

type dot struct {
	x int
	y int
}

func (s *space) perturbationSpeedCalcer(dots <-chan dot, wg *sync.WaitGroup) {
	for dot := range dots {
		s.field[dot.x][dot.y].calcPerturbationSpeed()
		wg.Done()
	}
}

func (s *space) weightSpeedCalcer(dots <-chan dot, wg *sync.WaitGroup) {
	for dot := range dots {
		if weight := s.field[dot.x][dot.y].weight; weight != nil {
			weight.calcSpeed()
		}
		wg.Done()
	}
}

func (s *space) perturbationBiasCalcer(dots <-chan dot, wg *sync.WaitGroup) {
	for dot := range dots {
		s.field[dot.x][dot.y].calcPerturbationBias()
		wg.Done()
	}
}

func (s *space) weightBiasCalcer(dots <-chan dot, wg *sync.WaitGroup) {
	for dot := range dots {
		if weight := s.field[dot.x][dot.y].weight; weight != nil {
			weight.calcBias()
		}
		wg.Done()
	}
}

func (s *space) leapCalcer(dots <-chan dot, wg *sync.WaitGroup) {
	for dot := range dots {
		if weight := s.field[dot.x][dot.y].weight; weight != nil {
			weight.leap()
		}
		wg.Done()
	}
}

func (s *space) pixelCalcer(dots <-chan dot, wg *sync.WaitGroup) {
	for dot := range dots {
		color := s.field[dot.x][dot.y].getColor()
		s.paletted.Set(dot.x, dot.y, color)
		wg.Done()
	}
}

func (s *space) simulate(frames, stepsPerFrame, calcers int) *gif.GIF {

	wg := &sync.WaitGroup{}
	count := s.sizeX * s.sizeY

	perturbationSpeed := make(chan dot, calcers)
	defer close(perturbationSpeed)
	for i := 0; i < calcers; i++ {
		go s.perturbationSpeedCalcer(perturbationSpeed, wg)
	}

	perturbationBias := make(chan dot, calcers)
	defer close(perturbationBias)
	for i := 0; i < calcers; i++ {
		go s.perturbationBiasCalcer(perturbationBias, wg)
	}

	weightSpeed := make(chan dot, calcers)
	defer close(weightSpeed)
	for i := 0; i < calcers; i++ {
		go s.weightSpeedCalcer(weightSpeed, wg)
	}

	weightBias := make(chan dot, calcers)
	defer close(weightBias)
	for i := 0; i < calcers; i++ {
		go s.weightBiasCalcer(weightBias, wg)
	}

	leap := make(chan dot, calcers)
	defer close(leap)
	for i := 0; i < calcers; i++ {
		go s.leapCalcer(leap, wg)
	}

	pixel := make(chan dot, calcers)
	defer close(pixel)
	for i := 0; i < calcers; i++ {
		go s.pixelCalcer(pixel, wg)
	}

	pallette := make([]color.Color, 0, 256)
	for i := 0; i < 256; i++ {
		pallette = append(pallette, color.RGBA{0, uint8(i / 2), uint8(i), 255})
	}
	images := make([]*image.Paletted, 0, frames)

	// wave generation
	steps := stepsPerFrame * frames
	for i := 0; i < steps; i++ {
		// speeds adjustment by external influence
		s.extInfluence(i)

		wg.Add(count)
		for x := 0; x < s.sizeX; x++ {
			for y := 0; y < s.sizeY; y++ {
				perturbationSpeed <- dot{x: x, y: y}
			}
		}
		wg.Wait()

		wg.Add(count)
		for x := 0; x < s.sizeX; x++ {
			for y := 0; y < s.sizeY; y++ {
				perturbationBias <- dot{x: x, y: y}
			}
		}
		wg.Wait()

		wg.Add(count)
		for x := 0; x < s.sizeX; x++ {
			for y := 0; y < s.sizeY; y++ {
				weightSpeed <- dot{x: x, y: y}
			}
		}
		wg.Wait()

		wg.Add(count)
		for x := 0; x < s.sizeX; x++ {
			for y := 0; y < s.sizeY; y++ {
				weightBias <- dot{x: x, y: y}
			}
		}
		wg.Wait()

		wg.Add(count)
		for x := 0; x < s.sizeX; x++ {
			for y := 0; y < s.sizeY; y++ {
				leap <- dot{x: x, y: y}
			}
		}
		wg.Wait()

		// heights snapshot generation
		if i%stepsPerFrame == 0 {
			s.paletted = image.NewPaletted(image.Rect(0, 0, s.sizeX, s.sizeY), pallette)
			wg.Add(count)
			for x := 0; x < s.sizeX; x++ {
				for y := 0; y < s.sizeY; y++ {
					pixel <- dot{x: x, y: y}
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

func newSpace(sizeX int, sizeY int, speedOfLight float64) *space {
	s := &space{
		sizeX:               sizeX,
		sizeY:               sizeY,
		speedOfLightSquared: speedOfLight * speedOfLight,
	}

	s.field = make([][]*cell, sizeX)
	for x := range s.field {
		s.field[x] = make([]*cell, sizeY)
		for y := range s.field[x] {
			s.field[x][y] = &cell{}
			s.field[x][y].space = s
		}
	}

	for x := 0; x < sizeX; x++ {
		for y := 0; y < sizeY; y++ {
			loop := func(val, size int) int {
				switch {
				case val < 0:
					val += size
				case val >= size:
					val -= size
				}
				return val
			}

			lastY := loop(y-1, sizeY)
			s.field[x][y].lastY = s.field[x][lastY]

			nextY := loop(y+1, sizeY)
			s.field[x][y].nextY = s.field[x][nextY]

			lastX := loop(x-1, sizeX)
			s.field[x][y].lastX = s.field[lastX][y]

			nextX := loop(x+1, sizeX)
			s.field[x][y].nextX = s.field[nextX][y]
		}
	}
	return s
}

func main() {
	t := time.Now()

	s := newSpace(501, 501, 0.2)

	s.field[250][250].weight = &matter{
		location:     s.field[250][250],
		lastLocation: s.field[250][250],
		speedX:       0.05,
		mass:         0.5,
	}

	// внешнее воздействие
	s.extInfluence = func(step int) {
		//s.field[100][250].perturbationSpeed += 10 * math.Sin(float64(step)*2.0*math.Pi/50)
		//s.field[400][250].perturbationSpeed += 10 * math.Sin(float64(step)*2.0*math.Pi/50)
	}

	g := s.simulate(10, 20, 60)

	file, err := os.Create("image.gif")
	if err != nil {
		return
	}
	defer file.Close()

	gif.EncodeAll(file, g)

	fmt.Println(time.Since(t))
}
