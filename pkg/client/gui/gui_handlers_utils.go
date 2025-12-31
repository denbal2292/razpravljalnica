package gui

import (
	"fmt"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// displayStatus updates the status bar with a message and color for 3s
func (gc *guiClient) displayStatus(message string, color string) {
	go func() {
		formattedMessage := fmt.Sprintf("[%s]%s[-]", color, message)

		// Refresh screen on status update
		gc.app.QueueUpdateDraw(func() {
			gc.statusBar.SetText(formattedMessage)
		})
		time.Sleep(3 * time.Second)
		gc.app.QueueUpdateDraw(func() {
			gc.statusBar.SetText("[green]Povezan[-]")
		})
	}()
}

func createButton(text string, color tcell.Color, activeColor tcell.Color, textColor tcell.Color, activeTextColor tcell.Color) *tview.Button {
	btn := tview.NewButton(text)

	inactiveStyle := tcell.StyleDefault.Background(color).Foreground(textColor)
	activeStyle := tcell.StyleDefault.Background(activeColor).Foreground(activeTextColor)

	// Set styles in order for tview to not override them with defaults
	btn.SetStyle(inactiveStyle)
	btn.SetActivatedStyle(activeStyle)

	return btn
}
