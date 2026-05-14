package screens

const (
	splitGap           = 2
	minPaneWidth       = 24
	preferredListWidth = 34
)

func splitPaneSizes(totalWidth, totalHeight int) (listWidth, detailWidth, paneHeight int) {
	listWidth, detailWidth = splitPaneWidths(totalWidth)
	paneHeight = max(1, totalHeight-4)
	return listWidth, detailWidth, paneHeight
}

func splitPaneWidths(totalWidth int) (listWidth, detailWidth int) {
	if totalWidth <= 0 {
		return 0, 0
	}
	if totalWidth <= splitGap {
		return totalWidth, 0
	}

	usable := totalWidth - splitGap
	if usable < minPaneWidth*2 {
		listWidth = max(1, usable/2)
		detailWidth = usable - listWidth
		return listWidth, max(0, detailWidth)
	}

	listWidth = min(preferredListWidth, usable/3)
	listWidth = max(minPaneWidth, listWidth)
	if usable-listWidth < minPaneWidth {
		listWidth = usable - minPaneWidth
	}

	detailWidth = usable - listWidth
	return max(0, listWidth), max(0, detailWidth)
}
