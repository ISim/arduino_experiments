package soqchi

import (
	"time"
)

type Status string

const (
	flgAlarm    byte = 0x80
	flgInfo     byte = 0x40
	flgHartbeat byte = 0x20
	flgDoorOpen byte = 0x01
)

func (s Status) String() string {
	return string(s)
}

type Message struct {
	DeviceID string    `json:"deviceID"`
	At       time.Time `json:"receivedAt"`
	Ack      bool      `json:"ack"`
	Flags    byte      `json:"flags"`
	Voltage  float64   `json:"voltage,omitempty"`
	Temp     float64   `json:"temp,omitempty"`
}

func (m *Message) Alarm() bool {
	return m.Flags&flgAlarm != 0
}

func (m *Message) Info() bool {
	return m.Flags&flgInfo != 0
}

func (m *Message) Hartbeat() bool {
	return m.Flags&flgHartbeat != 0
}

func (m *Message) DoorOpen() bool {

	return m.Flags&flgDoorOpen != 0
}
