package anim

import (
	"image"
	"image/color"
	"image/draw"
	"math"
)

// CutSheet cuts a sheet into frames: out[f][r] is frame f seen from
// rotation r (mirrored cells flipped), every image padded to a square with
// the content bottom-centred (sprites are drawn square, standing on their
// bottom edge).
func CutSheet(img image.Image, d *Desc) [][]*image.RGBA {
	b := img.Bounds()
	cw, ch := b.Dx()/d.Cols, b.Dy()/d.Rows
	out := make([][]*image.RGBA, d.Frames())
	for f := range out {
		out[f] = make([]*image.RGBA, d.Rotations)
		for r := range out[f] {
			col, row, mirror := d.Cell(f, r)
			src := image.Rect(b.Min.X+col*cw, b.Min.Y+row*ch, b.Min.X+(col+1)*cw, b.Min.Y+(row+1)*ch)
			out[f][r] = squareCell(img, src, mirror)
		}
	}
	return out
}

func squareCell(img image.Image, src image.Rectangle, mirror bool) *image.RGBA {
	w, h := src.Dx(), src.Dy()
	n := max(w, h)
	dst := image.NewRGBA(image.Rect(0, 0, n, n))
	ox, oy := (n-w)/2, n-h
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			sx := x
			if mirror {
				sx = w - 1 - x
			}
			dst.Set(ox+x, oy+y, img.At(src.Min.X+sx, src.Min.Y+y))
		}
	}
	return dst
}

// ToRGBA copies any image into a new RGBA image at the origin.
func ToRGBA(img image.Image) *image.RGBA {
	b := img.Bounds()
	out := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(out, out.Bounds(), img, b.Min, draw.Src)
	return out
}

// FakeRotations makes n placeholder view angles of a front-facing frame
// (for art that has only one view): the figure narrows as it turns
// sideways, its far side darkens, and seen from behind it is mirrored, dark
// and desaturated. Views past the half turn mirror the first half, so left
// and right match. View 0 is the frame itself.
func FakeRotations(src *image.RGBA, n int) []*image.RGBA {
	out := make([]*image.RGBA, n)
	for r := 0; r < n; r++ {
		if r > n/2 {
			out[r] = mirror(out[n-r])
			continue
		}
		out[r] = turned(src, 2*math.Pi*float64(r)/float64(n))
	}
	return out
}

func turned(src *image.RGBA, a float64) *image.RGBA {
	if a == 0 {
		return clone(src)
	}
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	c, s := math.Cos(a), math.Sin(a)
	sx := 0.55 + 0.45*math.Abs(c) // width as the figure turns
	back := c < -0.2
	body := averageColor(src)
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	half := float64(w) / 2
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			u := (float64(x) + 0.5 - half) / (half * sx)
			if u < -1 || u >= 1 {
				continue
			}
			srcX := int((u + 1) * half)
			if back {
				srcX = w - 1 - srcX
			}
			p := src.RGBAAt(b.Min.X+min(max(srcX, 0), w-1), b.Min.Y+y)
			if p.A == 0 {
				continue
			}
			k := 1 - 0.4*s*u // the side turned away is darker
			if back {
				// Seen from behind: the body colour with a little of the
				// original shading (no face, no eyes), darker.
				k *= 0.6
				g := (float64(p.R) + float64(p.G) + float64(p.B)) / 3 / 255
				shade := 0.75 + 0.5*math.Min(g, 0.5)
				p.R = uint8(math.Min(255, float64(body.R)*shade))
				p.G = uint8(math.Min(255, float64(body.G)*shade))
				p.B = uint8(math.Min(255, float64(body.B)*shade))
			}
			dst.SetRGBA(x, y, scale(p, k))
		}
	}
	return dst
}

// FakeDeath makes n placeholder death frames from a frame: the figure
// sinks and flattens towards the floor and darkens, ending as a corpse.
func FakeDeath(src *image.RGBA, n int) []*image.RGBA {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	out := make([]*image.RGBA, n)
	for i := range out {
		t := float64(i+1) / float64(n)
		sy := 1 - 0.8*t  // height left
		sx := 1 + 0.35*t // spreads a little
		k := 1 - 0.45*t  // darker
		red := 0.25 * t  // bloodier
		dst := image.NewRGBA(image.Rect(0, 0, w, h))
		half := float64(w) / 2
		for y := 0; y < h; y++ {
			v := 1 - (float64(h)-float64(y)-0.5)/(float64(h)*sy) // 1 at the bottom
			if v < 0 {
				continue
			}
			srcY := int(v * float64(h))
			for x := 0; x < w; x++ {
				u := (float64(x) + 0.5 - half) / (half * sx)
				if u < -1 || u >= 1 {
					continue
				}
				p := src.RGBAAt(b.Min.X+int((u+1)*half), b.Min.Y+min(srcY, h-1))
				if p.A == 0 {
					continue
				}
				q := scale(p, k)
				q.R = uint8(math.Min(255, float64(q.R)+red*255*float64(p.A)/255))
				dst.SetRGBA(x, y, q)
			}
		}
		out[i] = dst
	}
	return out
}

func scale(p color.RGBA, k float64) color.RGBA {
	f := func(v uint8) uint8 { return uint8(math.Min(255, math.Max(0, float64(v)*k))) }
	return color.RGBA{f(p.R), f(p.G), f(p.B), p.A}
}

func clone(src *image.RGBA) *image.RGBA {
	dst := image.NewRGBA(image.Rect(0, 0, src.Bounds().Dx(), src.Bounds().Dy()))
	draw.Draw(dst, dst.Bounds(), src, src.Bounds().Min, draw.Src)
	return dst
}

func mirror(src *image.RGBA) *image.RGBA {
	b := src.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			dst.SetRGBA(b.Dx()-1-x, y, src.RGBAAt(b.Min.X+x, b.Min.Y+y))
		}
	}
	return dst
}

// averageColor is the mean colour of the opaque pixels (the body colour).
func averageColor(img *image.RGBA) color.RGBA {
	var r, g, b, n float64
	bd := img.Bounds()
	for y := bd.Min.Y; y < bd.Max.Y; y++ {
		for x := bd.Min.X; x < bd.Max.X; x++ {
			p := img.RGBAAt(x, y)
			if p.A < 128 {
				continue
			}
			r, g, b, n = r+float64(p.R), g+float64(p.G), b+float64(p.B), n+1
		}
	}
	if n == 0 {
		return color.RGBA{}
	}
	return color.RGBA{uint8(r / n), uint8(g / n), uint8(b / n), 255}
}
