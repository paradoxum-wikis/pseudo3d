package internal

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
)

type SelectionOverlay struct {
	widget.BaseWidget

	MinX, MinY, MaxX, MaxY float32
	HasSelection           bool

	GetImageBounds func() (offX, offY, rendW, rendH float32)
	NatW, NatH     float32
}

func NewSelectionOverlay() *SelectionOverlay {
	s := &SelectionOverlay{}
	s.ExtendBaseWidget(s)
	return s
}

func (s *SelectionOverlay) SetSelection(minX, minY, maxX, maxY float32) {
	s.MinX, s.MinY, s.MaxX, s.MaxY = minX, minY, maxX, maxY
	s.HasSelection = true
	s.Refresh()
}

func (s *SelectionOverlay) ClearSelection() {
	s.HasSelection = false
	s.Refresh()
}

func (s *SelectionOverlay) MinSize() fyne.Size { return fyne.NewSize(0, 0) }

func (s *SelectionOverlay) CreateRenderer() fyne.WidgetRenderer {
	rect := canvas.NewRectangle(color.NRGBA{R: 153, G: 13, B: 255, A: 50})
	rect.StrokeColor = color.NRGBA{R: 153, G: 13, B: 255, A: 255}
	rect.StrokeWidth = 2
	rect.Hide()
	return &selectionRenderer{overlay: s, rect: rect}
}

type selectionRenderer struct {
	overlay *SelectionOverlay
	rect    *canvas.Rectangle
}

func (r *selectionRenderer) Layout(_ fyne.Size) {
	s := r.overlay
	if !s.HasSelection || s.GetImageBounds == nil ||
		s.MaxX <= s.MinX || s.MaxY <= s.MinY ||
		s.NatW == 0 || s.NatH == 0 {
		r.rect.Hide()
		return
	}
	ox, oy, rw, rh := s.GetImageBounds()
	if rw == 0 || rh == 0 {
		r.rect.Hide()
		return
	}
	r.rect.Move(fyne.NewPos(s.MinX/s.NatW*rw+ox, s.MinY/s.NatH*rh+oy))
	r.rect.Resize(fyne.NewSize((s.MaxX-s.MinX)/s.NatW*rw, (s.MaxY-s.MinY)/s.NatH*rh))
	r.rect.Show()
}

func (r *selectionRenderer) MinSize() fyne.Size           { return fyne.NewSize(0, 0) }
func (r *selectionRenderer) Refresh()                     { r.Layout(r.overlay.Size()) }
func (r *selectionRenderer) Destroy()                     {}
func (r *selectionRenderer) Objects() []fyne.CanvasObject { return []fyne.CanvasObject{r.rect} }

type InteractiveArea struct {
	widget.BaseWidget
	OnDrag    func(*fyne.DragEvent)
	OnDragEnd func()
	OnTap     func(*fyne.PointEvent)
	GetCursor func() desktop.Cursor
}

func (i *InteractiveArea) Dragged(e *fyne.DragEvent) { i.OnDrag(e) }
func (i *InteractiveArea) DragEnd()                  { i.OnDragEnd() }
func (i *InteractiveArea) Tapped(e *fyne.PointEvent) {
	if i.OnTap != nil {
		i.OnTap(e)
	}
}
func (i *InteractiveArea) Cursor() desktop.Cursor {
	if i.GetCursor != nil {
		return i.GetCursor()
	}
	return desktop.DefaultCursor
}

func (i *InteractiveArea) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(canvas.NewRectangle(color.Transparent))
}
