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
    client *http.Client
    logger *slog.Logger
    sourceDetailCache map[string][]*common.PricePoint // Cache key is now pairID (e.g., "ETHUSDC_Global")
    cacheMutex        sync.RWMutex
}

// NewCryptoAggregator creates a new CryptoAggregator
func NewCryptoAggregator() *CryptoAggregator {
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
        client: &http.Client{
            Timeout: 10 * time.Second,
        },
        logger: logger,
        sourceDetailCache: make(map[string][]*common.PricePoint),
    }
}

// FetchPrice fetches the price for a given resolved pair configuration concurrently.
func (a *CryptoAggregator) FetchPrice(resolvedCfg *common.ResolvedPairConfig) (*common.PricePoint, error) {
    if resolvedCfg == nil {
        return nil, fmt.Errorf("resolved config cannot be nil")
    }
    pairID := resolvedCfg.PairID // Use the resolved Pair ID for logging and caching

    var wg sync.WaitGroup
    numSources := len(resolvedCfg.ResolvedSources)
    resultsChan := make(chan *common.PricePoint, numSources)

    // Fetch from configured sources concurrently
    for _, resolvedSource := range resolvedCfg.ResolvedSources {
        wg.Add(1)
        go func(rs common.ResolvedSource) { // Capture resolved source
            defer wg.Done()
            var price *common.PricePoint
            var fetchErr error

            // Base symbol for CEX APIs (usually needs specific format)
            // This needs refinement - how do CEX fetchers know the exact pair string (e.g., BTCUSDT vs XBTUSDT)?
            // We might need to pass base/quote symbols or generate the API pair string here based on source type.
            // For now, using a simple concatenation - THIS WILL LIKELY BREAK for some CEXs/pairs.
            apiPairSymbol := resolvedCfg.BaseAsset.Symbol + resolvedCfg.QuoteAsset.Symbol

            switch rs.Details.Type {
            case common.SourceTypeCEX:
                switch rs.SourceID { // Route based on specific CEX source ID
                case "binance_cex":
                    price, fetchErr = a.fetchBinancePrice(rs.Details, apiPairSymbol)
                case "coinbase_cex":
                    // Coinbase needs format like BASE-QUOTE
                    coinbaseSymbol := resolvedCfg.BaseAsset.Symbol + "-" + resolvedCfg.QuoteAsset.Symbol
                    price, fetchErr = a.fetchCoinbasePrice(rs.Details, coinbaseSymbol)
                case "kraken_cex":
                    price, fetchErr = a.fetchKrakenPrice(rs.Details, apiPairSymbol)
                default:
                    fetchErr = fmt.Errorf("unsupported CEX source ID: %s", rs.SourceID)
                }
            case common.SourceTypeDEXSubgraph:
                switch rs.Details.QueryMethod {
                case "bundleEthPrice":
                    // Check if the pair is actually ETH/USD(T/C)
                    if resolvedCfg.BaseAsset.Symbol == "ETH" && strings.Contains(resolvedCfg.QuoteAsset.Symbol, "USD") {
                        price, fetchErr = a.fetchTheGraphBundleEthPrice(rs.Details)
                    } else {
                        fetchErr = fmt.Errorf("bundleEthPrice method only valid for ETH/USD pairs, requested for %s", pairID)
                    }
                case "poolPrice":
                    // Need the specific pool address for this pair - where does it come from?
                    // Option 1: Add PoolAddress field to ResolvedSource (populated by GetResolvedPairConfig?)
                    // Option 2: Add PoolAddress map to PairConfig keyed by source ID?
                    // For now, assuming it might be in rs.Details.PoolAddress (requires update to Source struct and JSON)
                    if rs.Details.PoolAddress == "" {
                        fetchErr = fmt.Errorf("pool address missing for poolPrice query method on source %s", rs.SourceID)
                    } else {
                        price, fetchErr = a.fetchTheGraphPoolPrice(rs.Details, rs.Details.PoolAddress) // Pass details and pool
                    }
                default:
                    fetchErr = fmt.Errorf("unsupported dex_subgraph query method '%s' for source %s", rs.Details.QueryMethod, rs.SourceID)
                }
            case common.SourceTypeDEXRPC:
                // Example: Solana fetcher needs specific market ID based on pair/source
                if rs.SourceID == "raydium_solana" {
                    // TODO: Implement fetchRaydiumPrice
                    // Needs: RPC endpoint from Chain config, Market ID (from where? PairConfig extension?)
                    fetchErr = fmt.Errorf("fetcher for %s not implemented", rs.SourceID)
                } else {
                    fetchErr = fmt.Errorf("unsupported dex_rpc source ID: %s", rs.SourceID)
                }
            default:
                fetchErr = fmt.Errorf("unsupported source type '%s' for source %s", rs.Details.Type, rs.SourceID)
            }

            if fetchErr != nil {
                a.logger.Warn("Error fetching price",
                    "pairID", pairID,
                    "sourceID", rs.SourceID,
                    "error", fetchErr.Error())
                resultsChan <- nil
                return
            }

            if price != nil {
                weight, ok := resolvedCfg.SourceWeights[rs.SourceID]
                if !ok {
                    a.logger.Warn("Weight configuration missing", "pairID", pairID, "sourceID", rs.SourceID)
                    resultsChan <- nil
                    return
                }
                price.Weight = weight
                price.Source = rs.SourceID // Set source ID on price point
                resultsChan <- price
            } else {
                // Should not happen if fetchErr is nil, but handle defensively
                a.logger.Warn("Fetcher returned nil price without error", "pairID", pairID, "sourceID", rs.SourceID)
                resultsChan <- nil
            }
        }(resolvedSource)
    }

    go func() {
        wg.Wait()
        close(resultsChan)
    }()

    prices := make([]*common.PricePoint, 0, numSources)
    for priceResult := range resultsChan {
        if priceResult != nil {
            prices = append(prices, priceResult)
        }
    }

    if len(prices) < resolvedCfg.AggregationParams.MinimumSources {
        return nil, fmt.Errorf("insufficient valid price sources after concurrent fetch for %s: got %d, need %d", pairID, len(prices), resolvedCfg.AggregationParams.MinimumSources)
    }

    // Pass aggregation parameters and PairID for caching
    aggregatedPricePoint, err := a.calculateAggregatePrice(prices, &resolvedCfg.AggregationParams, pairID)
    if err != nil {
        a.logger.Error("Failed to calculate aggregate price", "pairID", pairID, "error", err)
        return nil, fmt.Errorf("aggregation failed for %s: %w", pairID, err)
    }

    return aggregatedPricePoint, nil
}

// --- Fetcher Functions --- 
// Signatures updated to accept common.Source details

func (a *CryptoAggregator) fetchBinancePrice(source *common.Source, apiPairSymbol string) (*common.PricePoint, error) {
    url := fmt.Sprintf("%s/ticker/24hr?symbol=%s", source.BaseURL, apiPairSymbol)
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

func (a *CryptoAggregator) fetchCoinbasePrice(source *common.Source, apiPairSymbol string) (*common.PricePoint, error) {
    url := fmt.Sprintf("%s/prices/%s/spot", source.BaseURL, apiPairSymbol)
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

func (a *CryptoAggregator) fetchKrakenPrice(source *common.Source, apiPairSymbol string) (*common.PricePoint, error) {
    // Map common symbols to Kraken specific pairs if needed
    krakenPair := apiPairSymbol
    if strings.HasPrefix(apiPairSymbol, "BTC") {
        krakenPair = strings.Replace(apiPairSymbol, "BTC", "XBT", 1)
        a.logger.Info("Mapping symbol for Kraken", "original", apiPairSymbol, "kraken_pair", krakenPair)
    }
    url := fmt.Sprintf("%s/Ticker?pair=%s", source.BaseURL, krakenPair)
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

// fetchTheGraphPoolPrice fetches price from a subgraph pool.
// Needs poolAddress passed in.
func (a *CryptoAggregator) fetchTheGraphPoolPrice(source *common.Source, poolAddress string) (*common.PricePoint, error) {
    apiKey := os.Getenv("THE_GRAPH_API_KEY")
    if apiKey == "" {
        a.logger.Warn("THE_GRAPH_API_KEY environment variable not set. Subgraph queries might fail or be rate-limited.")
        // Proceed without key for public access attempt
    }
    // Construct endpoint URL
    endpoint := fmt.Sprintf("https://gateway-arbitrum.network.thegraph.com/api/%s/subgraphs/id/%s", apiKey, source.SubgraphID)

    a.logger.Info("Fetching price from The Graph Pool", "source", source.Name, "pool", poolAddress, "subgraphId", source.SubgraphID)

    // Query for the specific pool
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
        return nil, fmt.Errorf("failed to marshal graphql request body for %s: %w", source.Name, err)
    }

    req, err := http.NewRequest("POST", endpoint, bytes.NewBuffer(requestBodyBytes))
    if err != nil {
        return nil, fmt.Errorf("failed to create graphql request for %s: %w", source.Name, err)
    }
    req.Header.Set("Content-Type", "application/json")

    resp, err := a.client.Do(req)
    if err != nil {
        return nil, fmt.Errorf("graphql request failed for %s: %w", source.Name, err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        bodyBytes, _ := ioutil.ReadAll(resp.Body)
        return nil, fmt.Errorf("graphql API error for %s: status %d, body: %s", source.Name, resp.StatusCode, string(bodyBytes))
    }

    // --- Response parsing specific to POOL query --- 
    var response struct {
        Data struct {
            Pool *struct {
                Token0Price string `json:"token0Price"`
                Token1Price string `json:"token1Price"`
                VolumeUSD   string `json:"volumeUSD"`
                // ... token details ...
            } `json:"pool"`
        } `json:"data"`
        Errors []interface{} `json:"errors"`
    }

    body, err := ioutil.ReadAll(resp.Body)
    if err != nil {
        return nil, fmt.Errorf("failed to read graphql response body for %s: %w", source.Name, err)
    }
    if err := json.Unmarshal(body, &response); err != nil {
        return nil, fmt.Errorf("failed to unmarshal graphql response for %s: %w, body: %s", source.Name, err, string(body))
    }
    if len(response.Errors) > 0 {
        return nil, fmt.Errorf("graphql query for %s returned errors: %v", source.Name, response.Errors)
    }
    if response.Data.Pool == nil {
        return nil, fmt.Errorf("pool %s not found in subgraph response for %s", poolAddress, source.Name)
    }

    poolData := response.Data.Pool
    // TODO: Determine price direction (token0 vs token1) based on pair config
    priceStr := poolData.Token1Price // Needs proper logic
    price, err := parseFloat(priceStr)
    if err != nil {
        return nil, fmt.Errorf("failed to parse price for %s pool %s: %w", source.Name, poolAddress, err)
    }
    volume, err := parseFloat(poolData.VolumeUSD)
    if err != nil {
        a.logger.Warn("Failed to parse volumeUSD", "source", source.Name, "pool", poolAddress, "error", err)
        volume = 0
    }

    return &common.PricePoint{
        Source:    fmt.Sprintf("%s:%s", source.Name, poolAddress),
        Price:     price,
        Volume:    volume,
        Timestamp: time.Now().UTC(),
    }, nil
}

// fetchTheGraphBundleEthPrice fetches the global ETH price from The Graph bundle entity.
func (a *CryptoAggregator) fetchTheGraphBundleEthPrice(source *common.Source) (*common.PricePoint, error) {
    apiKey := os.Getenv("THE_GRAPH_API_KEY")
    if apiKey == "" {
        a.logger.Warn("THE_GRAPH_API_KEY environment variable not set. Subgraph queries might fail or be rate-limited.")
    }
    endpoint := fmt.Sprintf("https://gateway-arbitrum.network.thegraph.com/api/%s/subgraphs/id/%s", apiKey, source.SubgraphID)

    a.logger.Info("Fetching ETH price from The Graph Bundle", "source", source.Name, "subgraphId", source.SubgraphID)

    // Fixed query for bundle ETH price
    query := `{
        bundles(first: 1) { id ethPriceUSD }
    }`

    requestBody := map[string]string{
        "query": query,
    }
    requestBodyBytes, err := json.Marshal(requestBody)
    if err != nil {
        return nil, fmt.Errorf("failed to marshal graphql request body for %s bundle: %w", source.Name, err)
    }

    req, err := http.NewRequest("POST", endpoint, bytes.NewBuffer(requestBodyBytes))
    if err != nil {
        return nil, fmt.Errorf("failed to create graphql request for %s bundle: %w", source.Name, err)
    }
    req.Header.Set("Content-Type", "application/json")

    resp, err := a.client.Do(req)
    if err != nil {
        return nil, fmt.Errorf("graphql bundle request failed for %s: %w", source.Name, err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        bodyBytes, _ := ioutil.ReadAll(resp.Body)
        return nil, fmt.Errorf("graphql bundle API error for %s: status %d, body: %s", source.Name, resp.StatusCode, string(bodyBytes))
    }

    // --- Response parsing specific to BUNDLE query --- 
    var response struct {
        Data struct {
            Bundles []struct {
                EthPriceUSD string `json:"ethPriceUSD"`
            } `json:"bundles"`
        } `json:"data"`
        Errors []interface{} `json:"errors"`
    }

    body, err := ioutil.ReadAll(resp.Body)
    if err != nil {
        return nil, fmt.Errorf("failed to read graphql bundle response body for %s: %w", source.Name, err)
    }
    if err := json.Unmarshal(body, &response); err != nil {
        return nil, fmt.Errorf("failed to unmarshal graphql bundle response for %s: %w, body: %s", source.Name, err, string(body))
    }
    if len(response.Errors) > 0 {
        return nil, fmt.Errorf("graphql bundle query for %s returned errors: %v", source.Name, response.Errors)
    }
    if len(response.Data.Bundles) == 0 {
        return nil, fmt.Errorf("no bundles found in subgraph response for %s", source.Name)
    }

    priceStr := response.Data.Bundles[0].EthPriceUSD
    price, err := parseFloat(priceStr)
    if err != nil {
        return nil, fmt.Errorf("failed to parse ethPriceUSD for %s: %w", source.Name, err)
    }

    return &common.PricePoint{
        Source:    source.Name, // Use the configured source name
        Price:     price,
        Volume:    0, // Bundle query doesn't provide relevant volume for the pair
        Timestamp: time.Now().UTC(),
    }, nil
}

// --- End Fetcher Functions ---

// calculateAggregatePrice performs robust price aggregation and caches details.
// Accepts AggregationParams directly.
func (a *CryptoAggregator) calculateAggregatePrice(prices []*common.PricePoint, params *common.AggregationParams, pairID string) (*common.PricePoint, error) {
    if len(prices) == 0 || params == nil {
        return nil, fmt.Errorf("invalid arguments for aggregation")
    }

    // Create a new slice for processing and caching with status
    processedPrices := make([]*common.PricePoint, len(prices))
    for i, p := range prices {
        // Shallow copy is okay here as we only add Status
        processedPrices[i] = &common.PricePoint{
            Source:    p.Source,
            Price:     p.Price,
            Volume:    p.Volume,
            Timestamp: p.Timestamp,
            Weight:    p.Weight,
            Status:    "unknown", // Default status
        }
    }

    // 1. Filter stale prices
    now := time.Now().UTC()
    maxAge := time.Duration(params.MaxPriceAgeSeconds) * time.Second
    validPrices := make([]*common.PricePoint, 0, len(processedPrices))
    for _, p := range processedPrices {
        if now.Sub(p.Timestamp) <= maxAge {
            p.Status = "pending_validation" // Mark as pending further checks
            validPrices = append(validPrices, p)
        } else {
            p.Status = "stale" // Mark as stale
            a.logger.Warn("Discarding stale price", "source", p.Source, "timestamp", p.Timestamp, "max_age", maxAge)
        }
    }

    if len(validPrices) < params.MinimumSources {
        // Cache the results even if aggregation fails due to insufficient sources
        a.cacheMutex.Lock()
        a.sourceDetailCache[pairID] = processedPrices
        a.cacheMutex.Unlock()
        return nil, fmt.Errorf("insufficient non-stale price sources: got %d, need %d", len(validPrices), params.MinimumSources)
    }

    // 2. Outlier Rejection using IQR
    sort.Slice(validPrices, func(i, j int) bool { // Sort by price for IQR calc
        return validPrices[i].Price < validPrices[j].Price
    })

    q1Index := len(validPrices) / 4
    q3Index := (len(validPrices) * 3) / 4

    if len(validPrices) < 4 {
        a.logger.Info("Skipping IQR outlier detection, too few data points", "count", len(validPrices))
        // Mark all remaining as valid
        for _, p := range validPrices {
            if p.Status == "pending_validation" { // Only update those that passed staleness
                p.Status = "valid"
            }
        }
    } else {
        q1 := validPrices[q1Index].Price
        q3 := validPrices[q3Index].Price
        iqr := q3 - q1
        k := params.IQRMultiplier
        lowerBound := q1 - k*iqr
        upperBound := q3 + k*iqr

        filteredPrices := make([]*common.PricePoint, 0, len(validPrices))
        for _, p := range validPrices {
            if p.Price >= lowerBound && p.Price <= upperBound {
                p.Status = "valid" // Mark as valid
                filteredPrices = append(filteredPrices, p)
            } else {
                p.Status = "outlier" // Mark as outlier
                a.logger.Warn("Discarding outlier price (IQR)",
                    "source", p.Source, "price", p.Price, "lower_bound", lowerBound, "upper_bound", upperBound)
            }
        }
        validPrices = filteredPrices // Update validPrices to the list that passed IQR
    }

    // Ensure any price point still marked as pending_validation is now valid
    // (This handles the case where IQR was skipped but staleness check passed)
    for _, p := range processedPrices {
        if p.Status == "pending_validation" {
            p.Status = "valid"
        }
    }

    // Cache the results WITH status BEFORE final checks/calculation
    a.cacheMutex.Lock()
    a.sourceDetailCache[pairID] = processedPrices // Cache the full list with statuses
    a.cacheMutex.Unlock()

    // 3. Check minimum sources *after* outlier rejection
    if len(validPrices) < params.MinimumSources {
        return nil, fmt.Errorf("insufficient non-outlier price sources: got %d, need %d", len(validPrices), params.MinimumSources)
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

    // 5. Aggregate Volume (sum of volumes from the final valid set)
    aggregateVolume := totalVolume // We already calculated this

    // 6. Return aggregated result
    return &common.PricePoint{
        Source:    "aggregated_vol_weighted_median",
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

// GetLastAggregationDetails retrieves the cached source details for a pair ID.
func (a *CryptoAggregator) GetLastAggregationDetails(pairID string) ([]*common.PricePoint, error) {
    a.cacheMutex.RLock()
    defer a.cacheMutex.RUnlock()
    details, ok := a.sourceDetailCache[pairID]
    if !ok {
        return nil, fmt.Errorf("no aggregation details found for pair %s", pairID)
    }
    // Return a deep copy to prevent caller from modifying the cache accidentally
    detailsCopy := make([]*common.PricePoint, len(details))
    for i, p := range details {
        // Create a new PricePoint for the copy
        copyPoint := *p
        detailsCopy[i] = &copyPoint
    }
    return detailsCopy, nil
}