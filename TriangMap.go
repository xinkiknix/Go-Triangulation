//TriangMap
/* application reads .SHP file and projects data onto the screen
after creating triangles for all components
All entities belonging to the same group have the same color
*/
package main

import (
	"flag"
	"fmt"
	"log"
	"math"
	"math/rand"

	Tri "TriangMap/Triangulate"
	"regexp"
	"runtime"
	"sync"
	"time"

	Shp "TriangMap/ShpReader"

	"github.com/gopxl/pixel/v2"
	"github.com/gopxl/pixel/v2/backends/opengl"
	"github.com/gopxl/pixel/v2/ext/imdraw"

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
	r            *rand.Rand
	sizes        Sizing
	cfg          opengl.WindowConfig
	shapes       []Shp.ShapeData
	head         Shp.Header
	wg           sync.WaitGroup
	imdReady     chan *imdraw.IMDraw
	drawersReady chan []*pixel.Batch
)

// var (
// 	drawers []*pixel.Batch
// 	imd     *imdraw.IMDraw
// )

var (
	src = flag.String("ShpFile", "world_.Shp", "Input shape file")
	//src         = flag.String("ShpFile", "in.shp", "Input shape file")
	trim        = flag.Int("TrimFactor", 0, "Trim factor: 0 does not remove coordinates, any other number trims points closer than % to previous point") ////*trim 0 = no simplification, 1200 is arbitrary value that seems to workd for complex models
	detailColor = flag.Bool("Detail", false, "True value shows triangle details in color variation per triangle")
)

func translate(value float64, min float64, max float64, minrange float64, maxrange float64) float64 {
	return minrange + (maxrange-minrange)*((value-min)/(max-min))
}

func main() {
	flag.Parse()
	fmt.Printf("processing file:%v Trimfactor:%d detailcolor:%t\n", *src, *trim, *detailColor)

	runtime.GOMAXPROCS(runtime.NumCPU() * 2) //use double number of processes as queue length
	bf, err := Shp.New(*src)
	if err != nil {
		log.Fatal(err)
	}
	head, shapes, err = Shp.ReadPolygons(&bf)
	if err != nil {
		log.Fatal(err)
	}
	Debug()

	opengl.Run(run)
}

func createData() {
	defer Tri.TimeTrack(time.Now())
	var mu sync.Mutex
	var pointCnt = 0
	r := rand.New(rand.NewSource(time.Now().Unix()))
	var lists [][]*Tri.Poly
	var drawers []*pixel.Batch
	imd := imdraw.New(nil)
	imd.Color = pixel.RGB(1, 1, 1) //border color
	imd.EndShape = imdraw.RoundEndShape
	for _, shape := range shapes {
		var list []*Tri.Poly
		for partNum := 0; partNum < int(shape.NumParts); partNum++ {
			poly := Tri.NewPoly() //create new set
			for _, points := range shape.Coordinates[partNum] {
				x := translate(points[0], sizes.ValueMinX, sizes.ValueMaxX, sizes.ScreenMinX, sizes.ScreenMaxX)
				y := translate(points[1], sizes.ValueMinY, sizes.ValueMaxY, sizes.ScreenMinY, sizes.ScreenMaxY)
				poly.PushBack(Tri.Point{false, x, y}, float64(*trim)) //*trim 0 = no simplification, 1200 is arbitrary value that seems to workd for complex models
				//Contours
				imd.Push(pixel.V(x, y))
				pointCnt++
			}
			//Contours
			imd.Line(0.2)
			list = append(list, poly)
		}
		lists = append(lists, list)
	}
	imdReady <- imd // prevents run loop from atempting to draw empty imd, which causes Panic
	var jobs []chan int
	var timespent []chan int64
	for _, list := range lists {
		job := make(chan int)
		jobs = append(jobs, job)
		timeC := make(chan int64)
		timespent = append(timespent, timeC)
		colorBase := r.Float64()
		color := pixel.RGB(colorBase, 0.3+colorBase, 0.5+colorBase)
		go func(list []*Tri.Poly, job chan<- int, timeC chan int64) {
			triangleCnt := 0
			mu.Lock()
			now := time.Now()
			mu.Unlock()
			pointCnt := 0
			for _, poly := range list {
				//Triangels
				triangles, err := Tri.GetTriangles(poly) // get all triangles to cover polygone area
				if err != nil {
					log.Println("Triangulation error", err) // non fatal error, just might show gap in polygon
				}
				pointCnt += len(poly.P)
				trianglesdata := *pixel.MakeTrianglesData(len(triangles))
				triangleCnt += len(triangles)
				r, g, b := 0.0, 0.0, 0.0 //r.Float64(), r.Float64(), r.Float64()
				min := math.Min(math.Min(r, g), b)
				r -= min
				g -= min
				b -= min
				max := math.Max(math.Max(r, g), b)
				inc := (1 - max) / float64(len(triangles))
				//fmt.Println(len(triangles), inc, r, g, b)
				for i, _ := range triangles {
					trianglesdata[i].Position = triangles[i]
					if *detailColor {
						mu.Lock() // to frequent call to rand makes the system crash, rand is not thread safe, causes Panic
						color = pixel.RGB(r, g, b)
						r += inc
						g += inc
						b += inc
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
			//fmt.Printf("This item:%d points, %d Triangles in %2.2f ms\n", pointCnt, triangleCnt, float64(duration.Nanoseconds())/1000000)
			job <- triangleCnt
			timeC <- duration.Nanoseconds()
		}(list, job, timeC)
	}
	drawersReady <- drawers
	var totalTimeSpent int64
	totalNumTriangles := 0
	entityCnt := 0
	for i, result := range jobs {
		totalNumTriangles += <-result
		totalTimeSpent += <-timespent[i]
		entityCnt++
	}
	fmt.Printf("Processed \n%d entities\n%d points\n%d triangles\n in %d ms\n", entityCnt, pointCnt, totalNumTriangles, totalTimeSpent/1000000)
}

func run() {
	var (
		camPos       = pixel.ZV
		camSpeed     = 10.0
		camZoom      = 1.0
		camZoomSpeed = 1.2
	)
	DisplayWidth, DisplayHeight := opengl.PrimaryMonitor().Size()
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
	Debug()
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
	cfg = opengl.WindowConfig{
		Title:     "ShapeFileViewer & polyFill (Triangulation)",
		Bounds:    pixel.R(0, 0, sizes.ScreenMaxX, sizes.ScreenMaxY),
		VSync:     true,
		Resizable: true,
	}
	Debug()
	fmt.Println("about to run 'run'")
	win, err := opengl.NewWindow(cfg)
	if err != nil {
		panic(err)
	}
	var showImd, showDrawers = false, false
	var imd *imdraw.IMDraw
	imd = imdraw.New(nil)
	imdReady = make(chan *imdraw.IMDraw)
	var drawers []*pixel.Batch
	drawersReady = make(chan []*pixel.Batch)
	go createData()
	camPos = win.Bounds().Center()

	for !win.Closed() {
		if win.JustPressed(pixel.KeyEscape) {
			return
		}
		select {
		case imd = <-imdReady:
			{
				showImd = true
			}
		case drawers = <-drawersReady:
			{
				showDrawers = true
			}
		default:
		}
		cam := pixel.IM.Scaled(camPos, camZoom).Moved(win.Bounds().Center().Sub(camPos))
		win.SetMatrix(cam)
		if win.JustPressed(pixel.MouseButtonLeft) {
			mouse := cam.Unproject(win.MousePosition())
			camPos = mouse
		}
		if win.Pressed(pixel.KeyLeft) {
			camPos.X -= camSpeed * camZoom
		}
		if win.Pressed(pixel.KeyRight) {
			camPos.X += camSpeed * camZoom
		}
		if win.Pressed(pixel.KeyDown) {
			camPos.Y -= camSpeed * camZoom
		}
		if win.Pressed(pixel.KeyUp) {
			camPos.Y += camSpeed * camZoom
		}
		if win.Pressed(pixel.KeyKPAdd) {
			camZoom += 0.1
		}
		if win.Pressed(pixel.KeyKPSubtract) {
			camZoom -= 0.1
		}
		if win.JustPressed(pixel.KeySpace) {
			showImd = !showImd
			camZoom = 1.0
			camPos = win.Bounds().Center()
		}
		camZoom *= math.Pow(camZoomSpeed, win.MouseScroll().Y)
		win.Clear(colornames.Navy)

		if showDrawers && drawers != nil {
			for _, drawer := range drawers {
				drawer.Draw(win)
			}
		}
		if showImd && imd != nil {
			imd.Draw(win)
		}
		win.Update()
	}
}

/*
	func GetFunctionName(i interface{}) string {
	    return runtime.FuncForPC(reflect.ValueOf(i).Pointer()).Name()
	}
*/
func Debug() string {
	pc, file, line, _ := runtime.Caller(1)
	functionObject := runtime.FuncForPC(pc)
	// Regex to extract just the function name (and not the module path)
	extractFnName := regexp.MustCompile(`^.*\.(.*)$`)
	name := extractFnName.ReplaceAllString(functionObject.Name(), "$1")
	fmt.Printf("debug info %s: function: %v line: %d \n", file, name, line)
	return fmt.Sprintln("debug info %s: function: %v line: %d \n", file, name, line)
}
