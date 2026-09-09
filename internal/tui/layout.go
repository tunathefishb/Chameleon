package tui

type Layout struct {
	Width            int
	Height           int
	TabHeaderHeight  int
	TabContentHeight int
	TabContentWidth  int
	URLBarHeight     int
	FooterHeight     int
}

func calculateLayout(width, height int) Layout {
	if width < 40 {
		width = 40
	}
	if height < 16 {
		height = 16
	}

	tabHeaderHeight := 1
	urlBarHeight := 2
	footerHeight := 1
	tabContentHeight := height - tabHeaderHeight - urlBarHeight - footerHeight
	if tabContentHeight < 6 {
		tabContentHeight = 6
	}
	tabContentWidth := width

	return Layout{
		Width:            width,
		Height:           height,
		TabHeaderHeight:  tabHeaderHeight,
		TabContentHeight: tabContentHeight,
		TabContentWidth:  tabContentWidth,
		URLBarHeight:     urlBarHeight,
		FooterHeight:     footerHeight,
	}
}
