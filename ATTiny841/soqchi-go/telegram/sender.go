package telegram

import (
	tba "github.com/go-telegram-bot-api/telegram-bot-api"
)

type Sender struct {
}

func NewSender() *Sender {
	return &Sender{}
}

func (s *Sender) Send(chats []int64, msg string) error {
	var err error

	for _, c := range chats {
		tMsg := tba.NewMessage(c, msg)
		_, e := botAPI.Send(tMsg)
		if err == nil {
			err = e
		}
	}
	return err
}
