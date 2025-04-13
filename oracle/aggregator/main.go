package aggregator

import (
    "time"
    
    "yetaXYZ/oracle/common"
    "yetaXYZ/oracle/sources/crypto"
)

// MainAggregator coordinates all data source aggregators
type MainAggregator struct {
    CryptoAggregator *crypto.CryptoAggregator
    // Add other aggregators as they are implemented:
    // StockAggregator      *stocks.Aggregator
    // WeatherAggregator    *weather.Aggregator
    // DeFiAggregator       *defi.Aggregator
    // SportsAggregator     *sports.Aggregator
    // NFTAggregator        *nft.Aggregator
    // ForexAggregator      *forex.Aggregator
    // CommodityAggregator  *commodities.Aggregator
    
    config *common.BaseConfig
}

// NewMainAggregator creates a new main aggregator
func NewMainAggregator(config *common.BaseConfig) *MainAggregator {
    return &MainAggregator{
        CryptoAggregator: crypto.NewCryptoAggregator(config),
        config:          config,
    }
}

// FetchCryptoPrice fetches crypto price for a given symbol
func (ma *MainAggregator) FetchCryptoPrice(symbol string) (*common.PricePoint, error) {
    return ma.CryptoAggregator.FetchPrice(symbol)
}

// Future methods for other data types will be added here as they are implemented 