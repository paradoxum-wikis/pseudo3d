package main

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
)

type selectionOverlay struct {
	widget.BaseWidget

	minX, minY, maxX, maxY float32
	hasSelection           bool

	getImageBounds func() (offX, offY, rendW, rendH float32)
	natW, natH     float32
}

func newSelectionOverlay() *selectionOverlay {
	s := &selectionOverlay{}
	s.ExtendBaseWidget(s)
	return s
}

func (s *selectionOverlay) SetSelection(minX, minY, maxX, maxY float32) {
	s.minX, s.minY, s.maxX, s.maxY = minX, minY, maxX, maxY
	s.hasSelection = true
	s.Refresh()
}

func (s *selectionOverlay) ClearSelection() {
	s.hasSelection = false
	s.Refresh()
}

func (s *selectionOverlay) MinSize() fyne.Size { return fyne.NewSize(0, 0) }

func (s *selectionOverlay) CreateRenderer() fyne.WidgetRenderer {
	rect := canvas.NewRectangle(color.NRGBA{R: 153, G: 13, B: 255, A: 50})
	rect.StrokeColor = color.NRGBA{R: 153, G: 13, B: 255, A: 255}
	rect.StrokeWidth = 2
	rect.Hide()
	return &selectionRenderer{overlay: s, rect: rect}
}

type selectionRenderer struct {
	overlay *selectionOverlay
	rect    *canvas.Rectangle
}

func (r *selectionRenderer) Layout(_ fyne.Size) {
	s := r.overlay
	if !s.hasSelection || s.getImageBounds == nil ||
		s.maxX <= s.minX || s.maxY <= s.minY ||
		s.natW == 0 || s.natH == 0 {
		r.rect.Hide()
		return
	}
	ox, oy, rw, rh := s.getImageBounds()
	if rw == 0 || rh == 0 {
		r.rect.Hide()
		return
	}
	r.rect.Move(fyne.NewPos(s.minX/s.natW*rw+ox, s.minY/s.natH*rh+oy))
	r.rect.Resize(fyne.NewSize((s.maxX-s.minX)/s.natW*rw, (s.maxY-s.minY)/s.natH*rh))
	r.rect.Show()
}

func (r *selectionRenderer) MinSize() fyne.Size           { return fyne.NewSize(0, 0) }
func (r *selectionRenderer) Refresh()                     { r.Layout(r.overlay.Size()) }
func (r *selectionRenderer) Destroy()                     {}
func (r *selectionRenderer) Objects() []fyne.CanvasObject { return []fyne.CanvasObject{r.rect} }

type interactiveArea struct {
	widget.BaseWidget
	onDrag    func(*fyne.DragEvent)
	onDragEnd func()
	onTap     func(*fyne.PointEvent)
	getCursor func() desktop.Cursor
}

func (i *interactiveArea) Dragged(e *fyne.DragEvent) { i.onDrag(e) }
func (i *interactiveArea) DragEnd()                  { i.onDragEnd() }
func (i *interactiveArea) Tapped(e *fyne.PointEvent) {
	if i.onTap != nil {
		i.onTap(e)
	}
}
func (i *interactiveArea) Cursor() desktop.Cursor {
	if i.getCursor != nil {
		return i.getCursor()
	}
	return desktop.DefaultCursor
}

func (i *interactiveArea) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(canvas.NewRectangle(color.Transparent))
}
