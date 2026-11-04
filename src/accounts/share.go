package accounts

import "time"

const DefaultShareTerm = 12 * time.Hour

type Share struct {
	Name  string    `json:"name"`
	At    time.Time `json:"at"`
	Until time.Time `json:"until"`
}
