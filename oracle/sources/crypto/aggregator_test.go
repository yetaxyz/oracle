package crypto

import (
    "encoding/json"
    "fmt"
    "net/http"
    "net/http/httptest"
    "os"
    "path/filepath"
    "testing"
    "time"

    "yetaXYZ/oracle/common"
)

func setupTestConfig(t *testing.T) (string, func()) {
    // Create a temporary directory for test configs
    tmpDir, err := os.MkdirTemp("", "crypto-test")
    if err != nil {
        t.Fatalf("Failed to create temp dir: %v", err)
    }

    // Create crypto directory
    cryptoDir := filepath.Join(tmpDir, "crypto")
    if err := os.MkdirAll(cryptoDir, 0755); err != nil {
        t.Fatalf("Failed to create crypto dir: %v", err)
    }

    // Create test configuration files
    configs := map[string]common.ChainConfig{
        "eth.json": {
            Chain: &common.Chain{
                ID:             "1",
                Name:           "Ethereum",
                NativeCurrency: "ETH",
                RPCUrls:        []string{"https://eth.llamarpc.com"},
            },
            Assets: []common.Asset{
                {
                    Symbol:   "ETH",
                    Name:     "Ethereum",
                    Type:     "native",
                    Decimals: 18,
                    Pairs: []common.Pair{
                        {
                            Symbol:                "ETHUSDT",
                            BaseCurrency:          "ETH",
                            QuoteCurrency:         "USDT",
                            Exchanges:             []string{"Binance", "Coinbase"},
                            MinimumSources:        2,
                            UpdateFrequencySeconds: 5,
                            Sources: common.Sources{
                                CEX: common.CEXSource{
                                    Enabled: true,
                                    Weight:  1.0,
                                },
                            },
                        },
                    },
                },
            },
            Exchanges: []common.Exchange{
                {
                    Name:        "Binance",
                    BaseURL:     "https://api.binance.com/api/v3",
                    RequiresKey: true,
                },
                {
                    Name:        "Coinbase",
                    BaseURL:     "https://api.coinbase.com/v2",
                    RequiresKey: false,
                },
            },
        },
        "no-chain.json": {
            Assets: []common.Asset{
                {
                    Symbol:   "BTC",
                    Name:     "Bitcoin",
                    Type:     "native",
                    Decimals: 8,
                    Pairs: []common.Pair{
                        {
                            Symbol:                "BTCUSDT",
                            BaseCurrency:          "BTC",
                            QuoteCurrency:         "USDT",
                            Exchanges:             []string{"Binance", "Coinbase"},
                            MinimumSources:        2,
                            UpdateFrequencySeconds: 5,
                            Sources: common.Sources{
                                CEX: common.CEXSource{
                                    Enabled: true,
                                    Weight:  1.0,
                                },
                            },
                        },
                    },
                },
            },
            Exchanges: []common.Exchange{
                {
                    Name:        "Binance",
                    BaseURL:     "https://api.binance.com/api/v3",
                    RequiresKey: true,
                },
                {
                    Name:        "Coinbase",
                    BaseURL:     "https://api.coinbase.com/v2",
                    RequiresKey: false,
                },
            },
        },
    }

    // Write test configs
    for filename, config := range configs {
        data, err := json.MarshalIndent(config, "", "    ")
        if err != nil {
            t.Fatalf("Failed to marshal config: %v", err)
        }
        if err := os.WriteFile(filepath.Join(cryptoDir, filename), data, 0644); err != nil {
            t.Fatalf("Failed to write config file: %v", err)
        }
    }

    // Return cleanup function
    cleanup := func() {
        os.RemoveAll(tmpDir)
    }

    return tmpDir, cleanup
}

func setupMockExchanges(t *testing.T) (*httptest.Server, *httptest.Server) {
    // Mock Binance server
    binanceServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        symbol := r.URL.Query().Get("symbol")
        var resp string
        switch symbol {
        case "BTCUSDT":
            resp = `{"symbol":"BTCUSDT","price":"50000.00"}`
        case "ETHUSDT":
            resp = `{"symbol":"ETHUSDT","price":"3000.00"}`
        default:
            w.WriteHeader(http.StatusNotFound)
            return
        }
        w.Header().Set("Content-Type", "application/json")
        fmt.Fprintln(w, resp)
    }))

    // Mock Coinbase server
    coinbaseServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        var resp string
        switch r.URL.Path {
        case "/prices/BTC-USD/spot":
            resp = `{"data":{"base":"BTC","currency":"USD","amount":"51000.00"}}`
        case "/prices/ETH-USD/spot":
            resp = `{"data":{"base":"ETH","currency":"USD","amount":"3100.00"}}`
        default:
            w.WriteHeader(http.StatusNotFound)
            return
        }
        w.Header().Set("Content-Type", "application/json")
        fmt.Fprintln(w, resp)
    }))

    return binanceServer, coinbaseServer
}

func TestConfigLoading(t *testing.T) {
    configDir, cleanup := setupTestConfig(t)
    defer cleanup()

    // Test config loading
    if err := LoadConfig(configDir); err != nil {
        t.Fatalf("Failed to load test configs: %v", err)
    }

    // Verify chain configs
    if len(ChainConfigs) != 2 {
        t.Errorf("Expected 2 chain configs, got %d", len(ChainConfigs))
    }

    // Verify ETH config
    ethConfig := ChainConfigs["1"]
    if ethConfig == nil {
        t.Fatal("ETH config not found")
    }
    if ethConfig.Chain.Name != "Ethereum" {
        t.Errorf("Expected chain name 'Ethereum', got '%s'", ethConfig.Chain.Name)
    }

    // Verify BTC config
    btcConfig := ChainConfigs["no-chain"]
    if btcConfig == nil {
        t.Fatal("BTC config not found")
    }
    if len(btcConfig.Assets) != 1 || btcConfig.Assets[0].Symbol != "BTC" {
        t.Error("Invalid BTC config")
    }
}

func TestPriceFetching(t *testing.T) {
    configDir, cleanup := setupTestConfig(t)
    defer cleanup()

    // Setup mock exchange servers
    binanceServer, coinbaseServer := setupMockExchanges(t)
    defer binanceServer.Close()
    defer coinbaseServer.Close()

    // Load configs
    if err := LoadConfig(configDir); err != nil {
        t.Fatalf("Failed to load test configs: %v", err)
    }

    // Update exchange URLs to point to mock servers
    for i, exchange := range SupportedExchanges {
        switch exchange.Name {
        case "Binance":
            SupportedExchanges[i].BaseURL = binanceServer.URL
        case "Coinbase":
            SupportedExchanges[i].BaseURL = coinbaseServer.URL
        }
    }

    // Create aggregator
    config := &common.SourceConfig{
        MinimumSources:  1,
        UpdateFrequency: time.Second,
        RetryAttempts:   1,
        Timeout:         time.Second * 5,
    }
    agg := NewCryptoAggregator(config)

    // Test BTC price fetching
    t.Run("BTC Price", func(t *testing.T) {
        if err := agg.FetchPrices("BTCUSDT"); err != nil {
            t.Fatalf("Failed to fetch BTC price: %v", err)
        }

        price, err := agg.GetMedianPrice("BTCUSDT")
        if err != nil {
            t.Fatalf("Failed to get BTC median price: %v", err)
        }

        if price <= 0 {
            t.Errorf("Invalid BTC price: %f", price)
        }
    })

    // Test ETH price fetching
    t.Run("ETH Price", func(t *testing.T) {
        if err := agg.FetchPrices("ETHUSDT"); err != nil {
            t.Fatalf("Failed to fetch ETH price: %v", err)
        }

        price, err := agg.GetMedianPrice("ETHUSDT")
        if err != nil {
            t.Fatalf("Failed to get ETH median price: %v", err)
        }

        if price <= 0 {
            t.Errorf("Invalid ETH price: %f", price)
        }
    })

    // Test invalid symbol
    t.Run("Invalid Symbol", func(t *testing.T) {
        err := agg.FetchPrices("INVALIDUSDT")
        if err == nil {
            t.Error("Expected error for invalid symbol, got nil")
        }
    })
}

func TestConfigHelpers(t *testing.T) {
    configDir, cleanup := setupTestConfig(t)
    defer cleanup()

    if err := LoadConfig(configDir); err != nil {
        t.Fatalf("Failed to load test configs: %v", err)
    }

    // Test GetChainConfig
    t.Run("GetChainConfig", func(t *testing.T) {
        config := GetChainConfig("1")
        if config == nil {
            t.Error("Expected ETH chain config, got nil")
        }
        if config != nil && config.Chain.Name != "Ethereum" {
            t.Errorf("Expected chain name 'Ethereum', got '%s'", config.Chain.Name)
        }
    })

    // Test GetAssetConfig
    t.Run("GetAssetConfig", func(t *testing.T) {
        asset, chainID := GetAssetConfig("ETH")
        if asset == nil {
            t.Error("Expected ETH asset config, got nil")
        }
        if chainID != "1" {
            t.Errorf("Expected chain ID '1', got '%s'", chainID)
        }
    })

    // Test GetPairConfig
    t.Run("GetPairConfig", func(t *testing.T) {
        pair, asset, chainID := GetPairConfig("ETHUSDT")
        if pair == nil {
            t.Error("Expected ETHUSDT pair config, got nil")
        }
        if asset == nil {
            t.Error("Expected ETH asset config, got nil")
        }
        if chainID != "1" {
            t.Errorf("Expected chain ID '1', got '%s'", chainID)
        }
    })
}

func TestAggregatorBasic(t *testing.T) {
    // Create a basic config
    config := &common.SourceConfig{
        MinimumSources:  1,
        UpdateFrequency: time.Second,
        RetryAttempts:   1,
        Timeout:         time.Second * 5,
    }

    // Create mock servers
    binanceServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "application/json")
        fmt.Fprintln(w, `{"symbol":"BTCUSDT","price":"50000.00"}`)
    }))
    defer binanceServer.Close()

    coinbaseServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "application/json")
        fmt.Fprintln(w, `{"data":{"base":"BTC","currency":"USD","amount":"51000.00"}}`)
    }))
    defer coinbaseServer.Close()

    // Set up supported exchanges
    SupportedExchanges = []common.Exchange{
        {
            Name:        "Binance",
            BaseURL:     binanceServer.URL,
            RequiresKey: false,
        },
        {
            Name:        "Coinbase",
            BaseURL:     coinbaseServer.URL,
            RequiresKey: false,
        },
    }

    // Set up supported pairs
    SupportedPairs = []common.Pair{
        {
            Symbol:         "BTCUSDT",
            BaseCurrency:   "BTC",
            QuoteCurrency:  "USDT",
            Exchanges:      []string{"Binance", "Coinbase"},
            MinimumSources: 1,
            Sources: common.Sources{
                CEX: common.CEXSource{
                    Enabled: true,
                    Weight:  1.0,
                },
            },
        },
    }

    // Create aggregator
    agg := NewCryptoAggregator(config)

    // Test price fetching
    err := agg.FetchPrices("BTCUSDT")
    if err != nil {
        t.Fatalf("Failed to fetch prices: %v", err)
    }

    // Get median price
    price, err := agg.GetMedianPrice("BTCUSDT")
    if err != nil {
        t.Fatalf("Failed to get median price: %v", err)
    }

    // Verify price is in expected range
    if price < 49000 || price > 52000 {
        t.Errorf("Price %f is outside expected range", price)
    }

    t.Logf("Successfully fetched BTC price: $%.2f", price)
} 