// draw.Draw functions for BGRA.
// Mostly copied from http://golang.org/src/pkg/image/draw/draw.go
package bgra

import (
	"image"
	"image/color"
	"image/draw"
)

const m = 1<<16 - 1

type Image interface {
	image.Image
	Set(x, y int, c color.Color)
}

func Draw(dst draw.Image, r image.Rectangle, src image.Image, sp image.Point, op draw.Op) {
	DrawMask(dst, r, src, sp, nil, image.ZP, op)
}

func clip(dst draw.Image, r *image.Rectangle, src image.Image, sp *image.Point, mask image.Image, mp *image.Point) {
	orig := r.Min
	*r = r.Intersect(dst.Bounds())
	*r = r.Intersect(src.Bounds().Add(orig.Sub(*sp)))
	if mask != nil {
		*r = r.Intersect(mask.Bounds().Add(orig.Sub(*mp)))
	}
	dx := r.Min.X - orig.X
	dy := r.Min.Y - orig.Y
	if dx == 0 && dy == 0 {
		return
	}
	(*sp).X += dx
	(*sp).Y += dy
	(*mp).X += dx
	(*mp).Y += dy
}

func DrawMask(dst draw.Image, r image.Rectangle, src image.Image, sp image.Point, mask image.Image, mp image.Point, op draw.Op) {
	clip(dst, &r, src, &sp, mask, &mp)
	if r.Empty() {
		return
	}

	if dst0, ok := dst.(*BGRA); ok {
		if op == draw.Over {
			if mask == nil {
				switch src0 := src.(type) {
				case *image.Uniform:
					drawFillOver(dst0, r, src0)
					return
				case *image.RGBA:
					drawRGBAOver(dst0, r, src0, sp)
					return
				case *BGRA:
					drawCopyOver(dst0, r, src0, sp)
					return
				}
			} else if mask0, ok := mask.(*image.Alpha); ok {
				switch src0 := src.(type) {
				case *image.Uniform:
					drawGlyphOver(dst0, r, src0, mask0, mp)
					return
				}
			}
		} else {
			if mask == nil {
				switch src0 := src.(type) {
				case *image.Uniform:
					drawFillSrc(dst0, r, src0)
					return
				case *image.RGBA:
					drawRGBASrc(dst0, r, src0, sp)
					return
				case *BGRA:
					drawCopySrc(dst0, r, src0, sp)
					return
				}
			}
		}
		drawBGRA(dst0, r, src, sp, mask, mp, op)
		return
	} else if dst0, ok := dst.(*image.RGBA); ok {
		if src0, ok := dst.(*BGRA); ok {
			if op == draw.Over {
				if mask == nil {
					drawRGBADstOver(dst0, r, src0, sp)
					return
				}
			} else {
				if mask == nil {
					drawRGBADst(dst0, r, src0, sp)
					return
				}
			}
		}
	}
	draw.DrawMask(dst, r, src, sp, mask, mp, op)
}

func drawFillOver(dst *BGRA, r image.Rectangle, src *image.Uniform) {
	sr, sg, sb, sa := src.RGBA()
	// The 0x101 is here for the same reason as in drawRGBA.
	a := (m - sa) * 0x101
	i0 := dst.PixOffset(r.Min.X, r.Min.Y)
	i1 := i0 + r.Dx()*4
	for y := r.Min.Y; y != r.Max.Y; y++ {
		for i := i0; i < i1; i += 4 {
			dr := uint32(dst.Pix[i+2])
			dg := uint32(dst.Pix[i+1])
			db := uint32(dst.Pix[i+0])
			da := uint32(dst.Pix[i+3])

			dst.Pix[i+2] = uint8((dr*a/m + sr) >> 8)
			dst.Pix[i+1] = uint8((dg*a/m + sg) >> 8)
			dst.Pix[i+0] = uint8((db*a/m + sb) >> 8)
			dst.Pix[i+3] = uint8((da*a/m + sa) >> 8)
		}
		i0 += dst.Stride
		i1 += dst.Stride
	}
}

func drawFillSrc(dst *BGRA, r image.Rectangle, src *image.Uniform) {
	sr, sg, sb, sa := src.RGBA()
	// The built-in copy function is faster than a straightforward for loop to fill the destination with
	// the color, but copy requires a slice source. We therefore use a for loop to fill the first row, and
	// then use the first row as the slice source for the remaining rows.
	i0 := dst.PixOffset(r.Min.X, r.Min.Y)
	i1 := i0 + r.Dx()*4
	for i := i0; i < i1; i += 4 {
		dst.Pix[i+2] = uint8(sr >> 8)
		dst.Pix[i+1] = uint8(sg >> 8)
		dst.Pix[i+0] = uint8(sb >> 8)
		dst.Pix[i+3] = uint8(sa >> 8)
	}
	firstRow := dst.Pix[i0:i1]
	for y := r.Min.Y + 1; y < r.Max.Y; y++ {
		i0 += dst.Stride
		i1 += dst.Stride
		copy(dst.Pix[i0:i1], firstRow)
	}
}

func drawRGBAOver(dst *BGRA, r image.Rectangle, src *image.RGBA, sp image.Point) {
	dx, dy := r.Dx(), r.Dy()
	d0 := dst.PixOffset(r.Min.X, r.Min.Y)
	s0 := src.PixOffset(sp.X, sp.Y)
	var (
		ddelta, sdelta int
		i0, i1, idelta int
	)
	if r.Min.Y < sp.Y || r.Min.Y == sp.Y && r.Min.X <= sp.X {
		ddelta = dst.Stride
		sdelta = src.Stride
		i0, i1, idelta = 0, dx*4, +4
	} else {
		// If the source start point is higher than the destination start point, or equal height but to the left,
		// then we compose the rows in right-to-left, bottom-up order instead of left-to-right, top-down.
		d0 += (dy - 1) * dst.Stride
		s0 += (dy - 1) * src.Stride
		ddelta = -dst.Stride
		sdelta = -src.Stride
		i0, i1, idelta = (dx-1)*4, -4, -4
	}
	for ; dy > 0; dy-- {
		dpix := dst.Pix[d0:]
		spix := src.Pix[s0:]
		for i := i0; i != i1; i += idelta {
			sr := uint32(spix[i+0]) * 0x101
			sg := uint32(spix[i+1]) * 0x101
			sb := uint32(spix[i+2]) * 0x101
			sa := uint32(spix[i+3]) * 0x101

			dr := uint32(dpix[i+2])
			dg := uint32(dpix[i+1])
			db := uint32(dpix[i+0])
			da := uint32(dpix[i+3])

			// The 0x101 is here for the same reason as in drawRGBA.
			a := (m - sa) * 0x101

			dpix[i+2] = uint8((dr*a/m + sr) >> 8)
			dpix[i+1] = uint8((dg*a/m + sg) >> 8)
			dpix[i+0] = uint8((db*a/m + sb) >> 8)
			dpix[i+3] = uint8((da*a/m + sa) >> 8)
		}
		d0 += ddelta
		s0 += sdelta
	}
}

func drawRGBASrc(dst *BGRA, r image.Rectangle, src *image.RGBA, sp image.Point) {
	i0 := (r.Min.X - dst.Rect.Min.X) * 4
	i1 := (r.Max.X - dst.Rect.Min.X) * 4
	si0 := (sp.X - src.Rect.Min.X) * 4
	yMax := r.Max.Y - dst.Rect.Min.Y

	y := r.Min.Y - dst.Rect.Min.Y
	sy := sp.Y - src.Rect.Min.Y
	for ; y != yMax; y, sy = y+1, sy+1 {
		dpix := dst.Pix[y*dst.Stride:]
		spix := src.Pix[sy*src.Stride:]

		for i, si := i0, si0; i < i1; i, si = i+4, si+4 {
			dpix[i+0] = spix[si+2]
			dpix[i+1] = spix[si+1]
			dpix[i+2] = spix[si+0]
			dpix[i+3] = spix[si+3]
		}
	}
}

func drawCopyOver(dst *BGRA, r image.Rectangle, src *BGRA, sp image.Point) {
	dx, dy := r.Dx(), r.Dy()
	d0 := dst.PixOffset(r.Min.X, r.Min.Y)
	s0 := src.PixOffset(sp.X, sp.Y)
	var (
		ddelta, sdelta int
		i0, i1, idelta int
	)
	if r.Min.Y < sp.Y || r.Min.Y == sp.Y && r.Min.X <= sp.X {
		ddelta = dst.Stride
		sdelta = src.Stride
		i0, i1, idelta = 0, dx*4, +4
	} else {
		// If the source start point is higher than the destination start point, or equal height but to the left,
		// then we compose the rows in right-to-left, bottom-up order instead of left-to-right, top-down.
		d0 += (dy - 1) * dst.Stride
		s0 += (dy - 1) * src.Stride
		ddelta = -dst.Stride
		sdelta = -src.Stride
		i0, i1, idelta = (dx-1)*4, -4, -4
	}
	for ; dy > 0; dy-- {
		dpix := dst.Pix[d0:]
		spix := src.Pix[s0:]
		for i := i0; i != i1; i += idelta {
			sr := uint32(spix[i+2]) * 0x101
			sg := uint32(spix[i+1]) * 0x101
			sb := uint32(spix[i+0]) * 0x101
			sa := uint32(spix[i+3]) * 0x101

			dr := uint32(dpix[i+2])
			dg := uint32(dpix[i+1])
			db := uint32(dpix[i+0])
			da := uint32(dpix[i+3])

			// The 0x101 is here for the same reason as in drawRGBA.
			a := (m - sa) * 0x101

			dpix[i+2] = uint8((dr*a/m + sr) >> 8)
			dpix[i+1] = uint8((dg*a/m + sg) >> 8)
			dpix[i+0] = uint8((db*a/m + sb) >> 8)
			dpix[i+3] = uint8((da*a/m + sa) >> 8)
		}
		d0 += ddelta
		s0 += sdelta
	}
}

func drawCopySrc(dst *BGRA, r image.Rectangle, src *BGRA, sp image.Point) {
	n, dy := 4*r.Dx(), r.Dy()
	d0 := dst.PixOffset(r.Min.X, r.Min.Y)
	s0 := src.PixOffset(sp.X, sp.Y)
	var ddelta, sdelta int
	if r.Min.Y <= sp.Y {
		ddelta = dst.Stride
		sdelta = src.Stride
	} else {
		// If the source start point is higher than the destination start point, then we compose the rows
		// in bottom-up order instead of top-down. Unlike the drawCopydraw.Over function, we don't have to
		// check the x co-ordinates because the built-in copy function can handle overlapping slices.
		d0 += (dy - 1) * dst.Stride
		s0 += (dy - 1) * src.Stride
		ddelta = -dst.Stride
		sdelta = -src.Stride
	}
	for ; dy > 0; dy-- {
		copy(dst.Pix[d0:d0+n], src.Pix[s0:s0+n])
		d0 += ddelta
		s0 += sdelta
	}
}

func drawGlyphOver(dst *BGRA, r image.Rectangle, src *image.Uniform, mask *image.Alpha, mp image.Point) {
	i0 := dst.PixOffset(r.Min.X, r.Min.Y)
	i1 := i0 + r.Dx()*4
	mi0 := mask.PixOffset(mp.X, mp.Y)
	sr, sg, sb, sa := src.RGBA()
	for y, my := r.Min.Y, mp.Y; y != r.Max.Y; y, my = y+1, my+1 {
		for i, mi := i0, mi0; i < i1; i, mi = i+4, mi+1 {
			ma := uint32(mask.Pix[mi])
			if ma == 0 {
				continue
			}
			ma |= ma << 8

			dr := uint32(dst.Pix[i+2])
			dg := uint32(dst.Pix[i+1])
			db := uint32(dst.Pix[i+0])
			da := uint32(dst.Pix[i+3])

			// The 0x101 is here for the same reason as in drawRGBA.
			a := (m - (sa * ma / m)) * 0x101

			dst.Pix[i+2] = uint8((dr*a + sr*ma) / m >> 8)
			dst.Pix[i+1] = uint8((dg*a + sg*ma) / m >> 8)
			dst.Pix[i+0] = uint8((db*a + sb*ma) / m >> 8)
			dst.Pix[i+3] = uint8((da*a + sa*ma) / m >> 8)
		}
		i0 += dst.Stride
		i1 += dst.Stride
		mi0 += mask.Stride
	}
}

func drawBGRA(dst *BGRA, r image.Rectangle, src image.Image, sp image.Point, mask image.Image, mp image.Point, op draw.Op) {
	x0, x1, dx := r.Min.X, r.Max.X, 1
	y0, y1, dy := r.Min.Y, r.Max.Y, 1
	if image.Image(dst) == src && r.Overlaps(r.Add(sp.Sub(r.Min))) {
		if sp.Y < r.Min.Y || sp.Y == r.Min.Y && sp.X < r.Min.X {
			x0, x1, dx = x1-1, x0-1, -1
			y0, y1, dy = y1-1, y0-1, -1
		}
	}

	sy := sp.Y + y0 - r.Min.Y
	my := mp.Y + y0 - r.Min.Y
	sx0 := sp.X + x0 - r.Min.X
	mx0 := mp.X + x0 - r.Min.X
	sx1 := sx0 + (x1 - x0)
	i0 := dst.PixOffset(x0, y0)
	di := dx * 4
	for y := y0; y != y1; y, sy, my = y+dy, sy+dy, my+dy {
		for i, sx, mx := i0, sx0, mx0; sx != sx1; i, sx, mx = i+di, sx+dx, mx+dx {
			ma := uint32(m)
			if mask != nil {
				_, _, _, ma = mask.At(mx, my).RGBA()
			}
			sr, sg, sb, sa := src.At(sx, sy).RGBA()
			if op == draw.Over {
				dr := uint32(dst.Pix[i+2])
				dg := uint32(dst.Pix[i+1])
				db := uint32(dst.Pix[i+0])
				da := uint32(dst.Pix[i+3])

				// dr, dg, db and da are all 8-bit color at the moment, ranging in [0,255].
				// We work in 16-bit color, and so would normally do:
				// dr |= dr << 8
				// and similarly for dg, db and da, but instead we multiply a
				// (which is a 16-bit color, ranging in [0,65535]) by 0x101.
				// This yields the same result, but is fewer arithmetic operations.
				a := (m - (sa * ma / m)) * 0x101

				dst.Pix[i+2] = uint8((dr*a + sr*ma) / m >> 8)
				dst.Pix[i+1] = uint8((dg*a + sg*ma) / m >> 8)
				dst.Pix[i+0] = uint8((db*a + sb*ma) / m >> 8)
				dst.Pix[i+3] = uint8((da*a + sa*ma) / m >> 8)

			} else {
				dst.Pix[i+2] = uint8(sr * ma / m >> 8)
				dst.Pix[i+1] = uint8(sg * ma / m >> 8)
				dst.Pix[i+0] = uint8(sb * ma / m >> 8)
				dst.Pix[i+3] = uint8(sa * ma / m >> 8)
			}
		}
		i0 += dy * dst.Stride
	}
}

func drawRGBADstOver(dst *image.RGBA, r image.Rectangle, src *BGRA, sp image.Point) {
	dx, dy := r.Dx(), r.Dy()
	d0 := dst.PixOffset(r.Min.X, r.Min.Y)
	s0 := src.PixOffset(sp.X, sp.Y)
	var (
		ddelta, sdelta int
		i0, i1, idelta int
	)
	if r.Min.Y < sp.Y || r.Min.Y == sp.Y && r.Min.X <= sp.X {
		ddelta = dst.Stride
		sdelta = src.Stride
		i0, i1, idelta = 0, dx*4, +4
	} else {
		// If the source start point is higher than the destination start point, or equal height but to the left,
		// then we compose the rows in right-to-left, bottom-up order instead of left-to-right, top-down.
		d0 += (dy - 1) * dst.Stride
		s0 += (dy - 1) * src.Stride
		ddelta = -dst.Stride
		sdelta = -src.Stride
		i0, i1, idelta = (dx-1)*4, -4, -4
	}
	for ; dy > 0; dy-- {
		dpix := dst.Pix[d0:]
		spix := src.Pix[s0:]
		for i := i0; i != i1; i += idelta {
			sr := uint32(spix[i+2]) * 0x101
			sg := uint32(spix[i+1]) * 0x101
			sb := uint32(spix[i+0]) * 0x101
			sa := uint32(spix[i+3]) * 0x101

			dr := uint32(dpix[i+0])
			dg := uint32(dpix[i+1])
			db := uint32(dpix[i+2])
			da := uint32(dpix[i+3])

			// The 0x101 is here for the same reason as in drawRGBA.
			a := (m - sa) * 0x101

			dpix[i+0] = uint8((dr*a/m + sr) >> 8)
			dpix[i+1] = uint8((dg*a/m + sg) >> 8)
			dpix[i+2] = uint8((db*a/m + sb) >> 8)
			dpix[i+3] = uint8((da*a/m + sa) >> 8)
		}
		d0 += ddelta
		s0 += sdelta
	}
}

func drawRGBADst(dst *image.RGBA, r image.Rectangle, src *BGRA, sp image.Point) {
	i0 := (r.Min.X - dst.Rect.Min.X) * 4
	i1 := (r.Max.X - dst.Rect.Min.X) * 4
	si0 := (sp.X - src.Rect.Min.X) * 4
	yMax := r.Max.Y - dst.Rect.Min.Y

	y := r.Min.Y - dst.Rect.Min.Y
	sy := sp.Y - src.Rect.Min.Y
	for ; y != yMax; y, sy = y+1, sy+1 {
		dpix := dst.Pix[y*dst.Stride:]
		spix := src.Pix[sy*src.Stride:]

		for i, si := i0, si0; i < i1; i, si = i+4, si+4 {
			dpix[i+0] = spix[si+2]
			dpix[i+1] = spix[si+1]
			dpix[i+2] = spix[si+0]
			dpix[i+3] = spix[si+3]
		}
	}
}
