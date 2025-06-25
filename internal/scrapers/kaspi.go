package scrapers

import (
	"compress/flate"
	"compress/gzip"
	"strings"

	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"regexp"
	"runtime"
	"sync/atomic"
	"time"

	"mpt-parser-worker/internal/kafka"
	"mpt-parser-worker/internal/logger"
	"mpt-parser-worker/internal/model"

	"github.com/andybalholm/brotli"
)

type Scraper interface {
	Scrape() (int64, error)
}

const (
	// brandFilterPattern matches the manufacturer filter data in HTML response
	brandFilterPattern = `"id"\s*:\s*"manufacturerName".*?"rows"\s*:\s*(\[[\s\S]*?\])`
)

// KaspiScraper implements the Scraper interface.
type KaspiScraper struct {
	cfg      KaspiScraperConfig
	producer kafka.Producer
	logger   logger.Logger
}

// BrandFilter represents the manufacturer filter data structure
type BrandFilter struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// NewKaspiScraper constructs a new KaspiScraper.
func NewKaspiScraper(cfg KaspiScraperConfig, producer kafka.Producer, logger logger.Logger) *KaspiScraper {
	// If WorkerCount not set, use runtime.NumCPU().
	if cfg.WorkerCount <= 0 {
		cfg.WorkerCount = runtime.NumCPU()
	}
	return &KaspiScraper{
		cfg:      cfg,
		producer: producer,
		logger:   logger,
	}
}

// Scrape performs the main scraping process.
func (s *KaspiScraper) Scrape() (int64, error) {
	s.logger.Infof("Starting Kaspi scraping for category %s...", s.cfg.Category)

	// Create a root context with timeout for the entire scrape.
	rootCtx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	// Extract brands from the category page
	brands, err := s.extractBrands()
	if err != nil {
		s.logger.Errorf("Failed to extract brands: %v", err)
		return 0, fmt.Errorf("failed to extract brands: %w", err)
	}

	// Prepare a channel for brand tasks.
	brandsCh := make(chan string, len(brands))

	// Fill brands channel.
	go func() {
		defer close(brandsCh)
		for _, brand := range brands {
			brandsCh <- brand
		}
	}()

	// We will count total products across all brands.
	var totalCount int64

	// Worker function for processing brands.
	workerFn := func(ctx context.Context, brand string) error {
		s.logger.Infof("Worker started for brand: %s", brand)

		// Process pages for this brand until we hit a page with no products
		for page := 0; page < s.cfg.MaxPages; page++ {
			// Build the URL for this page and brand
			urlStr := s.buildURL(s.cfg.Category, s.cfg.City, page, "", brand)

			// Retry fetch logic.
			var pageData []byte
			retryErr := Retry(ctx, s.cfg.Retry, func() error {
				b, err := s.fetchProductsPage(ctx, urlStr)
				if err != nil {
					s.logger.Warnf("Retry warning: fetch failed for brand %s, page %d: %v", brand, page, err)
					return err
				}
				pageData = b
				return nil
			})

			if retryErr != nil {
				// All retry attempts failed.
				s.logger.Errorf("All retries failed for brand %s, page %d: %v", brand, page, retryErr)

				// If it's a network error or 5xx, we stop this brand
				if errors.Is(retryErr, context.DeadlineExceeded) ||
					strings.Contains(retryErr.Error(), "5") {
					return fmt.Errorf("failed to fetch brand %s: %w", brand, retryErr)
				}

				// If it's a client error (4xx), we stop this brand but don't report error
				if strings.Contains(retryErr.Error(), "4") {
					s.logger.Infof("Stopping brand %s due to client error", brand)
					return nil
				}

				// For other errors, we just continue to the next page
				continue
			}

			// Parse JSON into a slice of ProductRaw.
			products, err := s.parseKaspiProductsFromJSON(pageData)
			if err != nil {
				s.logger.Errorf("Failed to parse JSON for brand %s, page %d: %v", brand, page, err)
				continue
			}

			// If no products found, stop processing this brand
			if len(products) == 0 {
				s.logger.Infof("No more products for brand %s after page %d", brand, page)
				break
			}

			// Send a batch of products to Kafka.
			if err := s.producer.SendBatch(products); err != nil {
				s.logger.Errorf("Failed to send batch to Kafka for brand %s, page %d: %v", brand, page, err)
				continue
			}

			// Accumulate total count.
			atomic.AddInt64(&totalCount, int64(len(products)))

			s.logger.Infof("Successfully processed brand %s, page %d, products: %d", brand, page, len(products))

			// Sleep a bit to avoid rate limiting
			// time.Sleep(100 * time.Millisecond)
		}

		s.logger.Infof("Completed processing brand: %s", brand)
		return nil
	}

	// Start a worker pool.
	err = StartWorkerPool(rootCtx, brandsCh, s.cfg.WorkerCount, workerFn)
	if err != nil {
		s.logger.Errorf("Worker pool error: %v", err)
		return totalCount, err
	}

	s.logger.Infof("Kaspi scraping finished. Total products sent: %d", totalCount)
	return totalCount, nil
}

// extractBrands extracts the list of brand names from Kaspi's category page.
func (s *KaspiScraper) extractBrands() ([]string, error) {
	s.logger.Infof("Extracting available brands for category %s...", s.cfg.Category)
	categoryURL := fmt.Sprintf("https://kaspi.kz/shop/c/%s/", s.cfg.Category)

	req, _ := http.NewRequest("GET", categoryURL, nil)

	req.Header.Add("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8")
	req.Header.Add("Accept-Language", "en-US,en;q=0.6")
	req.Header.Add("Connection", "keep-alive")
	req.Header.Add("Cookie", fmt.Sprintf("ks.tg=93; k_stat=a85552eb-8baf-4354-a8cf-2bc8843b864d; kaspi.storefront.cookie.city=%d", s.cfg.City))
	req.Header.Add("Host", "kaspi.kz")
	// req.Header.Add("Referer", "https://kaspi.kz/shop/c/smartphones/")
	req.Header.Add("sec-ch-ua", `"Brave";v="137", "Chromium";v="137", "Not/A)Brand";v="24"`)
	req.Header.Add("sec-ch-ua-platform", `"Windows"`)
	req.Header.Add("sec-ch-ua-mobile", "?0")
	req.Header.Add("Sec-Fetch-Dest", "document")
	req.Header.Add("Sec-Fetch-Mode", "navigate")
	req.Header.Add("Sec-Fetch-Site", "same-origin")
	req.Header.Add("Sec-Fetch-User", "?1")
	req.Header.Add("Sec-GPC", "1")
	req.Header.Add("Upgrade-Insecure-Requests", "1")
	req.Header.Add("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/137.0.0.0 Safari/537.36")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		s.logger.Errorf("Failed to load category page: %v", err)
		return nil, fmt.Errorf("failed to load category page: %w", err)
	}

	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			s.logger.Errorf("Failed to close response body: %v", err)
		}
	}(res.Body)

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Find the filter JSON in the HTML
	brandFilters, err := s.parseBrandFilters(body)
	if err != nil {
		return nil, err
	}

	// Convert []BrandFilter to []string containing only brand names
	brandNames := make([]string, len(brandFilters))
	for i, brand := range brandFilters {
		brandNames[i] = brand.Name
	}

	s.logger.Infof("Found %d brands in cathegory %s", len(brandNames), s.cfg.Category)
	return brandNames, nil
}

// parseBrandFilters extracts and parses brand filters from HTML content
func (s *KaspiScraper) parseBrandFilters(htmlContent []byte) ([]BrandFilter, error) {
	filterRegex := regexp.MustCompile(brandFilterPattern)
	matches := filterRegex.FindSubmatch(htmlContent)
	if len(matches) < 2 {
		return nil, errors.New("filters data not found in HTML")
	}

	var brandFilters []BrandFilter
	if err := json.Unmarshal(matches[1], &brandFilters); err != nil {
		return nil, fmt.Errorf("failed to unmarshal brands: %w", err)
	}

	if len(brandFilters) == 0 {
		return nil, errors.New("no brands found in filters")
	}

	return brandFilters, nil
}

// buildURL constructs the URL to fetch products from the Kaspi API or page.
func (s *KaspiScraper) buildURL(category string, city int, page int, requestID string, brand string) string {
	base := "https://kaspi.kz/yml/product-view/pl/results"

	// Construct the 'q' parameter with "Magnum_ZONE1" suffix.
	qValue := ":category:" + category + ":availableInZones:Magnum_ZONE1"
	escapedQ := url.QueryEscape(qValue)

	// Add text parameter for brand filtering
	textParam := ""
	if brand != "" {
		textParam = "&text=" + url.QueryEscape(strings.ToLower(brand))
	} else {
		textParam = "&text"
	}

	// Add requestID parameter
	requestIDParam := ""
	if requestID != "" {
		requestIDParam = "&requestId=" + requestID
	} else {
		requestIDParam = "&requestId"
	}

	// Assemble the query string:
	resulUrl := fmt.Sprintf(
		"%s?page=%d&q=%s%s&sort=relevance&qs%s&ui=d&i=-1&c=%d",
		base,
		page,
		escapedQ,
		textParam,
		requestIDParam,
		city,
	)
	s.logger.Debugf("URL built: %s", resulUrl)
	return resulUrl
}

// fetchProductsPage performs an HTTP GET request to fetch the product data.
func (s *KaspiScraper) fetchProductsPage(ctx context.Context, urlStr string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", urlStr, nil)
	if err != nil {
		return nil, err
	}

	// Set custom headers as required by Kaspi.
	req.Header.Add("Accept", "application/json, text/*")
	req.Header.Add("Accept-Encoding", "gzip, deflate, br, zstd")
	req.Header.Add("Accept-Language", "en-US,en;q=0.7")
	req.Header.Add("Connection", "keep-alive")
	req.Header.Add("Cookie", "ks.tg=88; k_stat=a2dceb4c-1780-4e21-bd43-2ec36704520f; kaspi.storefront.cookie.city=750000000")
	req.Header.Add("Host", "kaspi.kz")
	// req.Header.Add("Referer", "https://kaspi.kz/shop/c/smartphones/")
	req.Header.Set("Referer", fmt.Sprintf("https://kaspi.kz/shop/c/%s/", s.cfg.Category))
	req.Header.Add("sec-ch-ua", `"Brave";v="137", "Chromium";v="137", "Not/A)Brand";v="24"`)
	req.Header.Add("sec-ch-ua-mobile", "?0")
	req.Header.Add("sec-ch-ua-platform", `"Windows"`)
	req.Header.Add("Sec-Fetch-Dest", "empty")
	req.Header.Add("Sec-Fetch-Mode", "cors")
	req.Header.Add("Sec-Fetch-Site", "same-origin")
	req.Header.Add("Sec-GPC", "1")
	req.Header.Add("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/137.0.0.0 Safari/537.36")
	// req.Header.Add("X-KS-City", "750000000")
	req.Header.Set("X-KS-City", fmt.Sprintf("%d", s.cfg.City))
	// s.logger.Debugf("Headers set: %v", req.Header)

	// Add compression support to the client
	client := &http.Client{
		Timeout: 15 * time.Second,
		Transport: &http.Transport{
			DisableCompression: false,
		},
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			s.logger.Errorf("Failed to close response body: %v", err)
		}
	}(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("received non-200 status: %d", resp.StatusCode)
	}

	// Handle different compression types
	var reader io.Reader
	switch resp.Header.Get("Content-Encoding") {
	case "gzip":
		gzReader, err := gzip.NewReader(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to create gzip reader: %w", err)
		}
		defer func(gzReader *gzip.Reader) {
			err := gzReader.Close()
			if err != nil {
				s.logger.Errorf("Failed to close gzip reader: %v", err)
			}
		}(gzReader)
		reader = gzReader
	case "br":
		reader = brotli.NewReader(resp.Body)
	case "deflate":
		flateReader := flate.NewReader(resp.Body)
		defer func(flateReader io.ReadCloser) {
			err := flateReader.Close()
			if err != nil {
				s.logger.Errorf("Failed to close flate reader: %v", err)
			}
		}(flateReader)
		reader = flateReader
	default:
		reader = resp.Body
	}

	// Read the decompressed data
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Debug the response
	s.logger.Debugf("Response headers: %v", resp.Header)
	s.logger.Debugf("Response body length: %d bytes", len(data))
	// s.logger.Debugf("First 100 bytes of response: %s", string(data[:min(len(data), 100)]))

	return data, nil
}

// parseKaspiProductsFromJSON deserializes the JSON into []model.ProductRaw.
func (s *KaspiScraper) parseKaspiProductsFromJSON(data []byte) ([]model.ProductRaw, error) {

	// First, try to parse the root response structure
	var response struct {
		Results []struct {
			Title    string `json:"title"`
			Price    int64  `json:"unitPrice"`
			ShopLink string `json:"shopLink"`
		} `json:"data"`
		// Add other fields if needed
		Total int `json:"total"`
	}

	if err := json.Unmarshal(data, &response); err != nil {
		// Log the actual JSON for debugging
		s.logger.Debugf("Failed to parse JSON, content: %s", string(data))
		return nil, fmt.Errorf("json unmarshal error: %w", err)
	}

	// Create a product slice with initial capacity
	products := make([]model.ProductRaw, 0, len(response.Results))

	// Convert each result to ProductRaw
	for _, result := range response.Results {
		s.logger.Debugf("Processing product: %+v", result)
		product := model.ProductRaw{
			Title:       result.Title,
			Price:       result.Price,
			URL:         "https://kaspi.kz/shop" + result.ShopLink,
			Marketplace: "kaspi",
			Category:    s.cfg.Category,
			Timestamp:   time.Now(),
		}
		products = append(products, product)
	}

	s.logger.Infof("Parsed %d products from JSON", len(products))
	return products, nil
}
