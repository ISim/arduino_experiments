package soqchigfc

import (
	gps "cloud.google.com/go/pubsub"
	"context"
	"fmt"
	"github.com/ISim/Arduino/soqchigfc/firestore"
	"github.com/ISim/Arduino/soqchigfc/pubsub"
	"github.com/ISim/Arduino/soqchigfc/soqchi"
	"time"
)

type watchdog struct {
	ctx     context.Context
	storage interface {
		AllDevices(ctx context.Context) ([]*soqchi.Device, error)
		AllChats(ctx context.Context, deviceID string) ([]int64, error)
	}
	publish interface {
		PlainMessage(ctx context.Context, chats []int64, msg string) error
	}
}

func Watchdog(ctx context.Context, m gps.Message) error {

	// irrelevantni - je nám jedno, co je v payloadu
	_ = m

	c, err := firestore.New(ctx)
	if err != nil {
		return fmt.Errorf("can't initialize firestore client: %w", err)
	}

	pub, err := pubsub.NewPublisher()
	if err != nil {
		return err
	}

	w := &watchdog{
		ctx:     ctx,
		storage: c,
		publish: pub,
	}

	return w.handle()
}

func (w *watchdog) handle() error {
	limit := time.Now().Add(-25 * time.Hour)

	devices, err := w.storage.AllDevices(w.ctx)
	if err != nil {
		return fmt.Errorf("can' retrieve devides list: %w", err)
	}

	for _, device := range devices {

		withChat := func(f func(ctx context.Context, device *soqchi.Device, chats []int64) error) error {
			chats, err := w.storage.AllChats(w.ctx, device.ID)
			if err != nil {
				return err
			}
			return f(w.ctx, device, chats)
		}

		switch {
		case device.LastHeartbeatAt.IsZero():
			err = withChat(w.noHeartbeat)
		case device.LastHeartbeatAt.Before(limit):
			err = withChat(w.heartbeatMissing)
		case device.Voltage < soqchi.VoltageLimit:
			err = withChat(w.lowVoltage)
		}

		if err != nil {
			return fmt.Errorf("can't publish heartbeat missing event for device %s: %w", device.ID, err)
		}
	}

	return nil
}

func (w *watchdog) noHeartbeat(ctx context.Context, device *soqchi.Device, chats []int64) error {
	return w.publish.PlainMessage(ctx, chats, fmt.Sprintf("⚠️ zařízení %s (%s) se dosud neohlásilo",
		device.Name,
		device.ID,
	))
}

func (w *watchdog) heartbeatMissing(ctx context.Context, device *soqchi.Device, chats []int64) error {
	return w.publish.PlainMessage(ctx, chats, fmt.Sprintf("⚠️ zařízení %s (%s) se neohlásilo od %s",
		device.Name,
		device.ID,
		device.LastMessageAt.Format("2.1. 15:04"),
	))
}

func (w *watchdog) lowVoltage(ctx context.Context, device *soqchi.Device, chats []int64) error {

	return w.publish.PlainMessage(ctx, chats, fmt.Sprintf("⚠️ zařízení %s (%s) má nízké napětí baterie %.3f",
		device.Name,
		device.ID,
		device.Voltage,
	))
}
