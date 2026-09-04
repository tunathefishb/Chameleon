package tui

type Layout struct {
	Width        int
	Height       int
	TopHeight    int
	BottomHeight int
	FooterHeight int

	LeftWidth        int
	CenterWidth      int
	RightWidth       int
	BottomLeftWidth  int
	BottomRightWidth int

	// Tabbed mode layout dimensions
	TabHeaderHeight  int
	TabContentHeight int
	TabContentWidth  int
	URLBarHeight     int
}

func calculateLayout(width, height int) Layout {
	if width < 40 {
		width = 40
	}
	if height < 16 {
		height = 16
	}

	bottomHeight := 6
	footerHeight := 1
	topHeight := height - bottomHeight - footerHeight
	if topHeight < 8 {
		topHeight = 8
	}

	leftWidth := width / 4
	rightWidth := width / 4
	centerWidth := width - leftWidth - rightWidth

	bottomLeftWidth := width / 2
	bottomRightWidth := width - bottomLeftWidth

	// Tabbed layout calculations
	tabHeaderHeight := 1
	urlBarHeight := 2
	tabContentHeight := height - tabHeaderHeight - urlBarHeight - footerHeight
	if tabContentHeight < 6 {
		tabContentHeight = 6
	}
	tabContentWidth := width

	return Layout{
		Width:            width,
		Height:           height,
		TopHeight:        topHeight,
		BottomHeight:     bottomHeight,
		FooterHeight:     footerHeight,
		LeftWidth:        leftWidth,
		CenterWidth:      centerWidth,
		RightWidth:       rightWidth,
		BottomLeftWidth:  bottomLeftWidth,
		BottomRightWidth: bottomRightWidth,
		TabHeaderHeight:  tabHeaderHeight,
		TabContentHeight: tabContentHeight,
		TabContentWidth:  tabContentWidth,
		URLBarHeight:     urlBarHeight,
	}
}
