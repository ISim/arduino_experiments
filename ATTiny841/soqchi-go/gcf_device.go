package soqchigfc

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/ISim/Arduino/soqchigfc/firestore"
	"github.com/ISim/Arduino/soqchigfc/pubsub"
	"github.com/ISim/Arduino/soqchigfc/soqchi"
	"io"

	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"
)

const (

	// Aby cloud funkce nereagovala ne neoprávněné requesty je "zabezpečeno" klíčem.

	// envKey je název promenné prostředí s "tajným" klíčem, který zasílá Sigfox backend
	envKey = "DEVICE_KEY"

	// hKey je název HTTP hlavičky zasílané ze Sigfox backendu s tajným klíčem
	hKey = "X-AuthKey"
)

// dataDTO je struktura přicházející POSTem ze sigfox backendu jako datová zpráva
type dataDTO struct {
	DeviceID string `json:"device"`
	TS       int64  `json:"ts"`
	Data     string `json:"data"`
	Ack      bool   `json:"ack"`
}

// inPayload je interní payload (bajty ze sigfoxu)
type inPayload []byte

type soqchiEvent struct {
	ctx        context.Context
	receivedAt time.Time
	deviceID   string
	data       inPayload
	ack        bool
}

type deviceMessage struct {
	publish interface {
		PlainMessage(ctx context.Context, chats []int64, msg string) error
	}

	storage interface {
		Device(ctx context.Context, deviceID string) (*soqchi.Device, error)
		AllChats(ctx context.Context, deviceID string) ([]int64, error)
		SaveHeartbeat(ctx context.Context, deviceID string, t time.Time, voltage, temperature float64) error
		SaveTimestamp(ctx context.Context, deviceID string, t time.Time) error
	}
}

// Device je handler vyvolávaný jako HTTP Google Cloud Funkce
func Device(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "not supported", http.StatusMethodNotAllowed)
		return
	}
	if r.Header.Get(hKey) != os.Getenv(envKey) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if r.Header.Get("Content-type") != "application/json" {
		http.Error(w, "invalid content-type", http.StatusBadRequest)
		return
	}

	msg, err := decodePayload(r.Body)
	if err != nil {
		log.Printf("pyaload decoding error: %s", err.Error())
		http.Error(w, "payload error", http.StatusBadRequest)
	}

	storage, err := firestore.New(r.Context())

	if err != nil {
		log.Printf("can't initialize firestore: %s", err.Error())
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	publish, err := pubsub.NewPublisher()
	if err != nil {
		log.Printf("can't initialize pub/sub service: %s", err.Error())
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	dm := &deviceMessage{
		publish: publish,
		storage: storage,
	}

	resp, err := dm.handle(r.Context(), msg)

	if err != nil {
		log.Printf("handle device message failed: %s", err.Error())
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	if !msg.Ack {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	w.Header().Set("Content-type", "application/json")
	w.WriteHeader(http.StatusOK)

	if resp == nil {
		// pošleme nuly
		resp = &soqchi.UplinkResponse{}
	}

	// uplink vyžaduje přesně 8 bajtů
	uplinkData, _ := json.Marshal(map[string]map[string]string{
		msg.DeviceID: {
			"downlinkData": resp.Serialize(),
		},
	})

	_, _ = w.Write(uplinkData)
}

func decodePayload(r io.Reader) (*soqchi.Message, error) {
	raw, _ := ioutil.ReadAll(r)

	var data dataDTO
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, fmt.Errorf("can't parse JSON payload\n%#q\n%w", string(raw), err)
	}

	tmp, err := hex.DecodeString(data.Data)
	if err != nil {
		return nil, fmt.Errorf("can't decode hexa payload data: %s", err.Error())
	}

	payload := inPayload(tmp)
	toFloat64 := func(f func() (float64, bool)) float64 {
		v, _ := f()
		return v
	}

	return &soqchi.Message{
		DeviceID: data.DeviceID,
		At:       time.Unix(data.TS, 0).In(soqchi.TZ),
		Ack:      data.Ack,
		Flags:    tmp[0],
		Voltage:  toFloat64(payload.voltage),
		Temp:     toFloat64(payload.temperature),
	}, nil
}

func (h *deviceMessage) handle(ctx context.Context, msg *soqchi.Message) (*soqchi.UplinkResponse, error) {
	devUplink := &soqchi.UplinkResponse{}

	device, err := h.storage.Device(ctx, msg.DeviceID)
	devUplink.AccessEnabled = device.AccessAllowed

	if err != nil {
		return nil, fmt.Errorf("can't retrieve device data id=%s: %w", msg.DeviceID, err)
	}

	// vzdy se aktualizuje datum zprávy
	h.storage.SaveTimestamp(ctx, msg.DeviceID, msg.At)

	if msg.Hartbeat() {
		logErr(h.storage.SaveHeartbeat(ctx, device.ID, msg.At, msg.Voltage, msg.Temp))
	}

	if !msg.Alarm() && !msg.Info() {
		// nic se nikomu nemá posílat
		return devUplink, nil
	}

	chats, err := h.storage.AllChats(ctx, msg.DeviceID)
	if err != nil {
		return devUplink, fmt.Errorf("can't publish messages: %w", err)
	}

	if len(chats) == 0 {
		return devUplink, nil
	}

	if msg.Alarm() {
		err := h.alarm(ctx, device, msg, chats)
		if err != nil {
			return devUplink, fmt.Errorf("alarm failed: %w", err)
		}
	}

	if msg.Info() {
		err := h.info(ctx, device, msg, chats)
		if err != nil {
			return devUplink, fmt.Errorf("info failed: %w", err)
		}
	}

	return devUplink, nil
}

func (h *deviceMessage) alarm(ctx context.Context, device *soqchi.Device, msg *soqchi.Message, chats []int64) error {

	return h.publish.PlainMessage(ctx, chats,
		fmt.Sprintf("‼️ %s (%s) - ALARM %s ‼️", device.Name, device.ID, msg.At.Format("2.1. 15:04"),
		))
}

func (h *deviceMessage) info(ctx context.Context, device *soqchi.Device, msg *soqchi.Message, chats []int64) error {
	var stateTxt string
	if msg.DoorOpen() {
		stateTxt = "🅾️"
	} else {
		stateTxt = "✅"
	}
	return h.publish.PlainMessage(ctx, chats,
		fmt.Sprintf("%s %s %s 🌡 %.1f °C 🔋 %.3f V",
			device.Name,
			stateTxt,
			msg.At.Format("2.1. 15:04"),
			msg.Temp,
			msg.Voltage))
}

func (p inPayload) temperature() (float64, bool) {
	if len(p) < 4 {
		return 0, false
	}
	t, err := strconv.Atoi(string(p[1:4]))
	if err != nil {
		log.Printf("temperature %q parse error", string(p[1:4]))
		return 0, false
	}
	return float64(t) / 10, true
}

func (p inPayload) voltage() (float64, bool) {
	if len(p) < 4 {
		return 0, false
	}
	t, err := strconv.Atoi(string(p[4:]))
	if err != nil {
		log.Printf("voltage %q parse error", string(p[4:]))
		return 0, false
	}
	return float64(t) / 1000, true
}

func logErr(err error) {
	if err == nil {
		return
	}
	log.Printf("%s", err.Error())
}
