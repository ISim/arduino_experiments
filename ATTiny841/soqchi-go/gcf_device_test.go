package soqchigfc

import (
	"testing"
)

func TestTemperatureParsing(t *testing.T) {
	s := &soqchiEvent{
		data: []byte{0x00, 0x32, 0x36, 0x31, 0x35, 0x38, 0x32, 0x35},
	}

	temp, ok := s.data.temperature()
	if !ok {
		t.Fatal("parse error")
	}
	if exp := 26.1; temp != exp {
		t.Errorf("expected temperature %f, got %f", exp, temp)
	}
}

func TestVoltageParsing(t *testing.T) {
	s := &soqchiEvent{
		data: []byte{0x00, 0x32, 0x36, 0x31, 0x35, 0x38, 0x32, 0x35},
	}

	v, ok := s.data.voltage()
	if !ok {
		t.Fatal("parse error")
	}
	if exp := 5.825; v != exp {
		t.Errorf("expected voltage %f, got %f", exp, v)
	}
}

