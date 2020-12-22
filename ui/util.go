package ui

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"io/ioutil"
	"math"
	"strings"

	"fyne.io/fyne"
	"fyne.io/fyne/canvas"
	"github.com/nfnt/resize"
)

func MustLoadStaticResource(rsc string) *fyne.StaticResource {
	b, err := ioutil.ReadFile(rsc)
	if err != nil {
		panic("error loading " + err.Error())
	}

	return &fyne.StaticResource{
		StaticName:    rsc,
		StaticContent: b,
	}
}

func StringToColor(s string) color.Color {
	var c color.NRGBA
	var err error
	if strings.HasPrefix(s, "#") {
		s = s[1:]
	}
	n := len(s)
	if n == 6 {
		c.A = 0xFF
		_, err = fmt.Sscanf(s, "%02x%02x%02x", &c.R, &c.G, &c.B)
	} else if n == 8 {
		_, err = fmt.Sscanf(s, "%02x%02x%02x%02x", &c.R, &c.G, &c.B, &c.A)
	} else if n == 3 || n == 4 {
		if n == 3 {
			c.A = 0xFF
			_, err = fmt.Sscanf(s, "%01x%01x%01x", &c.R, &c.G, &c.B)
		} else {
			_, err = fmt.Sscanf(s, "%01x%01x%01x%01x", &c.R, &c.G, &c.B, &c.A)
			c.A |= (c.A << 4)
		}
		c.R |= (c.R << 4)
		c.G |= (c.G << 4)
		c.B |= (c.B << 4)
	}
	if err != nil {
		panic("Error converting color string '" + s + "' : " + err.Error())
	}
	return c
}

func NewSizedImage(rsc *fyne.StaticResource, w, h uint) *canvas.Image {
	img, _, err := image.Decode(bytes.NewReader(rsc.StaticContent))
	if err != nil {
		panic("image err " + err.Error())
	}
	ogSize := img.Bounds().Size()
	imgW, imgH := uint(ogSize.X), uint(ogSize.Y)
	needsResize := (w != 0 && w != imgW) || (h != 0 && h != imgH)

	aspect := float64(imgW) / float64(imgH)
	if w == 0 {
		if h == 0 {
			w = imgW
			h = imgH
		} else {
			w = uint(math.Round(aspect * float64(h)))
		}
	} else if h == 0 {
		h = uint(math.Round(float64(w) / aspect))
	}

	if needsResize {
		img = resize.Resize(w, h, img, resize.Lanczos3)
	}

	fyneImg := canvas.NewImageFromImage(img)
	fyneImg.FillMode = canvas.ImageFillOriginal
	fyneImg.SetMinSize(fyne.NewSize(int(w), int(h)))
	fyneImg.Resize(fyne.NewSize(int(w), int(h)))
	return fyneImg
}

func NewHorizontalRule(strokeWidth int, c color.Color, my int) *Element {
	return NewElement(&Style{
		Height:  strokeWidth,
		BgColor: c,
		Margins: FourSpec{my, 0, my, 0},
	})
}

func line(startX, startY, endX, endY int, strokeWidth float32, c color.Color) *canvas.Line {
	ln := canvas.NewLine(c)
	ln.StrokeWidth = float32(strokeWidth)
	ln.Move(fyne.NewPos(startX, startY))
	ln.Resize(fyne.NewSize(endX-startX, endY-startY))
	return ln
}

func corner(x, y, r int, c color.Color) *canvas.Circle {
	circ := canvas.NewCircle(c)
	circ.Move(fyne.NewPos(x-r, y-r))
	circ.Resize(fyne.NewSize(r*2, r*2))
	return circ
}

// There are nine shapes that make up a rounded rectangle. 4 corner circles,
// 4 side lines, and 1 center rectangle.
func roundedRectangle(w, h, offsetX, offsetY, borderRadius int, strokeWidth float32, c color.Color) []fyne.CanvasObject {
	x, y := borderRadius+offsetX, borderRadius+offsetY
	innerH := h - 2*borderRadius
	innerW := w - 2*borderRadius
	left := line(x, y, x, y+innerH, strokeWidth, c)
	topLeft := corner(x, y, borderRadius, c)
	top := line(x, y, x+innerW, y, strokeWidth, c)
	x += innerW
	topRight := corner(x, y, borderRadius, c)
	right := line(x, y, x, y+innerH, strokeWidth, c)
	y += innerH
	bottomRight := corner(x, y, borderRadius, c)
	x = borderRadius + offsetX
	bottom := line(x, y, x+innerW, y, strokeWidth, c)
	bottomLeft := corner(x, y, borderRadius, c)

	fill := canvas.NewRectangle(c)
	fill.Move(fyne.NewPos(borderRadius+offsetX, borderRadius+offsetY))
	fill.Resize(fyne.NewSize(innerW, innerH))

	return []fyne.CanvasObject{
		left, topLeft, top, topRight, right, bottomRight, bottom, bottomLeft, fill,
	}
}

func layoutItems(objects []fyne.CanvasObject) (flowing, positioned []fyne.CanvasObject) {
	for _, o := range objects {
		if !o.Visible() {
			continue
		}
		if isAbsolutelyPositioned(o) {
			positioned = append(positioned, o)
			continue
		}
		flowing = append(flowing, o)
	}
	return
}
