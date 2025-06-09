package scrapers

type Scraper interface {
	Scrape() (int64, error)
}
