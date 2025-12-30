package gui

import (
	"fmt"
	"time"
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
