// shpReader
package shpReader

//package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io/ioutil"
	"math"
)

/*
Value 	Shape type 	Fields
   0 	Null shape 	None
   1 	Point 	        X, Y
   3 	Polyline 	MBR, Number of parts, Number of points, Parts, Points
   5 	Polygon 	MBR, Number of parts, Number of points, Parts, Points
   8 	MultiPoint 	MBR, Number of points, Points
   11 	PointZ 	        X, Y, Z, M
   13 	PolylineZ 	Mandatory: MBR, Number of parts, Number of points, Parts, Points, Z range, Z array Optional: M range, M array
   15 	PolygonZ 	Mandatory: MBR, Number of parts, Number of points, Parts, Points, Z range, Z array Optional: M range, M array
   18 	MultiPointZ 	Mandatory: MBR, Number of points, Points, Z range, Z array Optional: M range, M array
   21 	PointM 	        X, Y, M
   23 	PolylineM 	Mandatory: MBR, Number of parts, Number of points, Parts, Points Optional: M range, M array
   25 	PolygonM 	Mandatory: MBR, Number of parts, Number of points, Parts, Points Optional: M range, M array
   28 	MultiPointM 	Mandatory: MBR, Number of points, Points Optional Fields: M range, M array
   31 	MultiPatch 	Mandatory: MBR, Number of parts, Number of points, Parts, Part types, Points, Z range, Z array

   Optional: M range, M array

*/
const (
	NULLSHAPE   = 0
	POINT       = 1
	POLYLINE    = 3
	POLYGON     = 5
	MULTIPOINT  = 8
	POINTZ      = 11
	POLYLINEZ   = 13
	POLYGONZ    = 15
	MULTIPOINTZ = 18
	POINTM      = 21
	POLYLINEM   = 23
	POLYGONM    = 25
	MULTIPOINTM = 28
	MULTIPATH   = 31
)

type Header struct {
	/*
	   Bytes Type Endianness Usage
	   00–03 int32 big File code (always hex value 0x0000270a)*/
	FileCode uint32
	/* 04–23 int32 big Unused; five uint32 */
	_ [20]byte
	/* 24–27 int32 big File length (in 16-bit words, including the header)*/
	FileLength int32
	/* 28–31 int32 little Version*/
	Version int32
	/* 32–35 int32 little Shape type (see reference below)*/
	ShapeType int32
	/* 36–67 double little Minimum bounding rectangle (MBR) of all shapes contained within the shapefile; four doubles in the following order: min X, min Y, max X, max Y */
	MinX float64
	MinY float64
	MaxX float64
	MaxY float64
	/* 68–83 double little Range of Z; two doubles in the following order: min Z, max Z */
	MinZ float64
	MaxZ float64
	/* 84–99 double little Range of M; two doubles in the following order: min M, max M */
	MinM float64
	MaxM float64
	/*   The file then contains any number of variable-length records. Each record is prefixed with a record-header of 8
	bytes:
	Bytes Type  Endianness Usage
	0–3   int32 big        Record number (1-based)
	4–7   int32 big        Record length (in 16-bit words)
	Following the record header is the actual record:
	Bytes Type  Endianness Usage
	0–3   int32 little     Shape type (see reference above)
	4– - - Shape content
	*/
}

func (h Header) String() string {
	return fmt.Sprintf("filecode   :%#X\nfilelength :%v\nversion    :%v\nshape      :%d\nminX :%2.2f\nmaxX :%2.2f\nminY :%2.2f\nmaxY :%2.2f\n",
		h.FileCode, h.FileLength, h.Version, h.ShapeType,
		h.MinX, h.MaxX,
		h.MinY, h.MaxY)
}

func readHeader(bf *BinFileReader) (header Header, err error) {
	content := bf.ReadByte(100)
	buffer := bytes.NewBuffer(content)
	header = Header{}
	if err = binary.Read(buffer, binary.LittleEndian, &header); err != nil {
		return
	}
	header.FileCode = swapEncodingUint32(header.FileCode)
	if header.FileCode != 0X270A {
		err = errors.New("Not a Shapefile")
	}
	return
}

func writeHeader(header *Header) (err error) {
	buf := new(bytes.Buffer)
	header.FileCode = swapEncodingUint32(header.FileCode)
	if err = binary.Write(buf, binary.LittleEndian, header); err != nil {
		return
	}
	// raw data -> file fmt.Println(buf)
	fmt.Printf("% x\n", buf.Bytes())
	return
}

func swapEncodingUint16(val uint16) uint16 { //BigEndian <->LittleEndian
	return (val >> 8) | (val << 8)
}
func swapEncodingUint32(val uint32) uint32 { //BigEndian <->LittleEndian
	return ((val >> 24) & 0xff) | // move byte 3 to byte 0
		((val << 8) & 0xff0000) | // move byte 1 to byte 2
		((val >> 8) & 0xff00) | // move byte 2 to byte 1
		((val << 24) & 0xff000000) // move byte 0 to byte 3
}

type ShapeData struct {
	RecordNum     int32
	ContentLength int32
	ShapeType     int32 //Big endian
	Box0          float64
	Box1          float64
	Box2          float64
	Box3          float64
	NumParts      int32 //Big endian
	NumPoints     int32 //Big endian
	PartCount     []int
	Coordinates   [][][2]float64
}

func (s ShapeData) String() string {
	outStr := "Coordinates:\n    X    ,  Y\n"
	for _, p := range s.Coordinates {
		for _, c := range p {
			outStr += fmt.Sprintf("%2.2f,%2.2f\n", c[0], c[1])
		}
		outStr += ",\n"
	}
	return fmt.Sprintf("Record number:%d Content length:%d Shape type:%d Bounding box: %2.2f, %2.2f, %2.2f, %2.2f, Num parts: %d Num Points: %d, part count : %v \n%v",
		s.RecordNum, s.ContentLength, s.ShapeType, s.Box0, s.Box1, s.Box2, s.Box3, s.NumParts, s.NumPoints, s.PartCount, outStr)
}

func ReadPolygons(bf *BinFileReader) (header Header, data []ShapeData, err error) {
	/*
		   ***vvv Description of Main File Record Headers  vvv***
		    Byte Position Field          Value          Type    Order
		    Byte 0        Record Number  Record Number  Integer Big
		    Byte 4        Content Length Content Length Integer Big
		    ***^^^ Description of Main File Record Headers  ^^^***
			   Polygon
		    {
		    Float[4] Box // Bounding Box
		    Integer NumParts // Number of Parts
		    Integer NumPoints // Total Number of Points
		    Integer[NumParts] Parts // Index to First Point in Part
		    Point[NumPoints] Points // Points for All Parts
		    }
		    ***vvv Polygon Record Contents vvv***
		    Position Field          Value     Type     Number    Order
		    Byte 0   Shape Type     5         Integer  1         Little
		    Byte 4   Box            Box       Double   4         Little
		    Byte 36  NumParts       NumParts  Integer  1         Little
		    Byte 40  NumPoints      NumPoints Integer  1         Little
		    Byte 44  Parts          Parts     Integer  NumParts  Little
		    Byte X   Points         Points    Point    NumPoints Little

		    note X = 44 + 4 * numParts
		    ***^^^ Polygon Record Contents ^^^***
	*/

	//shapeNo := 0
	var shapesData []ShapeData
	head, err := readHeader(bf)
	if err != nil {
		return
	}
	for !bf.EOF() {
		shapeData := ShapeData{} //shapeNo++
		shapeData.RecordNum = bf.ReadIntLittle()
		shapeData.ContentLength = bf.ReadIntLittle()
		shapeData.ShapeType = bf.ReadIntBig()
		shapeData.Box0 = bf.ReadFloatLittle()
		shapeData.Box1 = bf.ReadFloatLittle()
		shapeData.Box2 = bf.ReadFloatLittle()
		shapeData.Box3 = bf.ReadFloatLittle()
		shapeData.NumParts = bf.ReadIntBig()
		shapeData.NumPoints = bf.ReadIntBig()
		shapeData.PartCount = make([]int, shapeData.NumParts)
		for i := 0; i < int(shapeData.NumParts); i++ {
			shapeData.PartCount[i] = int(bf.ReadIntBig())
		}
		count := 0
		for count = 0; count < len(shapeData.PartCount)-1; count++ {
			shapeData.PartCount[count] = shapeData.PartCount[count+1]
		}
		shapeData.PartCount[count] = int(shapeData.NumPoints)
		j := 0
		for i := 0; i < int(shapeData.NumParts); i++ {
			var points [][2]float64
			for ; j < shapeData.PartCount[i]; j++ {
				x := bf.ReadFloatLittle()
				y := bf.ReadFloatLittle()
				points = append(points, [2]float64{x, y})
			}
			shapeData.Coordinates = append(shapeData.Coordinates, points)
		}
		shapesData = append(shapesData, shapeData)
	}
	return head, shapesData, err
}

type BinFileReader struct {
	b      []byte
	pos    int
	length int
}

func New(filename string) (bf BinFileReader, err error) {
	content, err := ioutil.ReadFile(filename)
	if err != nil {
		return
	}
	//fmt.Println(len(content))
	bf.b = content
	bf.pos = 0
	bf.length = len(bf.b)
	return
}

func (bf *BinFileReader) EOF() bool {
	return bf.pos >= bf.length
}

func (bf *BinFileReader) ReadByte(nBytes int) []byte {
	buf := make([]byte, nBytes)
	for i := 0; i < nBytes; i++ {
		buf[i] = bf.b[bf.pos]
		bf.pos++
	}
	return buf
}

func (bf *BinFileReader) ReadIntBig() int32 {
	buf := make([]byte, 4)
	for i := 0; i < 4; i++ {
		buf[i] = bf.b[bf.pos]
		bf.pos++
	}
	return int32(buf[3])<<24 | int32(buf[2])<<16 | int32(buf[1])<<8 | int32(buf[0])
}

func (bf *BinFileReader) ReadIntLittle() int32 {
	buf := make([]byte, 4)
	for i := 0; i < 4; i++ {
		buf[i] = bf.b[bf.pos]
		bf.pos++
	}
	return int32(buf[0])<<24 | int32(buf[1])<<16 | int32(buf[2])<<8 | int32(buf[3])
}

func (bf *BinFileReader) ReadFloatBig() float64 {
	buf := make([]byte, 8)
	for i := 0; i < 8; i++ {
		buf[i] = bf.b[bf.pos]
		bf.pos++
	}
	bits := binary.LittleEndian.Uint64(buf)
	return math.Float64frombits(bits)

}

func (bf *BinFileReader) ReadFloatLittle() float64 {
	buf := make([]byte, 8)
	for i := 0; i < 8; i++ {
		buf[i] = bf.b[bf.pos]
		bf.pos++
	}
	bits := binary.LittleEndian.Uint64(buf)
	return math.Float64frombits(bits)
}

//func main() {
//	bf, err := New("cntry02.shp")
//	if err != nil {
//		fmt.Println(err)
//		return
//	}
//	head, shapes, err := ReadPolygons(&bf)
//	if err != nil {
//		fmt.Println(err)
//	}
//	if err = writeHeader(&head); err != nil {
//		fmt.Println(err)
//	}
//	fmt.Println(head)
//	for _, shape := range shapes {
//		fmt.Println(shape)
//	}

//}
