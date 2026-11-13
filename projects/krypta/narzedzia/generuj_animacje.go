//go:build ignore

// generuj_animacje.go writes the animated texture frames of Krypta:
// woda_0..3 (rippling water) and lawa_0..3 (glowing lava), 64x64, seamless
// in space and in time (phases advance by a quarter turn per frame).
//
//	go run projects/krypta/narzedzia/generuj_animacje.go projects/krypta/assets/textures
package main

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
)

const size = 64

func main() {
	dir := "."
	if len(os.Args) > 1 {
		dir = os.Args[1]
	}
	for f := 0; f < 4; f++ {
		ph := float64(f) / 4 * 2 * math.Pi
		write(filepath.Join(dir, fmt.Sprintf("woda_%d.png", f)), func(u, v float64) color.RGBA {
			w := 0.5*math.Sin(2*math.Pi*(2*u+v)+ph) + 0.35*math.Sin(2*math.Pi*(u-3*v)-ph) +
				0.25*math.Sin(2*math.Pi*(5*u+2*v)+2*ph)
			k := (w + 1.1) / 2.2 // 0..1
			hi := math.Pow(math.Max(0, w-0.6), 2) * 2.5
			return rgb(18+40*k+200*hi, 55+70*k+190*hi, 80+90*k+170*hi)
		})
		write(filepath.Join(dir, fmt.Sprintf("lawa_%d.png", f)), func(u, v float64) color.RGBA {
			n := 0.5*math.Sin(2*math.Pi*(u+2*v)+ph) + 0.5*math.Sin(2*math.Pi*(3*u-v)-ph) +
				0.3*math.Sin(2*math.Pi*(4*u+3*v)+ph)
			heat := math.Max(0, math.Min(1, (n+0.4)/1.6))
			crust := math.Max(0, 0.35-heat) * 2
			return rgb(255*heat+60*(1-heat)-90*crust, 200*heat*heat+20-15*crust, 40*heat*heat*heat)
		})
	}
}

func rgb(r, g, b float64) color.RGBA {
	c := func(x float64) uint8 { return uint8(math.Max(0, math.Min(255, x))) }
	return color.RGBA{c(r), c(g), c(b), 255}
}

func write(path string, fn func(u, v float64) color.RGBA) {
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			img.SetRGBA(x, y, fn(float64(x)/size, float64(y)/size))
		}
	}
	f, err := os.Create(path)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		panic(err)
	}
	fmt.Println(path)
}
