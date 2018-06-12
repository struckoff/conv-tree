package convtree

import (
	"errors"
	"fmt"
	"image/color"

	uuid "github.com/satori/go.uuid"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/vg"

	"gonum.org/v1/plot"
)

type QuadTree struct {
	ID               string
	IsLeaf           bool
	maxPoints        int
	maxDepth         int
	Depth            int
	splitSteps       int
	Points           []Point
	TopLeft          Point
	BottomRight      Point
	minXLength       float64
	minYLength       float64
	ChildTopLeft     *QuadTree
	ChildTopRight    *QuadTree
	ChildBottomLeft  *QuadTree
	ChildBottomRight *QuadTree
}

func NewQuadTree(topLeft Point, bottomRight Point, minXLength float64, minYLength float64, maxPoints int,
	maxDepth int, initPoints []Point) (QuadTree, error) {
	if topLeft.X >= bottomRight.X {
		err := errors.New("X of top left point is larger or equal to X of bottom right point")
		return QuadTree{}, err
	}
	if topLeft.Y >= bottomRight.Y {
		err := errors.New("Y of top left point is larger or equal to Y of bottom right point")
		return QuadTree{}, err
	}
	id, _ := uuid.NewV4()
	tree := QuadTree{
		ID:          id.String(),
		maxPoints:   maxPoints,
		maxDepth:    maxDepth,
		Depth:       0,
		splitSteps:  10,
		TopLeft:     topLeft,
		BottomRight: bottomRight,
		Points:      []Point{},
		minXLength:  minXLength,
		minYLength:  minYLength,
	}
	if initPoints != nil {
		tree.Points = initPoints
	}
	if tree.checkSplit() {
		tree.split()
	}
	return tree, nil
}

func (tree *QuadTree) Insert(point Point) {
	if !tree.IsLeaf {
		if point.X >= tree.ChildTopLeft.TopLeft.X && point.X <= tree.ChildTopLeft.BottomRight.X &&
			point.Y >= tree.ChildTopLeft.TopLeft.Y && point.Y <= tree.ChildTopLeft.BottomRight.Y {
			tree.ChildTopLeft.Insert(point)
			return
		}
		if point.X >= tree.ChildTopRight.TopLeft.X && point.X <= tree.ChildTopRight.BottomRight.X &&
			point.Y >= tree.ChildTopRight.TopLeft.Y && point.Y <= tree.ChildTopRight.BottomRight.Y {
			tree.ChildTopRight.Insert(point)
			return
		}
		if point.X >= tree.ChildBottomLeft.TopLeft.X && point.X <= tree.ChildBottomLeft.BottomRight.X &&
			point.Y >= tree.ChildBottomLeft.TopLeft.Y && point.Y <= tree.ChildBottomLeft.BottomRight.Y {
			tree.ChildBottomLeft.Insert(point)
			return
		}
		if point.X >= tree.ChildBottomRight.TopLeft.X && point.X <= tree.ChildBottomRight.BottomRight.X &&
			point.Y >= tree.ChildBottomRight.TopLeft.Y && point.Y <= tree.ChildBottomRight.BottomRight.Y {
			tree.ChildBottomRight.Insert(point)
			return
		}
	} else {
		tree.Points = append(tree.Points, point)
		if tree.checkSplit() {
			tree.split()
		}
	}
}

func (tree QuadTree) Print(prefix string) {
	innerPrefix := "\t"
	fmt.Printf("%s top left X - %f, top left Y - %f\n", prefix, tree.TopLeft.X, tree.TopLeft.Y)
	fmt.Printf("%s bottom right X - %f, bottom right Y - %f\n", prefix, tree.BottomRight.X, tree.BottomRight.Y)
	if tree.Points != nil {
		fmt.Printf("%s number of points - %d", prefix, len(tree.Points))
	}
	fmt.Println()
	if !tree.IsLeaf {
		tree.ChildTopLeft.Print(prefix + innerPrefix)
		tree.ChildTopRight.Print(prefix + innerPrefix)
		tree.ChildBottomLeft.Print(prefix + innerPrefix)
		tree.ChildBottomRight.Print(prefix + innerPrefix)
	}
}

func (tree QuadTree) Plot(filepath string, max int) error {
	p, err := tree.makePlot(&plot.Plot{}, max)
	if err != nil {
		return err
	}
	if err := p.Save(40*vg.Inch, 40*vg.Inch, filepath); err != nil {
		return err
	}
	return nil
}

func (tree QuadTree) makePlot(p *plot.Plot, max int) (*plot.Plot, error) {
	if p.Title.Text == "" {
		var err error
		p, err = plot.New()
		if err != nil {
			return nil, err
		}
		p.X.Min = tree.TopLeft.X
		p.X.Max = tree.BottomRight.X
		p.Y.Min = tree.TopLeft.Y
		p.Y.Max = tree.BottomRight.Y
		p.Title.Text = "Plot"
	}
	lines := make(plotter.XYs, 5)
	lines[0].X = tree.TopLeft.X
	lines[0].Y = tree.TopLeft.Y
	lines[1].X = tree.BottomRight.X
	lines[1].Y = tree.TopLeft.Y
	lines[2].X = tree.BottomRight.X
	lines[2].Y = tree.BottomRight.Y
	lines[3].X = tree.TopLeft.X
	lines[3].Y = tree.BottomRight.Y
	lines[4].X = tree.TopLeft.X
	lines[4].Y = tree.TopLeft.Y
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

func (tree QuadTree) checkSplit() bool {
	cond1 := (tree.BottomRight.X-tree.TopLeft.X) > 2*tree.minXLength && (tree.BottomRight.Y-tree.TopLeft.Y) > 2*tree.minYLength
	total := 0
	for _, point := range tree.Points {
		total += point.Weight
	}
	cond2 := total > tree.maxPoints && tree.Depth < tree.maxDepth
	return cond1 && cond2
}

func (tree QuadTree) checkSplitPoints(xLeft, xRight, yTop, yBottom float64) int {
	total := 0
	for _, point := range tree.Points {
		if point.X >= xLeft && point.X <= xRight && point.Y >= yTop && point.Y <= yBottom {
			total++
		}
	}
	return total
}

func (tree QuadTree) filterSplitPoints(topLeft, bottomRight Point) []Point {
	result := []Point{}
	for _, point := range tree.Points {
		if point.X >= topLeft.X && point.X <= bottomRight.X && point.Y >= topLeft.Y && point.Y <= bottomRight.Y {
			result = append(result, point)
		}
	}
	return result
}

func (tree *QuadTree) split() {
	xRight := tree.TopLeft.X + (tree.BottomRight.X-tree.TopLeft.X)/2.0
	yBottom := tree.TopLeft.Y + (tree.BottomRight.Y-tree.TopLeft.Y)/2.0
	id, _ := uuid.NewV4()
	tree.ChildTopLeft = &QuadTree{
		ID:      id.String(),
		TopLeft: tree.TopLeft,
		BottomRight: Point{
			X: xRight,
			Y: yBottom,
		},
		maxDepth:   tree.maxDepth,
		Depth:      tree.Depth + 1,
		maxPoints:  tree.maxPoints,
		splitSteps: tree.splitSteps,
		minXLength: tree.minXLength,
		minYLength: tree.minYLength,
		IsLeaf:     true,
	}
	tree.ChildTopLeft.Points = tree.filterSplitPoints(tree.ChildTopLeft.TopLeft, tree.ChildTopLeft.BottomRight)
	if tree.ChildTopLeft.checkSplit() {
		tree.ChildTopLeft.split()
	}

	id, _ = uuid.NewV4()
	tree.ChildTopRight = &QuadTree{
		ID: id.String(),
		TopLeft: Point{
			X: xRight,
			Y: tree.TopLeft.Y,
		},
		BottomRight: Point{
			X: tree.BottomRight.X,
			Y: yBottom,
		},
		maxDepth:   tree.maxDepth,
		Depth:      tree.Depth + 1,
		maxPoints:  tree.maxPoints,
		splitSteps: tree.splitSteps,
		minXLength: tree.minXLength,
		minYLength: tree.minYLength,
		IsLeaf:     true,
	}
	tree.ChildTopRight.Points = tree.filterSplitPoints(tree.ChildTopRight.TopLeft, tree.ChildTopRight.BottomRight)
	if tree.ChildTopRight.checkSplit() {
		tree.ChildTopRight.split()
	}

	id, _ = uuid.NewV4()
	tree.ChildBottomLeft = &QuadTree{
		ID: id.String(),
		TopLeft: Point{
			X: tree.TopLeft.X,
			Y: yBottom,
		},
		BottomRight: Point{
			X: xRight,
			Y: tree.BottomRight.Y,
		},
		maxDepth:   tree.maxDepth,
		Depth:      tree.Depth + 1,
		maxPoints:  tree.maxPoints,
		splitSteps: tree.splitSteps,
		minXLength: tree.minXLength,
		minYLength: tree.minYLength,
		IsLeaf:     true,
	}
	tree.ChildBottomLeft.Points = tree.filterSplitPoints(tree.ChildBottomLeft.TopLeft, tree.ChildBottomLeft.BottomRight)
	if tree.ChildBottomLeft.checkSplit() {
		tree.ChildBottomLeft.split()
	}

	id, _ = uuid.NewV4()
	tree.ChildBottomRight = &QuadTree{
		ID: id.String(),
		TopLeft: Point{
			X: xRight,
			Y: yBottom,
		},
		BottomRight: tree.BottomRight,
		maxDepth:    tree.maxDepth,
		Depth:       tree.Depth + 1,
		maxPoints:   tree.maxPoints,
		splitSteps:  tree.splitSteps,
		minXLength:  tree.minXLength,
		minYLength:  tree.minYLength,
		IsLeaf:      true,
	}
	tree.ChildBottomRight.Points = tree.filterSplitPoints(tree.ChildBottomRight.TopLeft, tree.ChildBottomRight.BottomRight)
	if tree.ChildBottomRight.checkSplit() {
		tree.ChildBottomRight.split()
	}

	tree.IsLeaf = false
	tree.Points = nil
}
