package main

import (
	"image"
	"image/color"
	"image/gif"
	"math"
	"os"
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

type space struct {
	field [][]*cell
	sizeX int
	sizeY int
}

func (s *space) run(steps int, extInfluence func(step int, s *space)) ([]*image.Paletted, []int) {
	cp := make([]color.Color, 0, 256)
	for i := 0; i < 256; i++ {
		cp = append(cp, color.RGBA{0, uint8(i / 2), uint8(i), 255})
	}
	var delays []int
	var img []*image.Paletted
	for i := 0; i < steps; i++ {
		for x := 0; x < s.sizeX; x++ {
			for y := 0; y < s.sizeY; y++ {
				s.field[x][y].calcSpeed()
			}
		}

		extInfluence(i, s)

		p := image.NewPaletted(image.Rect(0, 0, s.sizeX, s.sizeY), cp)
		for x := 0; x < s.sizeX; x++ {
			for y := 0; y < s.sizeY; y++ {
				s.field[x][y].calcHeight()
				func() {
					for _, n := range s.field[x][y].neighbors {
						if n.mass > s.field[x][y].mass {
							p.Set(x, y, color.Gray{
								Y: 255,
							})
							return
						}
						if n.mass < s.field[x][y].mass {
							p.Set(x, y, color.Gray{
								Y: 0,
							})
							return
						}
						p.Set(x, y, color.Gray{
							Y: uint8(10*s.field[x][y].height + 128),
						})
					}
				}()
			}
		}
		if i%20 == 0 {
			img = append(img, p)
			delays = append(delays, 0)
		}
	}
	return img, delays
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

	file, err := os.Create("image.gif")
	if err != nil {
		return
	}
	defer file.Close()

	s := newSpace(501, 501, 10)

	// корректируем пространство, удваивая массу пикселей, лежащих
	// внутри круга радиусом 100 с центром в x=250 y=250
	for x := 0; x < s.sizeX; x++ {
		for y := 0; y < s.sizeY; y++ {
			if (x-250)*(x-250)+(y-250)*(y-250) < 100*100 {
				s.field[x][y].mass *= 2
			}
		}
	}

	img, delays := s.run(5000, func(step int, s *space) {
		if step < 1000 {
			s.field[100][250].speed += 1 * math.Sin(float64(step)*(math.Pi/50))
		}
	})
	gif.EncodeAll(file, &gif.GIF{
		Image: img,
		Delay: delays,
	})
}
