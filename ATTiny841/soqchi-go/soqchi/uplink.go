package soqchi

import "encoding/hex"

type UplinkResponse struct {
	AccessEnabled bool
}


func (u *UplinkResponse) Serialize() string {
	r := make([]byte, 8)
	if  u.AccessEnabled {
		r[0] = 0x01
	}
	return hex.EncodeToString(r)
}