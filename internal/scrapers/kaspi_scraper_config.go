package scrapers

// DefaultKaspiConfig returns a KaspiScraperConfig with sensible defaults
import (
	"runtime"
	"time"
)

// KaspiScraperConfig holds Kaspi scraper configuration.
type KaspiScraperConfig struct {
	Category    string      // e.g. "smartphones"
	City        int         // e.g. 750000000
	MaxPages    int         // upper limit for pages
	WorkerCount int         // default = runtime.NumCPU()
	Retry       RetryConfig // retry config
}

// RetryConfig holds options for retry logic.
type RetryConfig struct {
	Attempts       int           // number of retry attempts
	InitialBackoff time.Duration // initial backoff duration between retries
}

// DefaultKaspiConfig returns a KaspiScraperConfig with sensible defaults
func DefaultKaspiConfig() KaspiScraperConfig {
	return KaspiScraperConfig{
		Category:    "smartphones",
		City:        750000000, // Almaty by default
		MaxPages:    300,       // Reasonable upper limit
		WorkerCount: runtime.NumCPU(),
		Retry: RetryConfig{
			Attempts:       3,
			InitialBackoff: 1 * time.Second,
		},
	}
}

// WithCategory sets the category and returns the config for chaining
func (c KaspiScraperConfig) WithCategory(category string) KaspiScraperConfig {
	c.Category = category
	return c
}

// WithCity sets the city code and returns the config for chaining
func (c KaspiScraperConfig) WithCity(cityCode int) KaspiScraperConfig {
	c.City = cityCode
	return c
}

// WithMaxPages sets the maximum number of pages to scrape
func (c KaspiScraperConfig) WithMaxPages(pages int) KaspiScraperConfig {
	c.MaxPages = pages
	return c
}

// WithWorkerCount sets the number of concurrent workers
func (c KaspiScraperConfig) WithWorkerCount(workers int) KaspiScraperConfig {
	c.WorkerCount = workers
	return c
}

// WithRetry sets the retry configuration
func (c KaspiScraperConfig) WithRetry(attempts int, initialBackoff time.Duration) KaspiScraperConfig {
	c.Retry = RetryConfig{
		Attempts:       attempts,
		InitialBackoff: initialBackoff,
	}
	return c
}
