package soqchi

import "time"

type Device struct {
	ID string
	Name string
	Voltage float64
	AccessAllowed bool
	LastMessageAt time.Time
	LastHeartbeatAt time.Time
}

type Heartbeat struct {
	At time.Time
	Voltage, Temperature float64
}

type Heartbeats []Heartbeat

type DeviceInfo struct {
	 HeartBeats Heartbeats
}