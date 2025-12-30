package gui

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/denbal2292/razpravljalnica/pkg/client/shared"
	pb "github.com/denbal2292/razpravljalnica/pkg/pb"
)

func (gc *guiClient) handleCreateUser() {
	// Extract the user name from the input field
	userName := strings.TrimSpace(gc.newUserInput.GetText())

	if userName == "" {
		gc.displayStatus("Ime uporabnika ne sme biti prazno", "red")
		return
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), shared.Timeout)
		defer cancel()

		user, err := gc.clients.Writes.CreateUser(ctx, &pb.CreateUserRequest{
			Name: userName,
		})

		if err != nil {
			gc.displayStatus("Napaka pri ustvarjanju uporabnika", "red")
			return
		} else {
			gc.displayStatus("Uporabnik uspešno ustvarjen", "green")
		}

		// Clear the input field after processing
		gc.app.QueueUpdateDraw(func() {
			// Set the user ID for future operations
			gc.clientMu.Lock()
			gc.userId = user.Id
			gc.clientMu.Unlock()

			gc.newUserInput.SetText("")
			gc.loggedInUserView.SetText(fmt.Sprintf("[green]Prijavljen kot[-]: [yellow]%s[-]", user.Name))
		})
	}()
}

func (gc *guiClient) handleLogInUser() {
	// Extract the user name from the input field
	userId := strings.TrimSpace(gc.logInUserInput.GetText())

	if userId == "" {
		gc.displayStatus("ID uporabnika ni bil podan", "red")
		return
	}

	go func() {
		// Set the user ID for future operations
		id, err := strconv.ParseInt(userId, 10, 64)

		// Probably caused by an overflow
		if err != nil {
			gc.displayStatus("Neveljaven ID uporabnika", "red")
			return
		}

		userName, err := gc.getUserName(id)
		if err != nil {
			gc.displayStatus("Napaka pri prijavi uporabnika", "red")
			return
		}

		gc.app.QueueUpdateDraw(func() {
			// We can safely set the user ID now
			gc.clientMu.Lock()
			gc.userId = id
			gc.clientMu.Unlock()
			gc.logInUserInput.SetText("")
			gc.loggedInUserView.SetText(fmt.Sprintf("[green]Prijavljen kot[-]: [yellow]%s[-]", userName))
			gc.displayStatus("Uspešna prijava uporabnika", "green")
		})
	}()
}

func (gc *guiClient) getUserName(id int64) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), shared.Timeout)
	defer cancel()

	user, err := gc.clients.Reads.GetUser(ctx, &pb.GetUserRequest{
		UserId: id,
	})

	if err != nil {
		return "", err
	}

	return user.Name, nil
}
