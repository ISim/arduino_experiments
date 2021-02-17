package soqchigfc

import (
	"cloud.google.com/go/pubsub"
	"context"
	"encoding/json"
	"fmt"
	"github.com/ISim/Arduino/soqchigfc/soqchi"
	"github.com/ISim/Arduino/soqchigfc/telegram"
)

func PlainTelegramMessage(ctx context.Context, m pubsub.Message) error {
	var msgRequest soqchi.PlainMessage
	err := json.Unmarshal(m.Data, &msgRequest)

	if err != nil {
		return fmt.Errorf("can't unmarshal %q as %T: %w", string(m.Data), msgRequest, err)
	}

	return telegram.NewSender().Send(msgRequest.Chats, msgRequest.Message)

}
