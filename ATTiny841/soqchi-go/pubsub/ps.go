package pubsub

import (
	"cloud.google.com/go/pubsub"
	"context"
	"encoding/json"
	"fmt"
	"github.com/ISim/Arduino/soqchigfc/soqchi"
	"os"
	"sync"
)

var (
	client          *pubsub.Client
	onceClient      sync.Once
	clientInitError error
)

type PubSub struct {
}

func NewPublisher() (*PubSub, error) {
	onceClient.Do(func() {
		client, clientInitError = pubsub.NewClient(context.Background(), os.Getenv("GOOGLE_CLOUD_PROJECT"))
	})
	if clientInitError != nil {
		return nil, fmt.Errorf("pub/sub client initialization error: %w", clientInitError)
	}
	return &PubSub{}, nil
}

func (p *PubSub) PlainMessage(ctx context.Context, chats []int64, msg string) error {
	raw, err := json.Marshal(soqchi.PlainMessage{
		Chats:   chats,
		Message: msg,
	})

	if err != nil {
		return fmt.Errorf("can't marshal data for pub/sub: %w", err)
	}

	res := client.Topic(soqchi.PlainMessageTopic).Publish(ctx, &pubsub.Message{
		Data: raw,
	})

	_, err = res.Get(ctx)

	if err != nil {
		return fmt.Errorf("publish to %q failed: %w", soqchi.PlainMessageTopic, err)
	}
	return nil
}
