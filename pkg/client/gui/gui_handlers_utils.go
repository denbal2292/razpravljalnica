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

// showModal displays a modal dialog with the given text and buttons.
func (gc *guiClient) showModal(text string, buttons []string, doneFunc func(buttonIndex int, buttonLabel string)) {
	modal := tview.NewModal().
		SetBackgroundColor(tcell.ColorBlack).
		SetTextColor(tcell.ColorWhite).
		SetText(text).
		AddButtons(buttons).
		SetDoneFunc(func(buttonIndex int, buttonLabel string) {
			// Just pop the modal from the pages
			gc.pages.RemovePage("modal")
			if doneFunc != nil {
				doneFunc(buttonIndex, buttonLabel)
			}
		})

	gc.pages.AddPage("modal", modal, true, true)
	gc.app.SetFocus(modal)
}
