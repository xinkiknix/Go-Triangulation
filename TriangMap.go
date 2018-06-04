//TriangMap
/* application reads .SHP file and projects data onto the screen
after creating triangles for all components
All entities belonging to the same group have the same color
*/
package main

import (
	"ShpReader"
	"Triangulate"
	"flag"
	"fmt"
	"log"
	"math"
	"math/rand"
	"runtime"
	"sync"
	"time"

	"github.com/faiface/pixel"
	"github.com/faiface/pixel/imdraw"
	"github.com/faiface/pixel/pixelgl"
	"github.com/go-gl/glfw/v3.2/glfw"
	"golang.org/x/image/colornames"
)

type Sizing struct {
	ValueMinX   float64
	ValueMaxX   float64
	ValueMinY   float64
	ValueMaxY   float64
	ScreenMinX  float64
	ScreenMaxX  float64
	ScreenMinY  float64
	ScreenMaxY  float64
	ScreenRatio float64
}

var (
	r      *rand.Rand
	sizes  Sizing
	cfg    pixelgl.WindowConfig
	shapes []shpReader.ShapeData
	head   shpReader.Header
	wg     sync.WaitGroup
	done   chan bool
)

var (
	drawers []*pixel.Batch
	imd     *imdraw.IMDraw
)

var (
	src         = flag.String("ShpFile", "world.Shp", "Input shape file")
	trim        = flag.Int("TrimFactor", 0, "Trim factor: 0 does not remove coordinates, any other number trims points closer than % to previous point")
	detailColor = flag.Bool("Detail", false, "True value shows triangle details in color variation per triangle")
)

func translate(value float64, min float64, max float64, minrange float64, maxrange float64) float64 {
	return minrange + (maxrange-minrange)*((value-min)/(max-min))
}

func main() {
	flag.Parse()
	fmt.Printf("processing file:%v Trimfactor:%d\n", *src, *trim)
	runtime.GOMAXPROCS(runtime.NumCPU() * 2) //use double number of processes as queue length
	bf, err := shpReader.New(*src)
	if err != nil {
		log.Fatal(err)
	}
	head, shapes, err = shpReader.ReadPolygons(&bf)
	if err != nil {
		log.Fatal(err)
	}
	glfw.Init()
	mn := glfw.GetPrimaryMonitor()
	DisplayWidth := float64(mn.GetVideoMode().Width)
	DisplayHeight := float64(mn.GetVideoMode().Height)
	sizes = Sizing{
		ValueMaxX:   head.MaxX,
		ValueMaxY:   head.MaxY,
		ValueMinX:   head.MinX,
		ValueMinY:   head.MinY,
		ScreenRatio: (head.MaxX - head.MinX) / (head.MaxY - head.MinY),
		ScreenMinX:  50,
		ScreenMaxX:  DisplayWidth - 50,
		ScreenMinY:  50,
		ScreenMaxY:  DisplayHeight - 50,
	}
	// stay within screenbounds
	sizes.ScreenMaxY = sizes.ScreenMaxX / sizes.ScreenRatio
	if sizes.ScreenMaxY > (DisplayHeight - 50) {
		sizes.ScreenMaxY = DisplayHeight - 50
		sizes.ScreenMaxX = sizes.ScreenMaxY * sizes.ScreenRatio
	}
	if sizes.ScreenMaxX > (DisplayWidth - 50) {
		sizes.ScreenMaxX = DisplayWidth - 50
		sizes.ScreenMaxY = sizes.ScreenMaxX * sizes.ScreenRatio
	}
	cfg = pixelgl.WindowConfig{
		Title:     "ShapeFileViewer",
		Bounds:    pixel.R(10, 10, sizes.ScreenMaxX+20, sizes.ScreenMaxY+20),
		VSync:     true,
		Resizable: true,
	}
	pixelgl.Run(run)
}

func createData() {
	defer Triangulate.TimeTrack(time.Now())
	var mu sync.Mutex
	var totalsPo = 0
	r := rand.New(rand.NewSource(time.Now().Unix()))
	var lists [][]*Triangulate.Poly
	imd.Color = pixel.RGB(1, 1, 1) //border color
	for _, shape := range shapes {
		var list []*Triangulate.Poly
		for parts := 0; parts < int(shape.NumParts); parts++ {
			poly := Triangulate.NewPoly() //create new set
			for _, points := range shape.Coordinates[parts] {
				x := translate(points[0], sizes.ValueMinX, sizes.ValueMaxX, sizes.ScreenMinX, sizes.ScreenMaxX)
				y := translate(points[1], sizes.ValueMinY, sizes.ValueMaxY, sizes.ScreenMinX, sizes.ScreenMaxY)
				poly.PushBack(Triangulate.Point{"-", x, y}, float64(*trim)) //*trim 0 = no simplification, 1200 is arbitrary value that seems to workd for complex models
				//Contours
				imd.Push(pixel.V(x, y))
				totalsPo++
			}
			//Contours
			imd.Line(0.5)
			list = append(list, poly)
		}
		lists = append(lists, list)
	}
	done <- true // prevents run loop from atempting to draw empty imd, which causes Panic
	var jobs []chan int
	var timespent []chan int64
	for _, list := range lists {
		job := make(chan int)
		jobs = append(jobs, job)
		timeC := make(chan int64)
		timespent = append(timespent, timeC)
		colorBase := r.Float64()
		color := pixel.RGB(colorBase, 0.3+colorBase, 0.5+colorBase)
		go func(list []*Triangulate.Poly, job chan<- int, timeC chan int64) {
			NumTriangles := 0
			mu.Lock()
			now := time.Now()
			mu.Unlock()
			pointcnt := 0
			for _, poly := range list {
				poly.SetClockwise()
				pointcnt += len(poly.P)
				poly.SetToLeftMost()
				//Triangels
				triangles, err := Triangulate.GetTriangles(poly) // get all triangles to cover polygone area
				if err != nil {
					log.Println("Triangulation error", err) // non fatal error, just might show gap in polygon
				}
				trianglesdata := *pixel.MakeTrianglesData(len(triangles))
				NumTriangles += len(triangles)
				for i, _ := range triangles {
					trianglesdata[i].Position = triangles[i]
					if *detailColor {
						mu.Lock() // to frequent call to rand makes the system crash, rand is not thread safe, causes Panic
						color = pixel.RGB(r.Float64(), r.Float64(), r.Float64())
						mu.Unlock()
					}
					trianglesdata[i].Color = color
				}
				drawer := pixel.NewBatch(&trianglesdata, nil)
				mu.Lock()
				drawers = append(drawers, drawer) //make append atomic
				mu.Unlock()
			}
			mu.Lock()
			duration := time.Since(now)
			mu.Unlock()
			//fmt.Printf("This item:%d points, %d Triangles in %2.2f ms\n", pointcnt, NumTriangles, float64(duration.Nanoseconds())/1000000)
			job <- NumTriangles
			timeC <- duration.Nanoseconds()
		}(list, job, timeC)
	}
	var totalTimespent int64
	totalNumTriangles := 0
	totalItems := 0
	for i, result := range jobs {
		totalNumTriangles += <-result
		totalTimespent += <-timespent[i]
		totalItems++
	}
	fmt.Printf("Processed \n%d entities\n%d points\n%d triangles\n in %d ms\n", totalItems, totalsPo, totalNumTriangles, totalTimespent/1000000)
}

func run() {
	var (
		camPos       = pixel.ZV
		camSpeed     = 10.0
		camZoom      = 1.0
		camZoomSpeed = 1.2
	)
	win, err := pixelgl.NewWindow(cfg)
	if err != nil {
		panic(err)
	}
	var ok = true
	var gedaan = false
	imd = imdraw.New(nil)
	done = make(chan bool)
	go createData()
	for !win.Closed() {
		if win.JustPressed(pixelgl.KeyEscape) {
			return
		}
		mousePos := win.MousePosition()
		camZoom *= math.Pow(camZoomSpeed, win.MouseScroll().Y)
		if camZoom > 50 {
			camZoom = 50
		} else {
			if camZoom < 0.5 {
				camZoom = 0.5
			}
		}
		win.SetMatrix(pixel.IM.Scaled(mousePos, camZoom))
		if win.Pressed(pixelgl.KeyLeft) {
			camPos.X -= camSpeed / camZoom
		}
		if win.Pressed(pixelgl.KeyRight) {
			camPos.X += camSpeed / camZoom
		}
		if win.Pressed(pixelgl.KeyDown) {
			camPos.Y -= camSpeed / camZoom
		}
		if win.Pressed(pixelgl.KeyUp) {
			camPos.Y += camSpeed / camZoom
		}
		if win.Pressed(pixelgl.KeyKPAdd) {
			camZoom += 0.1
		}
		if win.Pressed(pixelgl.KeyKPSubtract) {
			camZoom -= 0.1
		}
		cam := pixel.IM.Scaled(camPos, camZoom) //update scaling and movement
		win.SetMatrix(cam)                      //update view port
		if win.JustPressed(pixelgl.KeySpace) {
			ok = !ok
		}
		win.Clear(colornames.Black)
		win.SetSmooth(true)
		for _, drawer := range drawers {
			drawer.Draw(win)
		}
		if ok {
			select {
			case gedaan = <-done:
			default:
			}
			if gedaan {
				imd.Draw(win)
			}
		}
		win.Update()
	}
}
