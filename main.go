package main

import (
	"fmt"
	"github.com/rivo/tview"
	"os"
)

func main() {
	app := tview.NewApplication()

	// Create a text view
	textView := tview.NewTextView().
		SetText("Use arrow keys to move.\nPress 'q' to quit").
		SetTextAlign(tview.AlignCenter).
		SetDynamicColors(true)

	// Create a grid layout and add the text view to it
	grid := tview.NewGrid().
		SetRows(0).
		SetColumns(0).
		AddItem(textView, 0, 0, 1, 1, 0, 0, true)

	if err := app.SetRoot(grid, true).Run(); err != nil {
		fmt.Fprintf(os.Stderr, "%v", err)
	}
}

