package model

import (
	"time"
)

type ProductRaw struct {
	Title       string    `json:"title"`
	Price       int64     `json:"price"`
	URL         string    `json:"url"`
	Marketplace string    `json:"marketplace"`
	Category    string    `json:"category"`
	Timestamp   time.Time `json:"timestamp"`
}
