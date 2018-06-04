// Triangulate
package Triangulate

import (
	"errors"
	"fmt"
	"log"
	"math"
	"math/rand"
	"regexp"
	"runtime"
	"time"

	"github.com/faiface/pixel"
	"github.com/faiface/pixel/imdraw"
)

var (
	imds    []*imdraw.IMDraw
	r       *rand.Rand
	pressed bool
)

func TimeTrack(start time.Time) {
	// Skip this function, and fetch the PC and file for its parent
	pc, _, _, _ := runtime.Caller(1)
	// Retrieve a Function object this functions parent
	functionObject := runtime.FuncForPC(pc)
	// Regex to extract just the function name (and not the module path)
	extractFnName := regexp.MustCompile(`^.*\.(.*)$`)
	name := extractFnName.ReplaceAllString(functionObject.Name(), "$1")
	log.Printf("%s took %s\n", name, time.Since(start))
}

type Point struct {
	A string
	X float64
	Y float64
}

func (p *Point) Delete() {
	p.A = ""
}

func (p *Point) UnDelete() {
	p.A = "-"
}

func (p *Point) Vec() pixel.Vec {
	return pixel.V(p.X, p.Y)
}

func (p *Point) IsDeleted() bool {
	return p.A == ""
}

func (p Point) String() string {
	return fmt.Sprintf("Deleted: %t, X:%v, Y:%v", p.IsDeleted(), p.X, p.Y)
}

func IsConvex(p1, p2, p3 Point) bool {
	//http://myitlearnings.com/checking-collinearity-of-3-points-and-their-orientation/
	return (p2.Y-p1.Y)*(p3.X-p2.X)-(p3.Y-p2.Y)*(p2.X-p1.X) >= 0
}

func InTriangle(p1, p2, p3, p Point) bool {
	//barycentric coordinates
	α := ((p2.Y-p3.Y)*(p.X-p3.X) + (p3.X-p2.X)*(p.Y-p3.Y)) / ((p2.Y-p3.Y)*(p1.X-p3.X) + (p3.X-p2.X)*(p1.Y-p3.Y))
	β := ((p3.Y-p1.Y)*(p.X-p3.X) + (p1.X-p3.X)*(p.Y-p3.Y)) / ((p2.Y-p3.Y)*(p1.X-p3.X) + (p3.X-p2.X)*(p1.Y-p3.Y))
	γ := 1.0 - α - β
	//fmt.Println(α, β, γ, α > 0 && β > 0 && γ > 0)
	return α > 0 && β > 0 && γ > 0
}

//type Ear struct {
//	p1, p2, p3 Point
//}

//func (e Ear) Surface() float64 {
//	a := math.Sqrt((e.p2.X-e.p1.X)*(e.p2.X-e.p1.X) + (e.p2.Y-e.p1.Y)*(e.p2.Y-e.p1.Y))
//	b := math.Sqrt((e.p2.X-e.p3.X)*(e.p2.X-e.p3.X) + (e.p2.Y-e.p3.Y)*(e.p2.Y-e.p3.Y))
//	c := math.Sqrt((e.p3.X-e.p1.X)*(e.p3.X-e.p1.X) + (e.p3.Y-e.p1.Y)*(e.p3.Y-e.p1.Y))
//	//herons formula area of triangle
//	s := (a + b + c) / 2
//	return math.Sqrt(s * (s - a) * (s - b) * (s - c))

//}

//func (e Ear) String() string {
//	return fmt.Sprintf("%v\n%v\n%v\n", e.p1, e.p2, e.p3)
//}

type Poly struct {
	P    []Point
	Pos  int
	size int
}

func NewPoly() *Poly {
	p := new(Poly)
	p.Pos = 0
	p.size = 0
	return p
}

//IsClockwise checks if points are organized clockwise,
//Triangulation function works on clockwise ordered data only
func (poly Poly) IsClockwise() bool {
	p := poly.P
	first := p[0]
	last := p[len(p)-1]
	sum := (first.X - last.X) * (first.Y + last.Y)
	current := first
	for _, e := range p {
		sum += (e.X - current.X) * (e.Y + current.Y)
		current = e
	}
	return sum > 0
}

//First() returns first not-deleted element and position, sets pointer to first valid element
func (poly *Poly) First() (point Point, i int) {
	for i, point = range poly.P {
		if !point.IsDeleted() {
			poly.Pos = i
			return
		}
	}
	return
}

//Last() returns last valid element or empty
func (poly *Poly) Last() (point Point, i int) {
	for i = len(poly.P) - 1; i >= 0; i-- {
		if !poly.P[i].IsDeleted() {
			return poly.P[i], i
		}
	}
	return
}

//Next() returns next valid element or empty and position of the element
func (poly *Poly) Next() (point Point, i int) {
	for i = poly.Pos + 1; i < len(poly.P); i++ {
		if !poly.P[i].IsDeleted() {
			poly.Pos = i
			return poly.P[i], i
		}
	}
	return
}

func (poly *Poly) Size() int {
	return poly.size
}

func (poly *Poly) Delete(e int) {
	poly.P[e].Delete()
	poly.size--

}

//MoveToBack moves the first element to the last position shifting all elements 1 position forwards
func (poly *Poly) MoveToBack() {
	p := poly.P[0]
	copy(poly.P[0:], poly.P[0+1:])
	poly.P[len(poly.P)-1] = p
	poly.First()
}

//UnDeleteAll logically undeletes all poly points
func (poly *Poly) UnDeleteAll() {
	for i, _ := range poly.P {
		poly.P[i].UnDelete()
	}
	poly.Pos = 0
	poly.size = len(poly.P)
}

//MoveToFront moves the last element to the first position shifting alle elements 1 position backwards
func (poly *Poly) MoveToFront() {
	p := poly.P[len(poly.P)-1]
	copy(poly.P[1:], poly.P[:len(poly.P)-2])
	poly.P[0] = p
	poly.First()
}

func (poly Poly) String() string {
	st := "\n"
	for _, p := range poly.P {
		st += fmt.Sprintf("|%v |", p)
	}
	st += fmt.Sprintf("pos: %d", poly.Pos)
	return st
}

//SetToLeftMost sets the leftmost element as teh first for triangulation
//optional function works on some specific cases
func (poly *Poly) SetToLeftMost() {
	minX := math.MaxFloat64
	minPos := 0
	Plen := len(poly.P)
	for i, p := range poly.P {
		if minX > p.X {
			minX = p.X
			minPos = i
		}
	}
	p := poly.P[minPos : len(poly.P)-1]
	poly.P = append(p, poly.P[:Plen-len(p)]...)
}

//PushBack Pushes a new point onto the last position of the poly and sets the point as not-deleted
//if a limit is provided then points within this limit from the previouspoint will not be added
//simplifying the area to triangulate. passing 0 does not not remove points. Compares the Δ of X and Y distances
func (poly *Poly) PushBack(p Point, limit float64) {
	if limit > 0 && len(poly.P) > 0 {
		p1 := poly.P[len(poly.P)-1]
		pctX := p1.X / limit
		pctY := p1.Y / limit
		// don't add duplicates and very close-by points: created issues for point with a very small distance and third point relatively far
		if p1.X == p.X && p1.Y == p.Y || (math.Abs(p1.X-p.X) < pctX && math.Abs(p1.Y-p.Y) < pctY) {
			return
		}
	}
	if p.A == "" {
		p.A = "|"
	}
	poly.size++
	poly.P = append(poly.P, p)
}

//SetClockwise orders all poly points in reverse order if they are not yet clockwise
func (poly *Poly) SetClockwise() {
	if poly.IsClockwise() {
		return
	}
	for i, j := 0, len(poly.P)-1; i < j; i, j = i+1, j-1 {
		poly.P[i], poly.P[j] = poly.P[j], poly.P[i]
	}
}

//GetTriangles calculates the triangles to cover the aera of a polygon based on points of the polygon
//Points should be ordered clockwise for this to work
//In some rare cases the solution is not deterministic, the error show the missing points
func GetTriangles(poly *Poly) (ears []pixel.Vec, err error) {
	loop := 0
	for poly.Size() > 0 && loop < len(poly.P)*3 { //run until all elements are deleted & prevent endless loop
		loop++
		size := poly.Size() // non deleted elements count
		if size < 3 {       // can not be a triangle : should never happen
			break
		}
		if size == 3 { // 3 remaining points
			var points = [3]Point{}
			i := 0
			for j, e := range poly.P {
				if !e.IsDeleted() { // find not deleted points
					points[i] = e  //save points
					poly.Delete(j) // delete point
					i++
				}
			}
			if points[0].X*(points[1].Y-points[2].Y)+points[1].X*(points[2].Y-points[0].Y)+points[2].X*(points[0].Y-points[1].Y) == 0 { //test for colinearity
				//if all 3 points on a single line the (non) triangle is rejected
			} else {
				ears = append(ears, points[0].Vec(), points[1].Vec(), points[2].Vec())
			}
			break
		}
		if size == 4 { // 4 remaining points, cut into 2 triangles
			var points = [4]Point{}
			i := 0
			for j, e := range poly.P {
				if !e.IsDeleted() { // find not deleted points
					points[i] = e  //save points
					poly.Delete(j) // delete point
					i++
				}
			}
			//			if points[0].X*(points[1].Y-points[2].Y)+points[1].X*(points[2].Y-points[0].Y)+points[2].X*(points[0].Y-points[1].Y) == 0 ||
			//				points[0].X*(points[1].Y-points[2].Y)+points[3].X*(points[2].Y-points[0].Y)+points[2].X*(points[0].Y-points[3].Y) == 0 { //test for colinearity
			//				//if all 3 points on a single line the (non) triangle is rejected
			//			} else {
			ears = append(ears, points[0].Vec(), points[1].Vec(), points[2].Vec(), points[0].Vec(), points[3].Vec(), points[2].Vec())
			//			}
			break
		}
		for i := 1; i < size-1; i++ {
			p1, _ := poly.First() //retrieve 1st point for test
			p2, i2 := poly.Next() //retrieve 2nd
			p3, _ := poly.Next()  //retrieve 3rd
			valid := true
			if IsConvex(p1, p2, p3) { // angle must be convex e.i. in polygon
				for _, p := range poly.P { // test if any point (deleted or not) is inside the new triangle
					if p != p1 && p != p2 && p != p3 { // only test for all other points, not self
						//						valid = valid && !InTriangle(p1, p2, p3, p)
						//						if !valid {
						if InTriangle(p1, p2, p3, p) {
							poly.MoveToBack() //move first point to the end
							valid = false
							break
						}
					}
				}
				if valid { //cut ear
					poly.Delete(i2)                                              //logically delete middle point
					poly.MoveToBack()                                            //move firstpoint to end
					if p1.X*(p2.Y-p3.Y)+p2.X*(p3.Y-p1.Y)+p3.X*(p1.Y-p2.Y) == 0 { //test for colinearity
						//fmt.Println("3 in a row, not a triangle")
					} else {
						ears = append(ears, p1.Vec(), p2.Vec(), p3.Vec())
					}
				}
			} else {
				//not convex p1, p2, p3
				poly.MoveToBack() //move first point to the end
			}
		}
	}
	if loop >= len(poly.P)*3 {
		pointStr := ""
		for i, point := range poly.P {
			if !point.IsDeleted() {
				pointStr += fmt.Sprintf("%d, %v\n", i, point)
			}
		}
		err = errors.New(fmt.Sprintf(" not deterministic:  %d iterations for %d points %d points remaining %v\n", loop, len(poly.P), poly.Size(), pointStr))
		//fmt.Println(poly)
	}
	return ears, err
}
