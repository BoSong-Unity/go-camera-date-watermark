package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"io"
	"io/ioutil"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/BurntSushi/graphics-go/graphics"
	"github.com/golang/freetype"
	"github.com/golang/freetype/truetype"
	"github.com/rwcarlsen/goexif/exif"
)

var f *truetype.Font
var inputPath = flag.String("path", "./", "inputpath")

func main() {
	flag.Parse()
	path := *inputPath
	if !strings.HasSuffix(path, "/") {
		path = path + "/"
	}

	var wg = sync.WaitGroup{}
	maxGoroutines := 30
	guard := make(chan struct{}, maxGoroutines)

	fs, _ := ioutil.ReadDir(path)
	for _, file := range fs {
		extension := strings.ToLower(filepath.Ext(path + file.Name()))
		if !file.IsDir() && (extension == ".jpg" || extension == ".jpeg") {
			wg.Add(1)
			guard <- struct{}{}

			fmt.Println(path + file.Name())
			go func(name string, path string) {
				err := addTimestampWaterMark(name, path)
				if err != nil {
					fmt.Println(err)
				}

				<-guard
				wg.Done()
			}(file.Name(), path)
		}
	}

	wg.Wait()
}

func addTimestampWaterMark(name string, rootPath string) error {
	path := rootPath + name
	// 1. load pictures
	ef, err := os.Open(path)

	if err != nil {
		return err
	}
	defer ef.Close()

	// 2. load exif timestamp
	x, err := exif.Decode(ef)
	if err != nil {
		return err
	}
	tm, _ := x.DateTime()
	var otag int64
	orientation, err := x.Get(exif.Orientation)
	if err != nil {
		fmt.Println(err)
	} else {
		otag, err = orientation.Int64(0)
		if err != nil {
			fmt.Println(err)
			otag = 0
		}
	}

	fmt.Println("Taken: ", tm, otag)

	// 3. load width/height
	vf, err := os.Open(path)
	if err != nil {
		return err
	}
	defer vf.Close()
	imgConfig, _, err := image.DecodeConfig(vf)
	if err != nil {
		return err
	}
	if _, err := vf.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("cannot seek: %s", err)
	}
	img, _, err := image.Decode(vf)
	if err != nil || img == nil {
		return err
	}
	width := imgConfig.Width
	height := imgConfig.Height
	if (otag == 8 || otag == 6) && width > height {
		width, height = height, width
		srcDim := img.Bounds()
		dstImage := image.NewRGBA(image.Rect(0, 0, srcDim.Dy(), srcDim.Dx()))
		if otag == 6 {
			graphics.Rotate(dstImage, img, &graphics.RotateOptions{math.Pi / 2.0})
		} else {
			graphics.Rotate(dstImage, img, &graphics.RotateOptions{math.Pi * 1.5})
		}
		img = dstImage
	}
	posX := 3 * width / 4
	posY := 12 * height / 13
	var fontSize float64
	fontSize = float64(140.0 * width / 4000.0)
	fmt.Println(width)
	fmt.Println(height)
	if width > height {
		posX = 5 * width / 6
		posY = 9 * height / 10
		fontSize = float64(140.0 * width / 6000.0)
	}

	// 4. add timestamp label to the pic
	imageRGBA := imageToNRGBA(img)
	fmt.Println(posX, posY, fontSize)
	if tm.After(time.Now().Add(-30 * 365 * 24 * time.Hour)) {
		addLabel(imageRGBA, posX, posY, tm.Format("2006-01-02"), fontSize)
	}

	// 5. export the pic
	outputPath := rootPath + "output/"
	outputName := outputPath + name
	if _, err := os.Stat(outputPath); os.IsNotExist(err) {
		os.MkdirAll(outputPath, 0700)
	}
	out, err := os.Create(outputName)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	defer out.Close()
	var opt jpeg.Options
	opt.Quality = 100
	err = jpeg.Encode(out, imageRGBA, &opt)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	fmt.Println("Generated image to " + outputName)

	return nil
}

func imageToNRGBA(im image.Image) *image.NRGBA {
	dst := image.NewNRGBA(im.Bounds())
	draw.Draw(dst, im.Bounds(), im, im.Bounds().Min, draw.Src)
	return dst
}

func addLabel(img *image.NRGBA, x, y int, label string, fontSize float64) {
	col := color.NRGBA{200, 100, 0, 255}
	c := freetype.NewContext()
	c.SetDPI(72)
	c.SetFont(f)
	c.SetFontSize(fontSize)
	c.SetClip(img.Bounds())
	c.SetSrc(image.NewUniform(col))
	c.SetDst(img)
	pt := freetype.Pt(x, y+int(c.PointToFixed(fontSize)>>6))

	if _, err := c.DrawString(label, pt); err != nil {
		fmt.Println(err)
	}
}

func init() {
	fontBytes, err := ioutil.ReadFile("./OpenSans-Bold.ttf")
	if err != nil {
		fmt.Println(err)
		return
	}
	f, err = freetype.ParseFont(fontBytes)
	if err != nil {
		fmt.Println(err)
		return
	}
}
