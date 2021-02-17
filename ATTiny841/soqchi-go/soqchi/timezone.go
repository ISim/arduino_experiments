package soqchi

import "time"

var (
	TZ *time.Location
)


func init() {
	TZ, _ = time.LoadLocation("Europe/Prague")
}