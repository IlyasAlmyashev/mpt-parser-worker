# README.md

# Parser Worker

## Overview
Parser Worker is a Go microservice designed to scrape data from various marketplaces (Kaspi, Ozon, Wildberries), serialize it into a `ProductRaw` structure, and send it to a Kafka topic named `products.raw`.

## Project Structure
```
mpt-parser-worker
├── cmd
│   └── main.go          # Entry point of the application
├── internal
│   ├── config
│   │   └── config.go    # Configuration loading and management
│   ├── kafka
│   │   └── producer.go   # Kafka producer setup
│   ├── logger
│   │   └── logger.go     # Logging functionality
│   ├── model
│   │   └── product.go     # Product data structure
│   └── scrapers
│       ├── interfaces.go  # Scraper interface definition
│       └── kaspi.go       # Kaspi scraper implementation
├── .env                   # Environment variables
├── .gitignore             # Git ignore file
├── go.mod                 # Go module configuration
└── README.md              # Project documentation
```

## Setup Instructions
1. Clone the repository:
   ```
   git clone <repository-url>
   cd mpt-parser-worker
   ```

2. Create a `.env` file in the root directory with the necessary environment variables for Kafka and other configurations.

3. Install the required dependencies:
   ```
   go mod tidy
   ```

## Usage
To run the application, use the following command:
```
go run cmd/main.go --once --only=kaspi
```

You can use these commands from the terminal:
```bash
# Install dependencies
make deps

# Build the application
make build

# Run the application
make run

# Run with Kaspi only
make run-once-kaspi

# Run tests
make test

# Show all available commands
make help
```

## Features
- Scrapes product data from specified marketplaces.
- Sends serialized product data to a Kafka topic.
- Configurable via environment variables.
- Supports manual and scheduled scraping.

## License
This project is licensed under the MIT License.