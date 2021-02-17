package soqchigfc

import (
	"bytes"
	"context"
	"fmt"
	"github.com/ISim/Arduino/soqchigfc/firestore"
	"github.com/ISim/Arduino/soqchigfc/soqchi"
	"github.com/ISim/Arduino/soqchigfc/telegram"
	"github.com/wcharczuk/go-chart/v2"
	"github.com/wcharczuk/go-chart/v2/drawing"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	UrlParamTelegramKey = "k"
)

type telegramUpdate struct {
	botRq interface {
		FromUser() string
		ChatID() int64
		Command() (string, string)
		SendImage(chatID int64, name string, img io.Reader, size int64) error
	}
	storage interface {
		AddUser(ctx context.Context, deviceID string, chatID int64, username string) error
		DeviceInfo(ctx context.Context, deviceID string) (*soqchi.DeviceInfo, error)
	}
}

func TelegramHTTPReceiver(w http.ResponseWriter, r *http.Request) {

	if k := r.URL.Query().Get(UrlParamTelegramKey); k == "" || k != os.Getenv(telegram.EnvTelegramWebhookKey) {
		log.Printf("unauthorized request from %s", r.Host)
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	w.WriteHeader(http.StatusNoContent)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*20)
	defer cancel()

	msg, err := telegram.NewUpdate(r.Body)
	if err != nil {
		log.Printf("telegram update error: %s", err.Error())
		return
	}

	storage, err := firestore.New(ctx)
	if err != nil {
		log.Printf("firestore initialization failed error: %s", err.Error())
		return
	}

	tMsg := &telegramUpdate{
		botRq:   msg,
		storage: storage,
	}

	if err := tMsg.handle(ctx); err != nil {
		log.Printf("telegram message processing failed: %s", err.Error())
		return
	}
}

func (a *telegramUpdate) handle(ctx context.Context, ) error {
	cmd, argLine := a.botRq.Command()
	_ = argLine

	switch cmd {
	case "register":
		return a.cmdRegister(ctx, argLine)
	case "voltage":
		return a.cmdVoltageChart(ctx, argLine)
	}
	return nil
}

func (a *telegramUpdate) cmdVoltageChart(ctx context.Context, argLine string) error {
	deviceID := strings.Trim(argLine, " \n\t\r\"")
	if deviceID == "" {
		// tiše vymlčíme - kdo neví, co zadat, ať nevidí graf
		return nil
	}

	info, err := a.storage.DeviceInfo(ctx, strings.Trim(argLine, " \n\t\r\""))
	if err != nil {
		return err
	}
	var png = bytes.NewBuffer(nil)
	err = voltageChart(info.HeartBeats, png)
	if err != nil {
		return fmt.Errorf("graph creation failed: %w", err)
	}

	return a.botRq.SendImage(a.botRq.ChatID(), "stat", png, int64(png.Len()))
}

func (a *telegramUpdate) cmdRegister(ctx context.Context, argLine string) error {

	return a.storage.AddUser(ctx, strings.Trim(argLine, " \n\t\r\""), a.botRq.ChatID(), a.botRq.FromUser())
}

func roundUnix(t time.Time) float64 {
	return float64(time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, soqchi.TZ).Unix())
}

func genTicks(min, max, step float64) []chart.Tick {
	var ticks []chart.Tick
	tmp := min
	var format = "%.1f"
	if step == math.Floor(step) {
		format = "%.f"
	}
	for ; tmp <= max; tmp += step {
		ticks = append(ticks, chart.Tick{
			Value: tmp,
			Label: fmt.Sprintf(format, tmp),
		})
	}
	return ticks
}

func voltageChart(data soqchi.Heartbeats, w io.Writer) error {
	var (
		XOsa, YVolt []float64
		iMin        int
	)

	for i, v := range data {
		if data[iMin].Voltage >= v.Voltage {
			iMin = i
		}
		nD := roundUnix(v.At)

		l := len(XOsa)
		if l > 0 {
			pD := XOsa[l-1]
			if nD == pD {
				YVolt[l-1] = (YVolt[l-1] + v.Voltage) / 2
				continue
			}
			daysWithoutRecod := 0
			tmpD := pD
			for tmpD+float64(26*time.Hour) < nD {
				daysWithoutRecod++
				tmpD += float64(24 * time.Hour)
			}
			kV := (v.Voltage - YVolt[l-1]) / float64(daysWithoutRecod+1)
			for i := 1; i <= daysWithoutRecod; i++ {
				XOsa = append(XOsa, pD+float64(i*24*int(time.Hour)))
				YVolt = append(YVolt, float64(i)*kV+YVolt[l-1])
			}

		}

		XOsa = append(XOsa, nD)
		YVolt = append(YVolt, v.Voltage)
	}

	graph := chart.Chart{
		Background: chart.Style{
			Padding: chart.Box{
				Top:    50,
				Left:   10,
				Right:  25,
				Bottom: 10,
			},
			FillColor: drawing.ColorFromHex("eeeeee"),
		},
		XAxis: chart.XAxis{
			Name:         "Time",
			TickPosition: chart.TickPositionBetweenTicks,
			ValueFormatter: func(v interface{}) string {
				vf := v.(float64)
				t := time.Unix(int64(vf), 0)
				return t.Format("02.01")
			},
		},

		YAxis: chart.YAxis{
			Name: " ",
			NameStyle: chart.Style{
				TextRotationDegrees: 270,
			},
			Style: chart.Style{
				Padding:   chart.NewBox(10, 10, 10, 10),
				FillColor: drawing.ColorFromHex("efefef"),
			},

			TickStyle: chart.Style{
				TextRotationDegrees: 315,
			},

			Ticks: genTicks(1.5, 3.5, 0.5),
		},

		Series: []chart.Series{

			chart.ContinuousSeries{
				Name:    "Voltage",
				YAxis:   chart.YAxisPrimary,
				XValues: XOsa,
				Style: chart.Style{
					StrokeColor: drawing.ColorFromHex("008800"),
					FillColor:   drawing.ColorFromHex("CCFFCC"),
				},
				YValues: YVolt,
			},
			chart.AnnotationSeries{
				YAxis: chart.YAxisPrimary,
				Annotations: []chart.Value2{
					{
						XValue: roundUnix(data[iMin].At),
						YValue: data[iMin].Voltage,
						Label:  fmt.Sprintf("Min %.2fV", data[iMin].Voltage),
					},
				},
			},
		},
	}
	graph.Elements = []chart.Renderable{
		chart.Legend(&graph),
	}

	err := graph.Render(chart.PNG, w)
	return err
}
