package aggregator

import (
    "testing"
)

func TestMainAggregator(t *testing.T) {
    agg := NewMainAggregator()

    // Test crypto price fetching
    symbol := "BTCUSDT"
    price, err := agg.FetchCryptoPrice(symbol)
    if err != nil {
        t.Errorf("Failed to fetch crypto price: %v", err)
    }
    if price <= 0 {
        t.Errorf("Invalid price received: %f", price)
    }

    // Add more tests for other data types as they are implemented
} 