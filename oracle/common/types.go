package common

import "time"

// BaseConfig represents the root configuration structure
type BaseConfig struct {
    Exchanges ExchangeConfig `json:"exchanges"`
    Chains    ChainConfig   `json:"chains"`
    Assets    AssetConfig   `json:"assets"`
}

// ExchangeConfig holds both CEX and DEX configurations
type ExchangeConfig struct {
    CEX map[string]CEXDetails `json:"cex"`
    DEX map[string]DEXDetails `json:"dex"`
}

// CEXDetails represents a centralized exchange configuration
type CEXDetails struct {
    Name        string `json:"name"`
    BaseURL     string `json:"baseURL"`
    RequiresKey bool   `json:"requiresKey"`
    RateLimit   int    `json:"rateLimit"`
    Timeout     int    `json:"timeout"`
}

// DEXDetails represents a decentralized exchange configuration
type DEXDetails struct {
    Name         string `json:"name"`
    Type         string `json:"type"`
    Endpoint     string `json:"endpoint"`
    RequiresKey  bool   `json:"requiresKey"`
    MinLiquidity int64  `json:"minLiquidity"`
    Timeout      int    `json:"timeout"`
}

// ChainConfig represents blockchain network configurations
type ChainConfig map[string]Chain

// Chain represents a blockchain network
type Chain struct {
    ID                string   `json:"id"`
    Name              string   `json:"name"`
    NativeCurrency    string   `json:"nativeCurrency"`
    Decimals          int      `json:"decimals"`
    RPCUrls           []string `json:"rpcUrls"`
    BlockExplorerUrls []string `json:"blockExplorerUrls"`
    Type             string   `json:"type"`
    Parent           string   `json:"parent,omitempty"`
    RollupType       string   `json:"rollupType,omitempty"`
}

// AssetConfig represents token configurations across chains
type AssetConfig map[string]Asset

// Asset represents a tradeable asset
type Asset struct {
    Name     string                     `json:"name"`
    Decimals int                        `json:"decimals"`
    Chains   map[string]ChainAssetInfo `json:"chains"`
}

// ChainAssetInfo represents token information on a specific chain
type ChainAssetInfo struct {
    Type    string `json:"type"`    // native, wrapped, token
    Address string `json:"address"`
}

// PairConfig represents trading pair configurations
type PairConfig struct {
    Symbol                string        `json:"symbol"`
    BaseCurrency          string        `json:"baseCurrency"`
    QuoteCurrency         string        `json:"quoteCurrency"`
    MinimumSources        int           `json:"minimumSources"`
    MaxPriceAgeSeconds    int           `json:"maxPriceAgeSeconds"`
    IQRMultiplier         float64       `json:"iqrMultiplier"`
    Sources               SourcesConfig `json:"sources"`
}

// SourcesConfig represents available price sources for a pair
type SourcesConfig struct {
    CEX CEXSourceConfig `json:"cex"`
    DEX DEXSourceConfig `json:"dex,omitempty"`
}

// CEXSourceConfig represents CEX-specific configuration for a pair
type CEXSourceConfig struct {
    Enabled   bool                `json:"enabled"`
    Weights   map[string]float64 `json:"weights"`
    Exchanges []string            `json:"exchanges"`
}

// DEXSourceConfig represents DEX-specific configuration for a pair
type DEXSourceConfig struct {
    Enabled   bool             `json:"enabled"`
    Weights   map[string]float64 `json:"weights"`
    Sources   []DEXSource      `json:"sources"`
}

// DEXSource represents a single DEX source configuration
type DEXSource struct {
    Name        string `json:"name"`
    Type        string `json:"type"`
    Endpoint    string `json:"endpoint"`
    PoolAddress string `json:"poolAddress"`
    ChainID     string `json:"chainId,omitempty"`
}

// PricePoint represents a price data point from any source
type PricePoint struct {
    Source    string    `json:"source,omitempty"`
    Price     float64   `json:"price"`
    Volume    float64   `json:"volume"`
    Timestamp time.Time `json:"timestamp"`
    Weight    float64   `json:"-"`
} 