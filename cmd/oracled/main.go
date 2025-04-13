package main

import (
    "flag"
    "fmt"
    "log"
    "yetaXYZ/oracle"
    "time"
)

func main() {
    symbol := flag.String("symbol", "BTCUSDT", "Trading pair symbol")
    interval := flag.Duration("interval", 5*time.Second, "Update interval")
    flag.Parse()

    agg := oracle.NewAggregator()
    agg.SetupDefaultSources()

    for {
        err := agg.FetchPrices(*symbol)
        if err != nil {
            log.Printf("Error fetching prices: %v", err)
            continue
        }

        price, err := agg.GetMedianPrice(*symbol)
        if err != nil {
            log.Printf("Error calculating median: %v", err)
            continue
        }

        fmt.Printf("Current %s price: $%.2f\n", *symbol, price)
        time.Sleep(*interval)
    }
} 