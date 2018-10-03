package convtree

import (
	"errors"
	"fmt"
	"image/color"
	"math"
	"os"
	"strconv"

	"github.com/gonum/stat"
	"github.com/satori/go.uuid"

	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/vg"

	"gonum.org/v1/plot"
)

var initXSize float64
var initYSize float64

type ConvTree struct {
	ID               string
	IsLeaf           bool
	MaxPoints        int
	MaxDepth         int
	Depth            int
	GridSize         int
	ConvNum          int
	Kernel           [][]float64
	Points           []Point
	MinXLength       float64
	MinYLength       float64
	BottomLeft       Point
	TopRight         Point
	ChildTopLeft     *ConvTree
	ChildTopRight    *ConvTree
	ChildBottomLeft  *ConvTree
	ChildBottomRight *ConvTree
	Stats            CellStats
}

func NewConvTree(bottomLeft Point, topRight Point, minXLength float64, minYLength float64, maxPoints int, maxDepth int,
	convNumber int, gridSize int, kernel [][]float64, initPoints []Point) (ConvTree, error) {
	if bottomLeft.X >= topRight.X {
		err := errors.New("X of bottom left point is larger or equal to X of top right point")
		return ConvTree{}, err
	}
	if bottomLeft.Y >= topRight.Y {
		err := errors.New("Y of bottom left point is larger or equal to Y of top right point")
		return ConvTree{}, err
	}
	id, _ := uuid.NewV4()
	if !checkKernel(kernel) {
		kernel = [][]float64{
			[]float64{0.5, 0.5, 0.5},
			[]float64{0.5, 1.0, 0.5},
			[]float64{0.5, 0.5, 0.5},
		}
	}
	tree := ConvTree{
		IsLeaf:     true,
		ID:         id.String(),
		MaxPoints:  maxPoints,
		GridSize:   gridSize,
		ConvNum:    convNumber,
		Kernel:     kernel,
		MaxDepth:   maxDepth,
		BottomLeft: bottomLeft,
		TopRight:   topRight,
		Points:     []Point{},
		MinXLength: minXLength,
		MinYLength: minYLength,
	}
	if initPoints != nil {
		tree.Points = initPoints
	}
	initXSize = topRight.X - bottomLeft.X
	initYSize = topRight.Y - bottomLeft.Y
	if tree.checkSplit() {
		tree.split()
	} else {
		tree.getStats()
		tree.getBaseline()
	}
	return tree, nil
}

func checkKernel(kernel [][]float64) bool {
	if kernel == nil || len(kernel) == 0 {
		return false
	}
	if kernel[0] == nil {
		return false
	}
	xSize, ySize := len(kernel[0]), len(kernel)
	if xSize != ySize {
		return false
	}
	for _, row := range kernel {
		if len(row) != xSize {
			return false
		}
	}
	return true
}

func (tree *ConvTree) split() {
	xSize, ySize := tree.GridSize, tree.GridSize
	grid := make([][]float64, xSize)
	xStep := (tree.TopRight.X - tree.BottomLeft.X) / float64(xSize)
	yStep := (tree.TopRight.Y - tree.BottomLeft.Y) / float64(ySize)
	for i := 0; i < xSize; i++ {
		grid[i] = make([]float64, ySize)
		for j := 0; j < ySize; j++ {
			xLeft := tree.BottomLeft.X + float64(i)*xStep
			xRight := tree.BottomLeft.X + float64(i+1)*xStep
			yTop := tree.BottomLeft.Y + float64(j)*yStep
			yBottom := tree.BottomLeft.Y + float64(j+1)*yStep
			grid[i][j] = float64(tree.getNodeWeight(xLeft, xRight, yTop, yBottom))
		}
	}
	convolved := normalizeGrid(grid)
	for i := 0; i < tree.ConvNum; i++ {
		tmpGrid, err := convolve(convolved, tree.Kernel, 1, 1)
		if err != nil {
			fmt.Println(err)
			break
		}
		convolved = normalizeGrid(tmpGrid)
	}
	convolved = normalizeGrid(convolved)
	xMax, yMax := getSplitPoint(convolved)
	if xMax < 1 || xMax >= (len(convolved)-1) {
		xMax = len(convolved) / 2
	}
	if yMax < 1 || yMax >= (len(convolved[0])-1) {
		yMax = len(convolved[0]) / 2
	}
	xOffset := float64(xMax) * xStep
	yOffset := float64(yMax) * yStep

	xRight := tree.BottomLeft.X + xOffset
	if xRight-tree.BottomLeft.X < tree.MinXLength {
		xRight = tree.BottomLeft.X + tree.MinXLength
	}
	if tree.TopRight.X-xRight < tree.MinXLength {
		xRight = tree.TopRight.X - tree.MinXLength
	}
	yBottom := tree.BottomLeft.Y + yOffset
	if yBottom-tree.BottomLeft.Y < tree.MinYLength {
		yBottom = tree.BottomLeft.Y + tree.MinYLength
	}
	if tree.TopRight.Y-yBottom < tree.MinYLength {
		yBottom = tree.TopRight.Y - tree.MinYLength
	}
	id, _ := uuid.NewV4()
	tree.ChildTopLeft = &ConvTree{
		ID:         id.String(),
		BottomLeft: tree.BottomLeft,
		TopRight: Point{
			X: xRight,
			Y: yBottom,
		},
		MaxPoints:  tree.MaxPoints,
		MaxDepth:   tree.MaxDepth,
		Kernel:     tree.Kernel,
		Depth:      tree.Depth + 1,
		GridSize:   tree.GridSize,
		ConvNum:    tree.ConvNum,
		MinXLength: tree.MinXLength,
		MinYLength: tree.MinYLength,
		IsLeaf:     true,
	}
	tree.ChildTopLeft.Points = tree.filterSplitPoints(tree.ChildTopLeft.BottomLeft, tree.ChildTopLeft.TopRight)
	if tree.ChildTopLeft.checkSplit() {
		tree.ChildTopLeft.split()
	} else {
		tree.ChildTopLeft.getStats()
		tree.ChildTopLeft.Stats.BaselineTags = tree.Stats.BaselineTags
	}

	id, _ = uuid.NewV4()
	tree.ChildTopRight = &ConvTree{
		ID: id.String(),
		BottomLeft: Point{
			X: xRight,
			Y: tree.BottomLeft.Y,
		},
		TopRight: Point{
			X: tree.TopRight.X,
			Y: yBottom,
		},
		MaxPoints:  tree.MaxPoints,
		MaxDepth:   tree.MaxDepth,
		Kernel:     tree.Kernel,
		Depth:      tree.Depth + 1,
		GridSize:   tree.GridSize,
		ConvNum:    tree.ConvNum,
		MinXLength: tree.MinXLength,
		MinYLength: tree.MinYLength,
		IsLeaf:     true,
	}
	tree.ChildTopRight.Points = tree.filterSplitPoints(tree.ChildTopRight.BottomLeft, tree.ChildTopRight.TopRight)
	if tree.ChildTopRight.checkSplit() {
		tree.ChildTopRight.split()
	} else {
		tree.ChildTopRight.getStats()
		tree.ChildTopRight.Stats.BaselineTags = tree.Stats.BaselineTags
	}

	id, _ = uuid.NewV4()
	tree.ChildBottomLeft = &ConvTree{
		ID: id.String(),
		BottomLeft: Point{
			X: tree.BottomLeft.X,
			Y: yBottom,
		},
		TopRight: Point{
			X: xRight,
			Y: tree.TopRight.Y,
		},
		MaxPoints:  tree.MaxPoints,
		MaxDepth:   tree.MaxDepth,
		Kernel:     tree.Kernel,
		Depth:      tree.Depth + 1,
		GridSize:   tree.GridSize,
		ConvNum:    tree.ConvNum,
		MinXLength: tree.MinXLength,
		MinYLength: tree.MinYLength,
		IsLeaf:     true,
	}
	tree.ChildBottomLeft.Points = tree.filterSplitPoints(tree.ChildBottomLeft.BottomLeft, tree.ChildBottomLeft.TopRight)
	if tree.ChildBottomLeft.checkSplit() {
		tree.ChildBottomLeft.split()
	} else {
		tree.ChildBottomLeft.getStats()
		tree.ChildBottomLeft.Stats.BaselineTags = tree.Stats.BaselineTags
	}

	id, _ = uuid.NewV4()
	tree.ChildBottomRight = &ConvTree{
		ID: id.String(),
		BottomLeft: Point{
			X: xRight,
			Y: yBottom,
		},
		TopRight:   tree.TopRight,
		MaxPoints:  tree.MaxPoints,
		MaxDepth:   tree.MaxDepth,
		Kernel:     tree.Kernel,
		Depth:      tree.Depth + 1,
		GridSize:   tree.GridSize,
		ConvNum:    tree.ConvNum,
		MinXLength: tree.MinXLength,
		MinYLength: tree.MinYLength,
		IsLeaf:     true,
	}
	tree.ChildBottomRight.Points = tree.filterSplitPoints(tree.ChildBottomRight.BottomLeft, tree.ChildBottomLeft.TopRight)
	if tree.ChildBottomRight.checkSplit() {
		tree.ChildBottomRight.split()
	} else {
		tree.ChildBottomRight.getStats()
		tree.ChildBottomRight.Stats.BaselineTags = tree.Stats.BaselineTags
	}

	tree.IsLeaf = false
	tree.Points = nil
}

func getSplitPoint(grid [][]float64) (int, int) {
	threshold := 0.8
	maxX, maxY := 0, 0
	maxValue := 0.0
	for i := 0; i < len(grid); i++ {
		for j := 0; j < len(grid[0]); j++ {
			if grid[i][j] > maxValue {
				maxValue = grid[i][j]
				maxX, maxY = i, j
			}
		}
	}
	splitValue := maxValue * threshold
	counter := 1
	itemFound := false
	splitX, splitY := 0, 0
	for {
		x, y := 0, 0
		vals := []float64{}
		itemFound = false
		i := maxX - counter
		if i >= 0 {
			for j := maxY - counter; j <= maxY+counter; j++ {
				if j >= 0 && j < len(grid[0]) {
					if grid[i][j] > splitValue {
						itemFound = true
						x = i
						vals = append(vals, grid[i][j])
					}
				}
			}
		}
		i = maxX + counter
		if i < len(grid) {
			for j := maxY - counter; j <= maxY+counter; j++ {
				if j >= 0 && j < len(grid[0]) {
					if grid[i][j] > splitValue {
						itemFound = true
						if math.Abs(float64(x-len(grid)/2)) > math.Abs(float64(i-len(grid)/2)) {
							x = i
						}
						vals = append(vals, grid[i][j])
					}
				}
			}
		}
		i = maxY - counter
		if i >= 0 {
			for j := maxX - counter; j <= maxX+counter; j++ {
				if j >= 0 && j < len(grid) {
					if grid[j][i] > splitValue {
						itemFound = true
						y = i
						if j != maxX-counter && j != maxX+counter {
							vals = append(vals, grid[j][i])
						}
					}
				}
			}
		}
		i = maxY + counter
		if i < len(grid[0]) {
			for j := maxX - counter; j <= maxX+counter; j++ {
				if j >= 0 && j < len(grid) {
					if grid[j][i] > splitValue {
						itemFound = true
						if math.Abs(float64(y-len(grid[0])/2)) > math.Abs(float64(i-len(grid[0])/2)) {
							y = i
						}
						if j != maxX-counter && j != maxX+counter {
							vals = append(vals, grid[j][i])
						}
					}
				}
			}
		}
		if !itemFound {
			break
		}
		if x != 0 {
			splitX = x
		}
		if y != 0 {
			splitY = y
		}
		splitValue = stat.Mean(vals, nil) * threshold
		counter++
	}
	if splitX > maxX {
		splitX++
	} else {
		splitX--
	}
	if splitY > maxY {
		splitY++
	} else {
		splitY--
	}
	return splitX, splitY
}

func (tree *ConvTree) Insert(point Point, allowSplit bool) {
	if !tree.IsLeaf {
		if point.X >= tree.ChildTopLeft.BottomLeft.X && point.X <= tree.ChildTopLeft.TopRight.X &&
			point.Y >= tree.ChildTopLeft.BottomLeft.Y && point.Y <= tree.ChildTopLeft.TopRight.Y {
			tree.ChildTopLeft.Insert(point, allowSplit)
			return
		}
		if point.X >= tree.ChildTopRight.BottomLeft.X && point.X <= tree.ChildTopRight.TopRight.X &&
			point.Y >= tree.ChildTopRight.BottomLeft.Y && point.Y <= tree.ChildTopRight.TopRight.Y {
			tree.ChildTopRight.Insert(point, allowSplit)
			return
		}
		if point.X >= tree.ChildBottomLeft.BottomLeft.X && point.X <= tree.ChildBottomLeft.TopRight.X &&
			point.Y >= tree.ChildBottomLeft.BottomLeft.Y && point.Y <= tree.ChildBottomLeft.TopRight.Y {
			tree.ChildBottomLeft.Insert(point, allowSplit)
			return
		}
		if point.X >= tree.ChildBottomRight.BottomLeft.X && point.X <= tree.ChildBottomRight.TopRight.X &&
			point.Y >= tree.ChildBottomRight.BottomLeft.Y && point.Y <= tree.ChildBottomRight.TopRight.Y {
			tree.ChildBottomRight.Insert(point, allowSplit)
			return
		}
	} else {
		tree.Points = append(tree.Points, point)
		if allowSplit {
			if tree.checkSplit() {
				tree.split()
			} else {
				tree.getStats()
				tree.getBaseline()
			}
		}
	}
}

func (tree *ConvTree) getBaseline() {
	tagValues := map[string]int{}
	for _, item := range tree.Points {
		if item.Content != nil {
			itemTags := map[string]bool{}
			tags := item.Content.([]string)
			for _, tag := range tags {
				if _, ok := itemTags[tag]; !ok {
					itemTags[tag] = true
				}
			}
			for tag := range itemTags {
				if _, ok := tagValues[tag]; !ok {
					tagValues[tag] = 0
				}
				tagValues[tag]++
			}
		}
	}
	if len(tagValues) > 0 {
		filteredTags := filterTags(tagValues)
		tree.Stats.BaselineTags = filteredTags
	}
}

func filterTags(tags map[string]int) []string {
	numbers := make([]float64, len(tags))
	i := 0
	for _, v := range tags {
		numbers[i] = float64(v)
		i++
	}
	avg := stat.Mean(numbers, nil)
	splitValue := int(avg)
	result := []string{}
	for k, v := range tags {
		if v > splitValue {
			result = append(result, k)
		}
	}
	return result
}

func (tree *ConvTree) Check() {
	if tree.checkSplit() {
		tree.split()
	} else {
		tree.getStats()
	}
}

func (tree *ConvTree) Clear() {
	tree.Points = nil
	if tree.ChildBottomLeft != nil {
		tree.ChildBottomLeft.Clear()
	}
	if tree.ChildBottomRight != nil {
		tree.ChildBottomRight.Clear()
	}
	if tree.ChildTopLeft != nil {
		tree.ChildTopLeft.Clear()
	}
	if tree.ChildTopRight != nil {
		tree.ChildTopRight.Clear()
	}
}

func (tree *ConvTree) getStats() {
	if len(tree.Points) == 0 {
		return
	}
	tree.Stats = CellStats{}
	for _, point := range tree.Points {
		tree.Stats.PointsNumber += point.Weight
	}
	var xTotal float64
	var yTotal float64
	var totalDistance float64
	for idx1, p := range tree.Points {
		xTotal += p.X
		yTotal += p.Y
		for idx2, p2 := range tree.Points {
			if idx1 == idx2 {
				continue
			}
			dist := math.Abs(p.X-p2.X) + math.Abs(p.Y-p2.Y)
			totalDistance += dist
		}
	}
	tree.Stats.CenterPoint = Point{
		X: xTotal / float64(tree.Stats.PointsNumber),
		Y: yTotal / float64(tree.Stats.PointsNumber),
	}
	if tree.Stats.PointsNumber > 1 {
		tree.Stats.AvgDistance = totalDistance / (math.Pow(float64(tree.Stats.PointsNumber), 2) - float64(tree.Stats.PointsNumber))
	}
}

func (tree ConvTree) Plot(filepath string, max int) error {
	p, err := tree.makePlot(&plot.Plot{}, max)
	if err != nil {
		return err
	}
	return p.Save(40*vg.Inch, 40*vg.Inch, filepath)
}

func (tree ConvTree) makePlot(p *plot.Plot, max int) (*plot.Plot, error) {
	if p.Title.Text == "" {
		var err error
		p, err = plot.New()
		if err != nil {
			return nil, err
		}
		p.X.Min = tree.BottomLeft.X
		p.X.Max = tree.TopRight.X
		p.Y.Min = tree.BottomLeft.Y
		p.Y.Max = tree.TopRight.Y
		p.Title.Text = "Plot"
	}
	lines := make(plotter.XYs, 5)
	lines[0].X = tree.BottomLeft.X
	lines[0].Y = tree.BottomLeft.Y
	lines[1].X = tree.TopRight.X
	lines[1].Y = tree.BottomLeft.Y
	lines[2].X = tree.TopRight.X
	lines[2].Y = tree.TopRight.Y
	lines[3].X = tree.BottomLeft.X
	lines[3].Y = tree.TopRight.Y
	lines[4].X = tree.BottomLeft.X
	lines[4].Y = tree.BottomLeft.Y
	l, err := plotter.NewLine(lines)
	if err != nil {
		return nil, err
	}
	p.Add(l)
	if !tree.IsLeaf {
		p, err := tree.ChildTopLeft.makePlot(p, max)
		if err != nil {
			return nil, err
		}
		p, err = tree.ChildTopRight.makePlot(p, max)
		if err != nil {
			return nil, err
		}
		p, err = tree.ChildBottomLeft.makePlot(p, max)
		if err != nil {
			return nil, err
		}
		p, err = tree.ChildBottomRight.makePlot(p, max)
		if err != nil {
			return nil, err
		}
	} else {
		points := make(plotter.XYs, len(tree.Points))
		for i := 0; i < len(tree.Points); i++ {
			points[i].X = tree.Points[i].X
			points[i].Y = tree.Points[i].Y
		}
		s, err := plotter.NewScatter(points)
		s.Color = color.RGBA{R: 255, B: 128, A: 255}
		if err != nil {
			return nil, err
		}
		p.Add(s)
	}
	return p, nil
}

func plotGrid(grid [][]float64, depth int, id string) {
	os.Remove("./grid-plots")
	p, err := plot.New()
	if err != nil {
		fmt.Println(err)
		return
	}
	p.X.Min = 0
	p.X.Max = float64(len(grid) + 1)
	p.Y.Min = 0
	p.Y.Max = float64(len(grid[0]) + 1)
	for i := 0; i < len(grid); i++ {
		for j := 0; j < len(grid[0]); j++ {
			lines := make(plotter.XYs, 5)
			lines[0].X = float64(i)
			lines[0].Y = float64(j)
			lines[1].X = float64(i + 1)
			lines[1].Y = float64(j)
			lines[2].X = float64(i + 1)
			lines[2].Y = float64(j + 1)
			lines[3].X = float64(i)
			lines[3].Y = float64(j + 1)
			lines[4].X = float64(i)
			lines[4].Y = float64(j)
			pol, err := plotter.NewPolygon(lines)
			if err != nil {
				fmt.Println(err)
				return
			}
			pol.Color = color.RGBA{A: uint8(255.0 * grid[i][j])}
			p.Add(pol)
		}
	}
	os.MkdirAll("./grid-plots", 0777)
	filepath := "./grid-plots/" + id + "-conv-" + strconv.Itoa(depth) + ".png"
	if err := p.Save(20*vg.Inch, 20*vg.Inch, filepath); err != nil {
		fmt.Println(err)
		return
	}
}

func (tree ConvTree) checkSplit() bool {
	cond1 := (tree.TopRight.X-tree.BottomLeft.X) > 2*tree.MinXLength && (tree.TopRight.Y-tree.BottomLeft.Y) > 2*tree.MinYLength
	totalWeight := 0
	for _, point := range tree.Points {
		totalWeight += point.Weight
	}
	cond2 := totalWeight > tree.MaxPoints && tree.Depth < tree.MaxDepth
	return cond1 && cond2
}

func (tree ConvTree) getNodeWeight(xLeft, xRight, yTop, yBottom float64) int {
	total := 0
	for _, point := range tree.Points {
		if point.X >= xLeft && point.X <= xRight && point.Y >= yTop && point.Y <= yBottom {
			total += point.Weight
		}
	}
	return total
}

func (tree ConvTree) filterSplitPoints(topLeft, bottomRight Point) []Point {
	result := []Point{}
	for _, point := range tree.Points {
		if point.X >= topLeft.X && point.X <= bottomRight.X && point.Y >= topLeft.Y && point.Y <= bottomRight.Y {
			result = append(result, point)
		}
	}
	return result
}

func convolve(grid [][]float64, kernel [][]float64, stride, padding int) ([][]float64, error) {
	if stride < 1 {
		err := errors.New("convolutional stride must be larger than 0")
		return nil, err
	}
	if padding < 1 {
		err := errors.New("convolutional padding must be larger than 0")
		return nil, err
	}
	kernelSize := len(kernel)
	if len(grid) < kernelSize {
		err := errors.New("grid width is less than convolutional kernel size")
		return nil, err
	}
	if len(grid[0]) < kernelSize {
		err := errors.New("grid height is less than convolutional kernel size")
		return nil, err
	}
	procGrid := make([][]float64, len(grid)+2*padding)
	for i := 0; i < padding; i++ {
		procGrid[i] = make([]float64, len(grid)+2*padding)
		for j := range procGrid[i] {
			procGrid[i][j] = 0
		}
	}
	for i := 1; i < (len(procGrid) - 1); i++ {
		procGrid[i] = make([]float64, len(grid)+2*padding)
		procGrid[i][0] = 0
		for j := 1; j < len(procGrid[i])-1; j++ {
			procGrid[i][j] = grid[i-padding][j-padding]
		}
		procGrid[i][len(procGrid[i])-1] = 0
	}
	for i := 0; i < padding; i++ {
		procGrid[len(procGrid)-i-1] = make([]float64, len(grid)+2*padding)
		for j := range procGrid[len(procGrid)-i-1] {
			procGrid[len(procGrid)-i-1][j] = 0
		}
	}
	resultWidth := int((len(grid)-kernelSize+2*padding)/stride) + 1
	resultHeight := int((len(grid[0])-kernelSize+2*padding)/stride) + 1
	result := make([][]float64, resultWidth)
	for i := 0; i < resultWidth; i++ {
		result[i] = make([]float64, resultHeight)
		for j := 0; j < resultHeight; j++ {
			total := 0.0
			for x := 0; x < kernelSize; x++ {
				for y := 0; y < kernelSize; y++ {
					posX := stride*i + x
					posY := stride*j + y
					if posX >= 0 && posX < len(procGrid) && posY >= 0 && posY < len(procGrid[0]) {
						total += procGrid[posX][posY] * kernel[x][y]
					}
				}
			}
			result[i][j] = total
		}
	}
	return result, nil
}

func printGrid(grid [][]float64) {
	for i := 0; i < len(grid); i++ {
		for j := 0; j < len(grid); j++ {
			fmt.Print(grid[i][j])
			fmt.Print("\t")
		}
		fmt.Print("\n")
	}
	fmt.Println("-----")
}

func normalizeGrid(grid [][]float64) [][]float64 {
	maxValue := -math.MaxFloat64
	for i := 0; i < len(grid); i++ {
		for j := 0; j < len(grid[0]); j++ {
			if grid[i][j] > maxValue {
				maxValue = grid[i][j]
			}
		}
	}
	for i := 0; i < len(grid); i++ {
		for j := 0; j < len(grid[0]); j++ {
			grid[i][j] = grid[i][j] / maxValue
		}
	}
	return grid
}
