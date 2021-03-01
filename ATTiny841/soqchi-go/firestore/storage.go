package firestore

import (
	"cloud.google.com/go/firestore"
	"context"
	"fmt"
	"github.com/ISim/Arduino/soqchigfc/soqchi"
	"google.golang.org/api/iterator"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"strconv"
	"time"
)

type Client struct {
	c *firestore.Client
}

type Chat struct {
	Username  string
	CreatedAt time.Time
}

type Device struct {
	ID              string
	Name            string
	LastMessageAt   time.Time
	LastHeartbeatAt time.Time
	AccessAllowed   bool
	Voltage         float64
}

type Heartbeat struct {
	ReceivedAt time.Time
	Voltage    float64
	Temp       float64
}

func New(ctx context.Context) (*Client, error) {
	c, err := app.Firestore(ctx)
	if err != nil {
		return nil, err
	}
	return &Client{c: c}, nil
}

func (c *Client) AddUser(ctx context.Context, deviceID string, chatID int64, username string) error {
	_, err := c.c.Collection(collectionDevices).Doc(deviceID).Get(ctx)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return nil
		}
		return err
	}

	_, err = c.c.Collection(collectionDevices).Doc(deviceID).Collection(collectionChats).Doc(fmt.Sprintf("%d", chatID)).Set(ctx, Chat{
		Username:  username,
		CreatedAt: time.Now(),
	})
	return err
}

func (c *Client) Device(ctx context.Context, deviceID string) (*soqchi.Device, error) {
	d, err := c.c.Collection(collectionDevices).Doc(deviceID).Get(ctx)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return nil, nil
		}
		return nil, err
	}

	return unwrapDevice(d)
}

func (c *Client) AllChats(ctx context.Context, deviceID string) ([]int64, error) {
	iter := c.c.Collection(collectionDevices).Doc(deviceID).Collection(collectionChats).Documents(ctx)
	var chats []int64
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("retrieve all chats failed: %w", err)
		}
		if id, err := strconv.ParseInt(doc.Ref.ID, 10, 64); err == nil {
			chats = append(chats, id)
		}
	}
	return chats, nil
}

func (c *Client) SaveHeartbeat(ctx context.Context, deviceID string, t time.Time, voltage, temperature float64) error {
	_, err := c.c.Collection(collectionDevices).Doc(deviceID).Update(ctx, []firestore.Update{
		{Path: "LastMessageAt", Value: t}, {Path: "LastHeartbeatAt", Value: t}, {Path: "Voltage", Value: voltage}})

	if err != nil {
		return err
	}

	_, err = c.c.Collection(collectionDevices).Doc(deviceID).Collection(collectionHeartbeats).Doc(t.UTC().Format(time.RFC3339)).Set(ctx, Heartbeat{
		ReceivedAt: t,
		Voltage:    voltage,
		Temp:       temperature,
	})

	return err
}

func (c *Client) SaveTimestamp(ctx context.Context, deviceID string, t time.Time) error {
	_, err := c.c.Collection(collectionDevices).Doc(deviceID).Update(ctx, []firestore.Update{
		{Path: "LastMessageAt", Value: t}})
	return err
}

func (c *Client) DeviceInfo(ctx context.Context, deviceID string) (*soqchi.DeviceInfo, error) {
	it := c.c.Collection("devices").Doc(deviceID).Collection("Heartbeats").
		OrderBy("ReceivedAt", firestore.Asc).Limit(60).Documents(ctx)
	defer it.Stop()

	var inf = soqchi.DeviceInfo{HeartBeats: soqchi.Heartbeats{}}

	for {
		d, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("heartbeats for (%s) failed: %w", deviceID, err)
		}

		var h Heartbeat
		if err := d.DataTo(&h); err != nil {
			return nil, fmt.Errorf("heartbeat decoding failed: %w", err)
		}

		inf.HeartBeats = append(inf.HeartBeats, soqchi.Heartbeat{
			At:          h.ReceivedAt,
			Voltage:     h.Voltage,
			Temperature: h.Temp,
		})

	}
	return &inf, nil
}

func (c *Client) AllDevices(ctx context.Context) ([]*soqchi.Device, error) {
	var result []*soqchi.Device

	devices, err := c.c.Collection(collectionDevices).Documents(ctx).GetAll()

	if err != nil {
		return nil, fmt.Errorf("can't retrieve devices: %w", err)
	}

	for _, d := range devices {

		deviceInfo, err := unwrapDevice(d)

		if err != nil {
			return nil, err
		}
		result = append(result, deviceInfo)
	}
	return result, nil
}

func unwrapDevice(f *firestore.DocumentSnapshot) (*soqchi.Device, error) {
	var dev Device
	if err := f.DataTo(&dev); err != nil {
		return nil, err
	}

	return &soqchi.Device{
		ID:              f.Ref.ID,
		Name:            dev.Name,
		LastMessageAt:   dev.LastMessageAt,
		LastHeartbeatAt: dev.LastHeartbeatAt,
		AccessAllowed:   dev.AccessAllowed,
		Voltage:         dev.Voltage,
	}, nil
}
