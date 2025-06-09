package model

import (
	"time"
)

type ProductRaw struct {
	Title       string    `json:"title"`
	Price       int64     `json:"price"`
	URL         string    `json:"url"`
	Marketplace string    `json:"marketplace"`
	Timestamp   time.Time `json:"timestamp"`
}

// Additional methods related to ProductRaw can be added here.
