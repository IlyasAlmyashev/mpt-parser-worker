package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"mpt-parser-worker/internal/config"
	"mpt-parser-worker/internal/kafka"
	"mpt-parser-worker/internal/logger"
	"mpt-parser-worker/internal/scrapers"
)

func main() {

	// Initialize configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatal("Error loading config:", err)
	}

	// Initialize custom logger
	clog, err := logger.New(cfg.LogLevel)
	if err != nil {
		log.Fatal("failed to initialize clog:", err)
	}
	defer func(clog logger.Logger) {
		err := clog.Sync()
		if err != nil {
			log.Fatal("failed to sync custom logger:", err)
		}
	}(clog)

	clog.Infof("Starting application on port %d", cfg.AppPort)

	// Set up Kafka producer
	producer := kafka.NewProducer([]string{cfg.KafkaHost}, cfg.KafkaTopic, clog)
	if producer == nil {
		clog.Errorf("Failed to initialize Kafka producer")
		return
	}
	defer func() {
		if err := producer.Close(); err != nil {
			clog.Errorf("Failed to close Kafka producer: %v", err)
		}
	}()

	// Command-line flags
	once := flag.Bool("once", false, "Run scraping once")
	only := flag.String("only", "", "Specify marketplace to scrape (e.g., kaspi)")
	flag.Parse()

	// Initialize scrapers
	var scraper scrapers.Scraper
	if *only == "kaspi" {
		scraper = scrapers.NewKaspiScraper(scrapers.DefaultKaspiConfig(), producer, clog)
	} else {
		clog.Errorf("Unsupported marketplace: %s", *only)
	}

	// ctx, cancel := context.WithCancel(context.Background())
	// defer cancel()

	// Handle shutdown
	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt, syscall.SIGTERM)
		<-c
		clog.Infof("Shutting down gracefully...")
		// cancel()

		// Force shutdown after timeout
		time.Sleep(10 * time.Second)
		clog.Infof("Forced shutdown after timeout")
		os.Exit(1)
	}()

	// Start the scraping process
	if *once {
		count, err := scraper.Scrape()
		if err != nil {
			clog.Errorf("Error scraping products: %v", err)
			return
		}

		clog.Infof("Scraped %d products", count)
		clog.Infof("One-time scraping completed, shutting down...")
		return
	} else {
		// Set up the cron job for periodic scraping
		// scrapers.RunParsers(ctx, cfg, *only)
		// c := cron.New()
		// c.AddFunc(cfg.SchedulerSpec, run)
		// c.Start()

		// <-ctx.Done()
		// c.Stop()
	}

}
