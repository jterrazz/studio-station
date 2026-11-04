package accounts

import "time"

const (
	DefaultShareTerm = 12 * time.Hour
	ShortestShare    = time.Minute
	LongestShare     = 7 * 24 * time.Hour
)

type Share struct {
	Name  string    `json:"name"`
	At    time.Time `json:"at"`
	Until time.Time `json:"until"`
}

type Lending struct {
	LendList []string
	Lent     map[string][]string
	Lends    map[string][]Share
	Kept     []string
}

type LendReport struct {
	To    string
	Names []string
	At    time.Time
	Until time.Time
}

type RecallReport struct {
	From     map[string][]string
	Warnings []string
}
