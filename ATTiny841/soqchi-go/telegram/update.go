package telegram

import (
	"encoding/json"
	"fmt"
	tba "github.com/go-telegram-bot-api/telegram-bot-api"
	"io"
)

type Update struct {
	u *tba.Update
}

func NewUpdate(r io.Reader) (*Update, error) {
	var u tba.Update

	if err := json.NewDecoder(r).Decode(&u); err != nil {
		return nil, fmt.Errorf("bot update unmarshal error: %w", err)
	}

	return &Update{
		u: &u,
	}, nil

}

// Command vrátí příkaz zaslaný botovi a zbytek argumentů jako string. Pokud
// update nebyl command, vrací prázdné řetězce
func (u *Update) Command() (string, string) {
	return u.u.Message.Command(), u.u.Message.CommandArguments()
}

func (u *Update) FromUser() string {
	return u.u.Message.Chat.UserName
}

func (u *Update) ChatID() int64 {
	return u.u.Message.Chat.ID
}

func (s *Update) SendImage(chatID int64, name string, img io.Reader, size int64) error {
	fr := tba.FileReader{
		Name:   name,
		Reader: img,
		Size:   size,
	}
	uploader := tba.NewPhotoUpload(chatID, fr)
	uploader.DisableNotification = true
	_, err := botAPI.Send(uploader)
	return err
}
