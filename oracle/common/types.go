package common

import "time"

// --- Core Configuration Structures ---

// Chain defines properties of a supported blockchain network.
type Chain struct {
	ID                string   `json:"id"`                // Unique identifier (e.g., "1", "137", "solana-mainnet")
	Name              string   `json:"name"`              // Human-readable name (e.g., "Ethereum", "Polygon")
	NativeCurrency    string   `json:"nativeCurrency"`    // Symbol of the native gas token (e.g., "ETH", "MATIC")
	RPCEndpoints      []string `json:"rpcEndpoints"`      // List of RPC endpoints
	BlockExplorerURLs []string `json:"blockExplorerUrls,omitempty"`
	// Add other chain-specific details if needed (e.g., chain type, parent chain)
}

// Asset defines properties of a crypto asset.
type Asset struct {
	Symbol   string `json:"symbol"` // Unique symbol (e.g., "ETH", "USDC")
	Name     string `json:"name"`   // Human-readable name (e.g., "Ethereum", "USD Coin")
	Decimals int    `json:"decimals"` // Default decimal precision
	// Could add map[ChainID]Address if needed globally, but often handled per-pair/source
}

// SourceType defines the category of a price source.
type SourceType string

const (
	SourceTypeCEX      SourceType = "cex"
	SourceTypeDEXSubgraph SourceType = "dex_subgraph"
	SourceTypeDEXRPC     SourceType = "dex_rpc" // For direct RPC interaction (e.g., Solana)
	// Add other types like Oracle Aggregators (Chainlink, Pyth) if needed
)

// Source defines a single potential source of price data.
type Source struct {
	ID          string     `json:"-"` // Unique ID assigned during loading (e.g., "binance_cex", "uniswap_v3_eth")
	Name        string     `json:"name"`        // Human-readable name
	Type        SourceType `json:"type"`        // Type of source (cex, dex_subgraph, dex_rpc)
	ChainID     string     `json:"chainId,omitempty"` // Required for DEX types, references Chain.ID
	BaseURL     string     `json:"baseUrl,omitempty"`   // Base URL for CEX REST APIs
	SubgraphID  string     `json:"subgraphId,omitempty"` // Subgraph ID for The Graph sources
	// Endpoint    string `json:"endpoint,omitempty"`  // Removed - construct URL as needed
	QueryMethod string     `json:"queryMethod,omitempty"` // Specific query method for subgraphs (e.g., "poolPrice", "bundleEthPrice")
	PoolAddress string     `json:"poolAddress,omitempty"` // Added back: Pool address for pool-based DEX sources
	// Add fields for DEX RPC if needed (e.g., ProgramID, MarketID)
	// Add fields for API Keys if managed centrally here
}

// AggregationParams defines parameters for the aggregation logic for a specific pair.
type AggregationParams struct {
	MinimumSources     int     `json:"minimumSources"`     // Min required valid sources
	MaxPriceAgeSeconds int     `json:"maxPriceAgeSeconds"` // Max age for price data
	IQRMultiplier      float64 `json:"iqrMultiplier"`      // IQR multiplier for outlier rejection
	// Strategy string `json:"strategy,omitempty"` // Could add strategy field later (e.g., "weightedMedianVolBoost")
}

// PairConfig defines the configuration for a specific price feed (pair + context).
type PairConfig struct {
	ID            string            `json:"-"` // Unique ID assigned during loading (e.g., "ETH-USDC-Ethereum")
	BaseAsset     string            `json:"baseAsset"`     // Symbol referencing Asset.Symbol
	QuoteAsset    string            `json:"quoteAsset"`    // Symbol referencing Asset.Symbol
	ChainID       string            `json:"chainId,omitempty"` // References Chain.ID, empty/"global" for global feed
	Aggregation   AggregationParams `json:"aggregation"`   // Aggregation parameters
	SourceIDs     []string          `json:"sources"`        // List of Source IDs to use
	SourceWeights map[string]float64 `json:"weights"`        // Map of Source ID -> weight
}

// --- End Core Configuration Structures ---


// --- Runtime Structures ---

// PricePoint represents a price data point from any source, including status during aggregation.
type PricePoint struct {
	Source    string    `json:"source,omitempty"` // Source ID (e.g., "binance_cex", "uniswap_v3_eth:poolAddr")
	Price     float64   `json:"price"`
	Volume    float64   `json:"volume"`
	Timestamp time.Time `json:"timestamp"`
	Weight    float64   `json:"-"`                // Weight assigned by config (internal use)
	Status    string    `json:"status,omitempty"` // Aggregation status ("valid", "stale", "outlier", "pending")
}

// LoadedConfig holds all parsed configuration maps.
type LoadedConfig struct {
	Chains  map[string]Chain           // Map Chain.ID -> Chain
	Assets  map[string]Asset           // Map Asset.Symbol -> Asset
	Sources map[string]Source          // Map Source.ID -> Source
	Pairs   map[string]PairConfig      // Map PairConfig.ID -> PairConfig
}

// ResolvedPairConfig contains all necessary information derived from base configs to fetch/aggregate a pair.
type ResolvedPairConfig struct {
	PairID             string
	BaseAsset          *Asset
	QuoteAsset         *Asset
	Chain              *Chain // Optional, nil for global feeds
	AggregationParams  AggregationParams
	ResolvedSources    []ResolvedSource // List of sources with full details
	SourceWeights      map[string]float64 // Map Source.ID -> weight
}

// ResolvedSource combines the Pair's intention with the Source's details.
type ResolvedSource struct {
	SourceID string // The unique ID (e.g., "binance_cex")
	Details  *Source // Pointer to the full Source definition
	// Could add pair-specific overrides here if needed later
}

// --- End Runtime Structures ---


// Note: Removed old BaseConfig, ExchangeConfig, PairConfig, CEX/DEXSourceConfig etc.
// Ensure all usages are updated.