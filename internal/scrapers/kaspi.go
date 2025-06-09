package scrapers

import (
	"context"
	"fmt"
	"time"

	"mpt-parser-worker/internal/kafka"
	"mpt-parser-worker/internal/logger"
	"mpt-parser-worker/internal/model"

	"github.com/chromedp/chromedp"
)

type KaspiScraper struct {
	producer kafka.Producer
	logger   logger.Logger
}

func NewKaspiScraper(producer kafka.Producer, logger logger.Logger) *KaspiScraper {
	return &KaspiScraper{
		producer: producer,
		logger:   logger,
	}
}

// Структура для десериализации ответа Kaspi
type offer struct {
	ProductTitle string `json:"productTitle"`
	Price        int64  `json:"price"`
	ProductURL   string `json:"productUrl"`
}

func (k *KaspiScraper) Scrape() (int64, error) {
	k.logger.Infof("Starting Kaspi scraping with headless Chrome...")

	// Setup Chrome options
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-dev-shm-usage", true),
		// Use Edge if Chrome is not found
		chromedp.ExecPath("C:\\Program Files (x86)\\Microsoft\\Edge\\Application\\msedge.exe"),
	)

	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer allocCancel()

	// Create Chrome instance
	ctx, ctxCancel := chromedp.NewContext(
		allocCtx,
		chromedp.WithLogf(k.logger.Debugf),
	)
	defer ctxCancel()

	var count int64
	page := 1
	const baseURL = "https://kaspi.kz/shop/c/smartphones/?page=%d"

	for {
		url := fmt.Sprintf(baseURL, page)
		k.logger.Infof("Processing page %d: %s", page, url)

		// var htmlContent string
		var productData []offer

		// Add timeout to context for each attempt
		ctx, timeoutCancel := context.WithTimeout(ctx, 30*time.Second)

		err := chromedp.Run(ctx,
			chromedp.Navigate(url),
			chromedp.WaitVisible(".item-card"), // Wait for products to load
			chromedp.Evaluate(`
				Array.from(document.querySelectorAll('.item-card')).map(card => ({
					productTitle: card.querySelector('.item-card__name')?.innerText,
					price: parseInt(card.querySelector('.item-card__prices-price')?.innerText.replace(/[^\d]/g, '')),
					productUrl: card.querySelector('.item-card__name-link')?.getAttribute('href')
				}))
			`, &productData),
		)

		timeoutCancel() // Cancel the timeout context after the run

		if err != nil {
			k.logger.Errorf("Failed to scrape page %d: %v", page, err)
			break
		}

		k.logger.Infof("Found %d products on page %d", len(productData), page)

		if len(productData) == 0 {
			k.logger.Infof("No more products found, stopping pagination")
			break
		}

		// Process products
		for _, item := range productData {
			k.logger.Debugf("Processing product: %+v", item)
			product := model.ProductRaw{
				Title:       item.ProductTitle,
				Price:       item.Price,
				URL:         "https://kaspi.kz" + item.ProductURL,
				Marketplace: "Kaspi",
				Timestamp:   time.Now(),
			}

			// Send product to Kafka immediately
			k.logger.Infof("Sending product to Kafka: %+v", product)
			err := k.producer.Send(product)
			if err != nil {
				k.logger.Errorf("Error sending product to Kafka: %v", err)
				continue
			}

			k.logger.Infof("Successfully sent product to Kafka")
			count++
		}

		page++
	}

	k.logger.Infof("Scraping completed. Found total of %d products", count)
	return count, nil
}
