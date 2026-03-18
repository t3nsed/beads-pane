package main

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
)

type Theme struct {
	IsDark bool

	Bg       tcell.Color
	Fg       tcell.Color
	Border   tcell.Color
	SelectBg tcell.Color
	Accent   tcell.Color
	Dim      tcell.Color
	Header   tcell.Color

	Open    tcell.Color
	InProg  tcell.Color
	Blocked tcell.Color
	Closed  tcell.Color
	Ready   tcell.Color

	Pri [5]tcell.Color

	FgTag      string
	DimTag     string
	AccentTag  string
	HeaderTag  string
	OpenTag    string
	InProgTag  string
	BlockedTag string
	ClosedTag  string
	ReadyTag   string
	PriTag     [5]string
}

func hexTag(c tcell.Color) string {
	r, g, b := c.RGB()
	return fmt.Sprintf("#%02x%02x%02x", r, g, b)
}

func (t *Theme) computeTags() {
	t.FgTag = hexTag(t.Fg)
	t.DimTag = hexTag(t.Dim)
	t.AccentTag = hexTag(t.Accent)
	t.HeaderTag = hexTag(t.Header)
	t.OpenTag = hexTag(t.Open)
	t.InProgTag = hexTag(t.InProg)
	t.BlockedTag = hexTag(t.Blocked)
	t.ClosedTag = hexTag(t.Closed)
	t.ReadyTag = hexTag(t.Ready)
	for i, c := range t.Pri {
		t.PriTag[i] = hexTag(c)
	}
}

func (t *Theme) StatusColor(s string) tcell.Color {
	switch s {
	case "open":
		return t.Open
	case "in_progress":
		return t.InProg
	case "blocked":
		return t.Blocked
	case "closed":
		return t.Closed
	default:
		return t.Fg
	}
}

func (t *Theme) StatusTag(s string) string {
	switch s {
	case "open":
		return t.OpenTag
	case "in_progress":
		return t.InProgTag
	case "blocked":
		return t.BlockedTag
	case "closed":
		return t.ClosedTag
	default:
		return t.FgTag
	}
}

func (t *Theme) PriColor(p int) tcell.Color {
	if p >= 0 && p <= 4 {
		return t.Pri[p]
	}
	return t.Fg
}

func newDarkTheme() Theme {
	t := Theme{
		IsDark:   true,
		Bg:       tcell.ColorDefault,
		Fg:       tcell.NewRGBColor(230, 230, 230),
		Border:   tcell.ColorDimGray,
		SelectBg: tcell.NewRGBColor(40, 50, 65),
		Accent:   tcell.NewRGBColor(135, 175, 175),
		Dim:      tcell.NewRGBColor(110, 110, 110),
		Header:   tcell.NewRGBColor(255, 215, 0),
		Open:     tcell.NewRGBColor(80, 200, 80),
		InProg:   tcell.NewRGBColor(95, 135, 255),
		Blocked:  tcell.NewRGBColor(230, 60, 60),
		Closed:   tcell.NewRGBColor(136, 136, 136),
		Ready:    tcell.NewRGBColor(80, 200, 80),
		Pri: [5]tcell.Color{
			tcell.NewRGBColor(230, 60, 60),
			tcell.NewRGBColor(255, 135, 0),
			tcell.NewRGBColor(230, 210, 40),
			tcell.NewRGBColor(80, 200, 80),
			tcell.NewRGBColor(136, 136, 136),
		},
	}
	t.computeTags()
	return t
}

func newLightTheme() Theme {
	t := Theme{
		IsDark:   false,
		Bg:       tcell.NewRGBColor(252, 252, 250),
		Fg:       tcell.NewRGBColor(30, 30, 30),
		Border:   tcell.NewRGBColor(190, 190, 190),
		SelectBg: tcell.NewRGBColor(210, 222, 240),
		Accent:   tcell.NewRGBColor(50, 100, 120),
		Dim:      tcell.NewRGBColor(145, 145, 145),
		Header:   tcell.NewRGBColor(180, 130, 0),
		Open:     tcell.NewRGBColor(20, 130, 20),
		InProg:   tcell.NewRGBColor(40, 70, 190),
		Blocked:  tcell.NewRGBColor(195, 30, 30),
		Closed:   tcell.NewRGBColor(130, 130, 130),
		Ready:    tcell.NewRGBColor(20, 130, 20),
		Pri: [5]tcell.Color{
			tcell.NewRGBColor(195, 30, 30),
			tcell.NewRGBColor(190, 95, 0),
			tcell.NewRGBColor(155, 130, 0),
			tcell.NewRGBColor(20, 130, 20),
			tcell.NewRGBColor(145, 145, 145),
		},
	}
	t.computeTags()
	return t
}
