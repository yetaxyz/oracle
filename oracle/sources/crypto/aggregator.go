package crypto

import (
    "bytes"
    "encoding/json"
    "fmt"
    "io/ioutil"
    "log/slog"
    "net/http"
    "os"
    "sort"
    "strconv"
    "strings"
    "sync"
    "time"
    "yetaXYZ/oracle/common"
)

// CryptoAggregator handles cryptocurrency price aggregation
type CryptoAggregator struct {
    config *common.BaseConfig
    client *http.Client
    logger *slog.Logger
}

// NewCryptoAggregator creates a new CryptoAggregator
func NewCryptoAggregator(config *common.BaseConfig) *CryptoAggregator {
    logFilePath := "oracle/sources/crypto/aggregator_errors.log" // Define log file path
    logFile, err := os.OpenFile(logFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)

    var logger *slog.Logger
    if err != nil {
        // Log error about failing to open file and fall back to stdout
        fmt.Fprintf(os.Stderr, "Error opening log file %s: %v. Falling back to stdout.\n", logFilePath, err)
        logger = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
            AddSource: true,
        }))
    } else {
        // Use the file for logging
        // Note: The file handle logFile should ideally be closed when the application exits.
        // This might require adding a Close method to CryptoAggregator if it's managed.
        logger = slog.New(slog.NewTextHandler(logFile, &slog.HandlerOptions{
            AddSource: true,
        }))
        logger.Info("Logging configured to file", "path", logFilePath) // Log confirmation
    }

    return &CryptoAggregator{
        config: config,
        client: &http.Client{
            Timeout: 10 * time.Second,
        },
        logger: logger, // Assign the configured logger
    }
}

// FetchPrice fetches the price for a given trading pair concurrently from all configured sources.
func (a *CryptoAggregator) FetchPrice(symbol string) (*common.PricePoint, error) {
    // Get pair configuration
    pairConfig, err := GetPairConfig(symbol)
    if err != nil {
        return nil, fmt.Errorf("failed to get pair config for symbol %s: %w", symbol, err)
    }

    // Initialize WaitGroup and channel for collecting results
    var wg sync.WaitGroup
    // Estimate buffer size based on potential sources
    numSources := len(pairConfig.Sources.CEX.Exchanges) + len(pairConfig.Sources.DEX.Sources)
    resultsChan := make(chan *common.PricePoint, numSources)

    // Fetch from enabled CEX sources concurrently
    if pairConfig.Sources.CEX.Enabled {
        for _, exchange := range pairConfig.Sources.CEX.Exchanges {
            wg.Add(1)
            go func(exchangeName string) { // Capture loop variable
                defer wg.Done()
                var price *common.PricePoint
                var fetchErr error

                switch exchangeName {
                case "binance":
                    price, fetchErr = a.fetchBinancePrice(symbol)
                case "coinbase":
                    coinbaseSymbol := pairConfig.BaseCurrency + "-" + pairConfig.QuoteCurrency
                    price, fetchErr = a.fetchCoinbasePrice(coinbaseSymbol)
                case "kraken":
                    krakenSymbol := symbol // Assuming symbol is okay, might need mapping
                    price, fetchErr = a.fetchKrakenPrice(krakenSymbol)
                default:
                    a.logger.Warn("Unsupported CEX exchange configured", "exchange", exchangeName, "symbol", symbol)
                    resultsChan <- nil // Send nil for unsupported types
                    return             // Exit goroutine
                }

                if fetchErr != nil {
                    a.logger.Warn("Error fetching price from CEX",
                        "exchange", exchangeName,
                        "symbol", symbol,
                        "error", fetchErr.Error())
                    resultsChan <- nil // Send nil on error
                    return             // Exit goroutine
                }

                if price != nil {
                    // Assign per-source weight from config
                    if weight, ok := pairConfig.Sources.CEX.Weights[exchangeName]; ok {
                        price.Weight = weight
                        resultsChan <- price // Send valid price point
                    } else {
                        a.logger.Warn("Weight configuration missing for CEX source", "exchange", exchangeName, "symbol", symbol)
                        resultsChan <- nil // Send nil if weight is missing
                    }
                } else {
                    // Should not happen if fetchErr was nil, but handle defensively
                    resultsChan <- nil
                }
            }(exchange) // Pass the exchange name to the goroutine
        }
    }

    // Fetch from enabled DEX sources concurrently
    if pairConfig.Sources.DEX.Enabled {
        for _, source := range pairConfig.Sources.DEX.Sources {
            wg.Add(1)
            go func(sourceDetail common.DEXSource) { // Changed type here
                defer wg.Done()
                var price *common.PricePoint
                var fetchErr error

                switch sourceDetail.Name {
                case "uniswap_v3":
                    price, fetchErr = a.fetchUniswapV3Price(sourceDetail.Endpoint, sourceDetail.PoolAddress)
                default:
                    a.logger.Warn("Unsupported DEX source configured", "source_name", sourceDetail.Name, "symbol", symbol)
                    resultsChan <- nil
                    return
                }

                if fetchErr != nil {
                    a.logger.Warn("Error fetching price from DEX",
                        "source_name", sourceDetail.Name,
                        "symbol", symbol,
                        "error", fetchErr.Error())
                    resultsChan <- nil
                    return
                }

                if price != nil {
                    // Assign per-source weight from config
                    dexSourceName := sourceDetail.Name
                    if weight, ok := pairConfig.Sources.DEX.Weights[dexSourceName]; ok {
                        price.Weight = weight
                        resultsChan <- price
                    } else {
                        a.logger.Warn("Weight configuration missing for DEX source", "source_name", dexSourceName, "symbol", symbol)
                        resultsChan <- nil
                    }
                } else {
                    resultsChan <- nil
                }
            }(source) // Pass the source detail struct to the goroutine
        }
    }

    // Goroutine to close the channel once all fetchers are done
    go func() {
        wg.Wait()
        close(resultsChan)
    }()

    // Collect results from the channel
    prices := make([]*common.PricePoint, 0, numSources) // Pre-allocate slice capacity
    for priceResult := range resultsChan {
        if priceResult != nil { // Only append valid, non-error results with weights
            prices = append(prices, priceResult)
        }
    }

    // Check minimum sources after collecting all results
    if len(prices) < pairConfig.MinimumSources {
        return nil, fmt.Errorf("insufficient valid price sources after concurrent fetch for %s: got %d, need %d", symbol, len(prices), pairConfig.MinimumSources)
    }

    // Calculate aggregate price using the collected prices
    aggregatedPricePoint, err := a.calculateAggregatePrice(prices, pairConfig)
    if err != nil {
        // Log the aggregation error in addition to returning it
        a.logger.Error("Failed to calculate aggregate price", "symbol", symbol, "error", err)
        return nil, fmt.Errorf("aggregation failed for %s: %w", symbol, err)
    }

    return aggregatedPricePoint, nil
}

// fetchBinancePrice fetches price from Binance
func (a *CryptoAggregator) fetchBinancePrice(symbol string) (*common.PricePoint, error) {
    url := fmt.Sprintf("https://api.binance.com/api/v3/ticker/24hr?symbol=%s", symbol)
    resp, err := a.client.Get(url)
    if err != nil {
        return nil, fmt.Errorf("binance request failed: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        bodyBytes, _ := ioutil.ReadAll(resp.Body)
        return nil, fmt.Errorf("binance API error: status %d, body: %s", resp.StatusCode, string(bodyBytes))
    }

    var data struct {
        LastPrice string `json:"lastPrice"`
        Volume    string `json:"volume"`
    }

    body, err := ioutil.ReadAll(resp.Body)
    if err != nil {
        return nil, fmt.Errorf("failed to read binance response body: %w", err)
    }

    if err := json.Unmarshal(body, &data); err != nil {
        return nil, fmt.Errorf("failed to unmarshal binance response: %w, body: %s", err, string(body))
    }

    price, err := parseFloat(data.LastPrice)
    if err != nil {
        return nil, fmt.Errorf("failed to parse binance price '%s': %w", data.LastPrice, err)
    }

    volume, err := parseFloat(data.Volume)
    if err != nil {
        a.logger.Warn("Failed to parse binance volume, setting to 0", "volume_str", data.Volume, "error", err)
        volume = 0
    }

    return &common.PricePoint{
        Source:    "binance",
        Price:     price,
        Volume:    volume,
        Timestamp: time.Now().UTC(),
    }, nil
}

// fetchCoinbasePrice fetches price from Coinbase
func (a *CryptoAggregator) fetchCoinbasePrice(symbol string) (*common.PricePoint, error) {
    url := fmt.Sprintf("https://api.coinbase.com/v2/prices/%s/spot", symbol)
    resp, err := a.client.Get(url)
    if err != nil {
        return nil, fmt.Errorf("coinbase request failed: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        bodyBytes, _ := ioutil.ReadAll(resp.Body)
        return nil, fmt.Errorf("coinbase API error: status %d, body: %s", resp.StatusCode, string(bodyBytes))
    }

    var data struct {
        Data struct {
            Amount string `json:"amount"`
        } `json:"data"`
    }

    if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
        return nil, fmt.Errorf("failed to decode coinbase response: %w", err)
    }

    price, err := parseFloat(data.Data.Amount)
    if err != nil {
        return nil, fmt.Errorf("failed to parse coinbase price '%s': %w", data.Data.Amount, err)
    }

    return &common.PricePoint{
        Source:    "coinbase",
        Price:     price,
        Volume:    0,
        Timestamp: time.Now().UTC(),
    }, nil
}

// fetchKrakenPrice fetches price from Kraken
func (a *CryptoAggregator) fetchKrakenPrice(symbol string) (*common.PricePoint, error) {
    // Map common symbols to Kraken specific pairs if needed
    krakenPair := symbol
    if strings.HasPrefix(symbol, "BTC") {
        krakenPair = strings.Replace(symbol, "BTC", "XBT", 1)
        // Add other mappings if necessary (e.g., DOGE -> XDG)
        a.logger.Info("Mapping symbol for Kraken", "original", symbol, "kraken_pair", krakenPair)
    }
    url := fmt.Sprintf("https://api.kraken.com/0/public/Ticker?pair=%s", krakenPair)
    resp, err := a.client.Get(url)
    if err != nil {
        return nil, fmt.Errorf("kraken request failed: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        bodyBytes, _ := ioutil.ReadAll(resp.Body)
        return nil, fmt.Errorf("kraken API error: status %d, body: %s", resp.StatusCode, string(bodyBytes))
    }

    var response struct {
        Error  []string                            `json:"error"`
        Result map[string]krakenTickerInfo `json:"result"`
    }

    if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
        return nil, fmt.Errorf("failed to decode kraken response: %w", err)
    }

    if len(response.Error) > 0 {
        return nil, fmt.Errorf("kraken API returned errors: %v", response.Error)
    }

    tickerInfo, ok := response.Result[krakenPair]
    if !ok {
        availablePairs := make([]string, 0, len(response.Result))
        for k := range response.Result {
            availablePairs = append(availablePairs, k)
        }
        return nil, fmt.Errorf("kraken response did not contain data for pair %s. Available pairs: %v", krakenPair, availablePairs)
    }

    if len(tickerInfo.LastTrade) < 1 || len(tickerInfo.Volume) < 2 {
        return nil, fmt.Errorf("invalid ticker data structure from Kraken for pair %s: %+v", krakenPair, tickerInfo)
    }

    price, err := parseFloat(tickerInfo.LastTrade[0])
    if err != nil {
        return nil, fmt.Errorf("failed to parse kraken price '%s': %w", tickerInfo.LastTrade[0], err)
    }

    volume, err := parseFloat(tickerInfo.Volume[1])
    if err != nil {
        a.logger.Warn("Failed to parse kraken volume, setting to 0", "volume_str", tickerInfo.Volume[1], "error", err)
        volume = 0
    }

    return &common.PricePoint{
        Source:    "kraken",
        Price:     price,
        Volume:    volume,
        Timestamp: time.Now().UTC(),
    }, nil
}

// Helper struct for Kraken ticker response
type krakenTickerInfo struct {
    Ask       []string `json:"a"`
    Bid       []string `json:"b"`
    LastTrade []string `json:"c"`
    Volume    []string `json:"v"`
    VWAP      []string `json:"p"`
    Trades    []int    `json:"t"`
    Low       []string `json:"l"`
    High      []string `json:"h"`
    Open      string   `json:"o"`
}

// fetchUniswapV3Price fetches price from Uniswap V3
func (a *CryptoAggregator) fetchUniswapV3Price(endpoint, poolAddress string) (*common.PricePoint, error) {
    a.logger.Info("Fetching price from Uniswap V3 subgraph", "endpoint", endpoint, "pool", poolAddress)

    query := fmt.Sprintf(`{
        pool(id: "%s") {
            token0Price
            token1Price
            volumeUSD
            token0 { id symbol decimals }
            token1 { id symbol decimals }
        }
    }`, poolAddress)

    requestBody := map[string]string{
        "query": query,
    }
    requestBodyBytes, err := json.Marshal(requestBody)
    if err != nil {
        return nil, fmt.Errorf("failed to marshal uniswap graphql request body: %w", err)
    }

    req, err := http.NewRequest("POST", endpoint, bytes.NewBuffer(requestBodyBytes))
    if err != nil {
        return nil, fmt.Errorf("failed to create uniswap request: %w", err)
    }
    req.Header.Set("Content-Type", "application/json")

    resp, err := a.client.Do(req)
    if err != nil {
        return nil, fmt.Errorf("uniswap subgraph request failed: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        bodyBytes, _ := ioutil.ReadAll(resp.Body)
        return nil, fmt.Errorf("uniswap subgraph API error: status %d, body: %s", resp.StatusCode, string(bodyBytes))
    }

    var response struct {
        Data struct {
            Pool *struct {
                Token0Price string `json:"token0Price"`
                Token1Price string `json:"token1Price"`
                VolumeUSD   string `json:"volumeUSD"`
                Token0      struct {
                    ID       string `json:"id"`
                    Symbol   string `json:"symbol"`
                    Decimals string `json:"decimals"`
                } `json:"token0"`
                Token1      struct {
                    ID       string `json:"id"`
                    Symbol   string `json:"symbol"`
                    Decimals string `json:"decimals"`
                } `json:"token1"`
            } `json:"pool"`
        } `json:"data"`
        Errors []interface{} `json:"errors"`
    }

    body, err := ioutil.ReadAll(resp.Body)
    if err != nil {
        return nil, fmt.Errorf("failed to read uniswap response body: %w", err)
    }

    if err := json.Unmarshal(body, &response); err != nil {
        return nil, fmt.Errorf("failed to unmarshal uniswap response: %w, body: %s", err, string(body))
    }

    if len(response.Errors) > 0 {
        return nil, fmt.Errorf("uniswap subgraph returned errors: %v", response.Errors)
    }

    if response.Data.Pool == nil {
        return nil, fmt.Errorf("pool %s not found in uniswap subgraph response", poolAddress)
    }

    poolData := response.Data.Pool

    priceStr := poolData.Token1Price

    price, err := parseFloat(priceStr)
    if err != nil {
        return nil, fmt.Errorf("failed to parse uniswap price '%s': %w", priceStr, err)
    }

    volume, err := parseFloat(poolData.VolumeUSD)
    if err != nil {
        a.logger.Warn("Failed to parse uniswap volumeUSD, setting to 0", "volume_str", poolData.VolumeUSD, "error", err)
        volume = 0
    }

    return &common.PricePoint{
        Source:    fmt.Sprintf("uniswap_v3:%s", poolAddress),
        Price:     price,
        Volume:    volume,
        Timestamp: time.Now().UTC(),
    }, nil
}

// calculateAggregatePrice performs robust price aggregation:
// 1. Filters stale prices.
// 2. Performs outlier rejection using IQR.
// 3. Calculates the weighted median price.
// 4. Calculates aggregate volume.
func (a *CryptoAggregator) calculateAggregatePrice(prices []*common.PricePoint, config *common.PairConfig) (*common.PricePoint, error) {
    if len(prices) == 0 {
        return nil, fmt.Errorf("no prices provided for aggregation")
    }

    // 1. Filter stale prices
    now := time.Now().UTC()
    // Use MaxPriceAgeSeconds from config and convert to Duration
    maxAge := time.Duration(config.MaxPriceAgeSeconds) * time.Second
    validPrices := make([]*common.PricePoint, 0, len(prices))
    for _, p := range prices {
        if now.Sub(p.Timestamp) <= maxAge {
            validPrices = append(validPrices, p)
        } else {
            a.logger.Warn("Discarding stale price", "source", p.Source, "timestamp", p.Timestamp, "max_age", maxAge)
        }
    }

    if len(validPrices) < config.MinimumSources {
        return nil, fmt.Errorf("insufficient non-stale price sources: got %d, need %d", len(validPrices), config.MinimumSources)
    }

    // 2. Outlier Rejection using IQR
    sort.Slice(validPrices, func(i, j int) bool { // Sort by price for IQR calc
        return validPrices[i].Price < validPrices[j].Price
    })

    q1Index := len(validPrices) / 4
    q3Index := (len(validPrices) * 3) / 4

    // Handle cases with few data points where Q1/Q3 might be less meaningful
    // We need at least 4 points for distinct Q1 and Q3 indices.
    // If fewer than 4, skip IQR filtering for now? Or use a different method?
    // For simplicity, let's proceed, but be aware this might not be robust for < 4 prices.
    if len(validPrices) < 4 {
        a.logger.Info("Skipping IQR outlier detection, too few data points", "count", len(validPrices))
    } else {
        q1 := validPrices[q1Index].Price
        q3 := validPrices[q3Index].Price
        iqr := q3 - q1
        // Use IQRMultiplier from config
        k := config.IQRMultiplier // This usage should now be correct
        lowerBound := q1 - k*iqr
        upperBound := q3 + k*iqr

        filteredPrices := make([]*common.PricePoint, 0, len(validPrices))
        for _, p := range validPrices {
            if p.Price >= lowerBound && p.Price <= upperBound {
                filteredPrices = append(filteredPrices, p)
            } else {
                a.logger.Warn("Discarding outlier price (IQR)",
                    "source", p.Source,
                    "price", p.Price,
                    "q1", q1, "q3", q3, "iqr", iqr,
                    "lower_bound", lowerBound, "upper_bound", upperBound)
            }
        }
        validPrices = filteredPrices // Update validPrices to the filtered list
    }

    // 3. Check minimum sources *after* outlier rejection
    if len(validPrices) < config.MinimumSources {
        return nil, fmt.Errorf("insufficient non-outlier price sources: got %d, need %d", len(validPrices), config.MinimumSources)
    }

    // 4. Calculate Weighted Median (Volume-Enhanced)
    sort.Slice(validPrices, func(i, j int) bool { // Ensure sorted by price
        return validPrices[i].Price < validPrices[j].Price
    })

    // Calculate total static weight and total volume for dynamic weighting
    totalStaticWeight := 0.0
    totalVolume := 0.0
    for _, p := range validPrices {
        totalStaticWeight += p.Weight
        if p.Volume > 0 { // Only include positive volume
            totalVolume += p.Volume
        }
    }

    if totalStaticWeight <= 0 {
        // Fallback logic remains the same if base weights are invalid
        a.logger.Error("Total static weight of valid sources is zero or negative", "total_weight", totalStaticWeight)
        // Fallback: simple median of the current `validPrices`
        if len(validPrices) > 0 {
            medianIdx := len(validPrices) / 2
            if len(validPrices)%2 == 0 && medianIdx > 0 {
                medianIdx-- // Use lower-middle for even count
            }
            a.logger.Warn("Falling back to simple median due to zero total static weight")
            // Need to calculate aggregate volume even for fallback
            aggregateVolumeFallback := 0.0
            for _, p := range validPrices {
                aggregateVolumeFallback += p.Volume
            }
            return &common.PricePoint{
                Source:    "aggregated_fallback_median_dyn",
                Price:     validPrices[medianIdx].Price,
                Volume:    aggregateVolumeFallback,
                Timestamp: validPrices[medianIdx].Timestamp,
            }, nil
        } else {
            return nil, fmt.Errorf("cannot calculate median, no valid prices after filtering for dynamic fallback")
        }
    }

    // Calculate dynamic weights and total dynamic weight
    totalDynamicWeight := 0.0
    dynamicWeights := make(map[*common.PricePoint]float64, len(validPrices))
    for _, p := range validPrices {
        dynamicWeight := p.Weight // Start with static weight
        if totalVolume > 0 && p.Volume > 0 { // Add volume boost if available
            volumeShare := p.Volume / totalVolume
            // Simple boost: increase weight by a factor proportional to volume share
            // Example: weight * (1 + volumeShare). Adjust formula as needed.
            dynamicWeight *= (1 + volumeShare)
        }
        dynamicWeights[p] = dynamicWeight
        totalDynamicWeight += dynamicWeight
    }

    if totalDynamicWeight <= 0 {
        // If dynamic weights sum to zero (e.g., all volumes were zero and static weights were zero),
        // fall back similar to the static weight check
        a.logger.Error("Total dynamic weight is zero or negative, falling back", "total_dynamic_weight", totalDynamicWeight)
        // Fallback logic (simple median of validPrices) as above...
        if len(validPrices) > 0 {
            medianIdx := len(validPrices) / 2
            if len(validPrices)%2 == 0 && medianIdx > 0 {
                medianIdx--
            }
            a.logger.Warn("Falling back to simple median due to zero total dynamic weight")
            aggregateVolumeFallback := 0.0
            for _, p := range validPrices {
                aggregateVolumeFallback += p.Volume
            }
            return &common.PricePoint{
                Source:    "aggregated_fallback_median_dyn",
                Price:     validPrices[medianIdx].Price,
                Volume:    aggregateVolumeFallback,
                Timestamp: validPrices[medianIdx].Timestamp,
            }, nil
        } else {
            return nil, fmt.Errorf("cannot calculate median, no valid prices after filtering for dynamic fallback")
        }
    }

    // Find the weighted median using dynamic weights
    cumulativeDynamicWeight := 0.0
    var weightedMedianPricePoint *common.PricePoint
    for _, p := range validPrices { // Iterate through prices sorted by price
        dynamicWeight := dynamicWeights[p]
        cumulativeDynamicWeight += dynamicWeight
        if cumulativeDynamicWeight >= totalDynamicWeight*0.5 {
            weightedMedianPricePoint = p
            break
        }
    }

    if weightedMedianPricePoint == nil {
        // Defensive check
        return nil, fmt.Errorf("failed to determine volume-enhanced weighted median price point")
    }

    // 5. Aggregate Volume (sum of volumes from the final valid set - calculated earlier)
    aggregateVolume := totalVolume // We already calculated this

    // 6. Return aggregated result
    return &common.PricePoint{
        Source:    "aggregated_vol_weighted_median", // Indicate aggregation method
        Price:     weightedMedianPricePoint.Price,
        Volume:    aggregateVolume,
        Timestamp: weightedMedianPricePoint.Timestamp,
    }, nil
}

// parseFloat safely parses a string into a float64
func parseFloat(s string) (float64, error) {
    val, err := strconv.ParseFloat(s, 64)
    if err != nil {
        return 0, fmt.Errorf("invalid number format '%s': %w", s, err)
    }
    return val, nil
}

// The actual GetPairConfig function is expected to be in the config.go file within this package.