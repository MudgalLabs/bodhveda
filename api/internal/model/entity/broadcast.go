package entity

import "encoding/json"

type Broadcast struct {
	ID int `json:"id"`
}

func NewBroadcast(projectID int, payload json.RawMessage, channel string, topic string, event string) *Broadcast {
	return &Broadcast{}
}
