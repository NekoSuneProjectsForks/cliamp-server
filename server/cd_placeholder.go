package server

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"math"
	"net/http"
	"sync"
)

const discArtSize = 390

var discArtOnce sync.Once
var discArtPNG []byte

// cdJewelCasePNG creates a fixed, original CD-in-a-clear-case image. It uses
// only the Go standard library and is rasterized once per server process.
func cdJewelCasePNG() []byte {
	discArtOnce.Do(func() {
		img := image.NewNRGBA(image.Rect(0, 0, discArtSize, discArtSize))
		for y := 0; y < discArtSize; y++ {
			for x := 0; x < discArtSize; x++ {
				img.SetNRGBA(x, y, color.NRGBA{R: 8, G: 22, B: 33, A: 255})
			}
		}
		// Jewel case back tray and transparent case edges.
		cdRect(img, 20, 24, 370, 364, color.NRGBA{142, 183, 193, 255})
		cdRect(img, 24, 28, 366, 360, color.NRGBA{15, 34, 47, 255})
		cdRect(img, 25, 29, 60, 359, color.NRGBA{36, 91, 91, 255})
		cdRect(img, 29, 34, 34, 354, color.NRGBA{132, 198, 196, 255})
		cdRect(img, 55, 30, 58, 357, color.NRGBA{6, 30, 40, 255})
		cdRect(img, 64, 29, 66, 358, color.NRGBA{130, 212, 202, 255})
		// The disc itself: iridescent surface, concentric grooves and reflective arc.
		const cx, cy = 208.0, 185.0
		for y := 40; y < 326; y++ {
			for x := 64; x < 352; x++ {
				dx, dy := float64(x)-cx, float64(y)-cy
				r := math.Hypot(dx, dy)
				if r > 125 {
					continue
				}
				angle := math.Atan2(dy, dx)
				glow := math.Sin(0.07*r+1.6*angle)
				var c color.NRGBA
				switch {
				case r < 13:
					c = color.NRGBA{R: 10, G: 25, B: 34, A: 255}
				case r < 46:
					c = color.NRGBA{R: 26, G: 61, B: 72, A: 255}
				default:
					v := 147.0 + 55*glow + 21*math.Sin(r*0.38)
					c = color.NRGBA{
						R: cdClamp(v+22*math.Sin(angle*3)),
						G: cdClamp(v+21*math.Sin(angle*2.7+2)),
						B: cdClamp(v+45*math.Sin(angle*3.3+4)),
						A: 255,
					}
					if r > 118 {
						c = color.NRGBA{R: 215, G: 242, B: 239, A: 255}
					}
				}
				img.SetNRGBA(x, y, c)
			}
		}
		// Recessed disc ring and centre pin.
		cdRing(img, 208, 185, 44, 47, color.NRGBA{R: 145, G: 233, B: 198, A: 255})
		cdRing(img, 208, 185, 12, 16, color.NRGBA{R: 204, G: 222, B: 227, A: 255})
		cdCircle(img, 208, 185, 9, color.NRGBA{R: 7, G: 19, B: 29, A: 255})
		// Transparent lid reflections and moulded rim.
		cdRect(img, 66, 31, 360, 34, color.NRGBA{R: 165, G: 222, B: 230, A: 255})
		cdRect(img, 66, 354, 360, 357, color.NRGBA{R: 117, G: 183, B: 193, A: 255})
		cdRect(img, 357, 37, 361, 353, color.NRGBA{R: 108, G: 175, B: 183, A: 255})
		cdRect(img, 31, 64, 53, 71, color.NRGBA{R: 7, G: 26, B: 38, A: 255})
		cdRect(img, 31, 308, 53, 315, color.NRGBA{R: 7, G: 26, B: 38, A: 255})
		cdRect(img, 344, 166, 353, 210, color.NRGBA{R: 116, G: 183, B: 192, A: 255})
		// Translucent triangular sheen suggests clear jewel-case plastic.
		for y := 38; y < 140; y++ {
			for x := 75; x < 342; x++ {
				if float64(y-38) < float64(342-x)*0.39 {
					cdBlend(img, x, y, color.NRGBA{R: 211, G: 250, B: 250, A: 20})
				}
			}
		}
		cdArtPNG, err := pngBytes(img)
		if err == nil {
			discArtPNG = cdArtPNG
		}
	})
	return discArtPNG
}

func cdClamp(v float64) uint8 {
	if v < 0 { return 0 }
	if v > 255 { return 255 }
	return uint8(v)
}

func cdRect(img *image.NRGBA, x0, y0, x1, y1 int, c color.NRGBA) {
	for y:=y0; y<y1; y++ {
		for x:=x0; x<x1; x++ { img.SetNRGBA(x,y,c) }
	}
}
func cdCircle(img *image.NRGBA, cx,cy,r int, c color.NRGBA) {
	cdRing(img,cx,cy,0,r,c)
}
func cdRing(img *image.NRGBA, cx,cy,minR,maxR int,c color.NRGBA) {
	for y:=cy-maxR; y<=cy+maxR; y++ {
		for x:=cx-maxR; x<=cx+maxR; x++ {
			d:=(x-cx)*(x-cx)+(y-cy)*(y-cy)
			if d>=minR*minR&&d<=maxR*maxR { img.SetNRGBA(x,y,c) }
		}
	}
}
func cdBlend(img *image.NRGBA,x,y int,top color.NRGBA) {
	base:=img.NRGBAAt(x,y)
	a:=int(top.A)
	img.SetNRGBA(x,y,color.NRGBA{
		R:uint8((int(base.R)*(255-a)+int(top.R)*a)/255),
		G:uint8((int(base.G)*(255-a)+int(top.G)*a)/255),
		B:uint8((int(base.B)*(255-a)+int(top.B)*a)/255),
		A:255,
	})
}
func pngBytes(img image.Image)([]byte,error) {
	var buf bytes.Buffer
	if err:=png.Encode(&buf,img);err!=nil {return nil,err}
	return buf.Bytes(),nil
}
func serveCDJewelCase(w http.ResponseWriter) {
	data:=cdJewelCasePNG()
	if len(data)==0 {http.Error(w,"CD artwork unavailable",http.StatusInternalServerError);return}
	w.Header().Set("Content-Type","image/png")
	w.Header().Set("Cache-Control","no-store")
	w.Header().Set("X-Content-Type-Options","nosniff")
	_,_=w.Write(data)
}
