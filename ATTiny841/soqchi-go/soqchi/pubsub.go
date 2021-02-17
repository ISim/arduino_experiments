package soqchi

const (

	PlainMessageTopic = "PlainMessage2Telegram"
)

type PlainMessage struct {
	Chats   []int64 `json:"chats"`
	Message string  `json:"message"`
}
