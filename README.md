# Go-Triangulation
Poly shape Triangulation example in OPENGL in GO
TriangMap uses Triangulate and ShpReader
ShpReader reads SHP files used to construct maps.
Maps are filled using Triangulation method
For the executional version there are 3 optional parameters
 * "ShpFile", Default = "world.Shp", "Input shape file"
 * "TrimFactor", Default = 0, "Trim factor: 0 does not remove coordinates, any other number will trim points closer than a derived % to previous point, normal values are 1000 - 2000, this is done because for some models there are way to many points that are very close together and have no visual values in the end-result"
 * "Detail", Default = false, "True value shows triangle details in color variation per triangle"
 
 Navigation of the map: Left, right, up, down arrow
 Zoom: + or - key on numpad
 Scroll Zoom/Navigation with mouse scroll wheel
 Terminate with esc
 
 The default SHP file is large, and shows most islands and territories over 1.000.000 triangles will be calculated.
 Any other map shape file is normally/of course significantly smaller.
 
 The solution is not deterministic for all shapes due to the fact that in some cases some points will remain that can not create a triangle without inclusion of other points in the shape. Changing the TrimFactor can prevent/create the problem.
