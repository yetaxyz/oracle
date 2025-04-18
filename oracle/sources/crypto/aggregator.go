package crypto

import (
    "bytes"
    "encoding/json"
    "fmt"
    "io/ioutil"
    "log"
    "net/http"
    "sort"
    "time"
    "yetaXYZ/oracle/common"
)

// CryptoAggregator handles cryptocurrency price aggregation
type CryptoAggregator struct {
    config *common.BaseConfig
    client *http.Client
}

// NewCryptoAggregator creates a new CryptoAggregator
func NewCryptoAggregator(config *common.BaseConfig) *CryptoAggregator {
    return &CryptoAggregator{
        config: config,
        client: &http.Client{
            Timeout: 10 * time.Second,
        },
    }
}

// FetchPrice fetches the price for a given trading pair
func (a *CryptoAggregator) FetchPrice(symbol string) (*common.PricePoint, error) {
    // Get pair configuration
    pairConfig, err := GetPairConfig(symbol)
    if err != nil {
        return nil, fmt.Errorf("failed to get pair config: %v", err)
    }

    prices := make([]*common.PricePoint, 0)

    // Fetch from enabled CEX sources
    if pairConfig.Sources.CEX.Enabled {
        for _, exchange := range pairConfig.Sources.CEX.Exchanges {
            var price *common.PricePoint
            var err error

            switch exchange {
            case "binance":
                price, err = a.fetchBinancePrice(symbol)
            case "coinbase":
                price, err = a.fetchCoinbasePrice(pairConfig.BaseCurrency + "-" + pairConfig.QuoteCurrency)
            case "kraken":
                price, err = a.fetchKrakenPrice(symbol)
            }

            if err != nil {
                log.Printf("Error fetching price from %s for %s: %v", exchange, symbol, err)
                continue
            }

            if price != nil {
                price.Price *= pairConfig.Sources.CEX.Weight
                prices = append(prices, price)
            }
        }
    }

    // Fetch from enabled DEX sources
    if pairConfig.Sources.DEX.Enabled {
        for _, source := range pairConfig.Sources.DEX.Sources {
            var price *common.PricePoint
            var err error

            switch source.Name {
            case "uniswap_v3":
                price, err = a.fetchUniswapV3Price(source.Endpoint, source.PoolAddress)
            }

            if err != nil {
                log.Printf("Error fetching price from DEX %s for %s: %v", source.Name, symbol, err)
                continue
            }

            if price != nil {
                price.Price *= pairConfig.Sources.DEX.Weight
                prices = append(prices, price)
            }
        }
    }

    if len(prices) < pairConfig.MinimumSources {
        return nil, fmt.Errorf("insufficient price sources for %s: got %d, need %d", symbol, len(prices), pairConfig.MinimumSources)
    }

    // Calculate median price
    return a.calculateMedian(prices), nil
}

// fetchBinancePrice fetches price from Binance
func (a *CryptoAggregator) fetchBinancePrice(symbol string) (*common.PricePoint, error) {
    url := fmt.Sprintf("https://api.binance.com/api/v3/ticker/24hr?symbol=%s", symbol)
    resp, err := a.client.Get(url)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    var data struct {
        LastPrice string `json:"lastPrice"`
        Volume    string `json:"volume"`
    }

    body, err := ioutil.ReadAll(resp.Body)
    if err != nil {
        return nil, err
    }

    if err := json.Unmarshal(body, &data); err != nil {
        return nil, err
    }

    price, err := parseFloat(data.LastPrice)
    if err != nil {
        return nil, err
    }

    volume, err := parseFloat(data.Volume)
    if err != nil {
        return nil, err
    }

    return &common.PricePoint{
        Price:     price,
        Volume:    volume,
        Timestamp: time.Now(),
    }, nil
}

// fetchCoinbasePrice fetches price from Coinbase
func (a *CryptoAggregator) fetchCoinbasePrice(symbol string) (*common.PricePoint, error) {
    url := fmt.Sprintf("https://api.coinbase.com/v2/prices/%s/spot", symbol)
    resp, err := a.client.Get(url)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    var data struct {
        Data struct {
            Amount string `json:"amount"`
        } `json:"data"`
    }

    if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
        return nil, err
    }

    price, err := parseFloat(data.Data.Amount)
    if err != nil {
        return nil, err
    }

    return &common.PricePoint{
        Price:     price,
        Volume:    0, // Coinbase spot API doesn't provide volume
        Timestamp: time.Now(),
    }, nil
}

// fetchKrakenPrice fetches price from Kraken
func (a *CryptoAggregator) fetchKrakenPrice(symbol string) (*common.PricePoint, error) {
    url := fmt.Sprintf("https://api.kraken.com/0/public/Ticker?pair=%s", symbol)
    resp, err := a.client.Get(url)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    var data struct {
        Result map[string]struct {
            LastTrade []string `json:"c"`
            Volume    []string `json:"v"`
        } `json:"result"`
    }

    if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
        return nil, err
    }

    // Kraken returns data in a map with the pair name as key
    var result struct {
        LastTrade []string
        Volume    []string
    }
    for _, v := range data.Result {
        result = struct {
            LastTrade []string
            Volume    []string
        }{
            LastTrade: v.LastTrade,
            Volume:    v.Volume,
        }
        break
    }

    if len(result.LastTrade) < 1 || len(result.Volume) < 1 {
        return nil, fmt.Errorf("invalid response from Kraken")
    }

    price, err := parseFloat(result.LastTrade[0])
    if err != nil {
        return nil, err
    }

    volume, err := parseFloat(result.Volume[0])
    if err != nil {
        return nil, err
    }

    return &common.PricePoint{
        Price:     price,
        Volume:    volume,
        Timestamp: time.Now(),
    }, nil
}

// fetchUniswapV3Price fetches price from Uniswap V3
func (a *CryptoAggregator) fetchUniswapV3Price(endpoint, poolAddress string) (*common.PricePoint, error) {
    log.Printf("Fetching price from Uniswap V3, endpoint: %s, pool: %s", endpoint, poolAddress)
    
    // Set API key for The Graph
    apiKey := "45d074d31e37af2cdc395665d35964c9"
    
    // GraphQL query for ETH price via a specific pool
    query := fmt.Sprintf(`{
        pool(id: "%s") {
            token0Price
            token1Price
            volumeUSD
            token0 {
                symbol
            }
            token1 {
                symbol
            }
        }
    }`, poolAddress)

    // Prepare request body
    requestBody := map[string]interface{}{
        "query": query,
    }

    bodyBytes, err := json.Marshal(requestBody)
    if err != nil {
        return nil, fmt.Errorf("failed to marshal request body: %v", err)
    }

    // Make request
    req, err := http.NewRequest("POST", endpoint, bytes.NewBuffer(bodyBytes))
    if err != nil {
        return nil, fmt.Errorf("failed to create request: %v", err)
    }
    req.Header.Set("Content-Type", "application/json")
    
    // Add the API key to the request
    if apiKey != "" {
        req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", apiKey))
    }

    resp, err := a.client.Do(req)
    if err != nil {
        return nil, fmt.Errorf("failed to make request: %v", err)
    }
    defer resp.Body.Close()

    // Read and log response for debugging
    respBody, err := ioutil.ReadAll(resp.Body)
    if err != nil {
        return nil, fmt.Errorf("failed to read response body: %v", err)
    }
    log.Printf("Uniswap V3 response: %s", string(respBody))

    // Parse response
    var result struct {
        Data struct {
            Pool struct {
                Token0Price string `json:"token0Price"`
                Token1Price string `json:"token1Price"`
                VolumeUSD   string `json:"volumeUSD"`
                Token0      struct {
                    Symbol string `json:"symbol"`
                } `json:"token0"`
                Token1 struct {
                    Symbol string `json:"symbol"`
                } `json:"token1"`
            } `json:"pool"`
        } `json:"data"`
    }

    if err := json.Unmarshal(respBody, &result); err != nil {
        return nil, fmt.Errorf("failed to parse response: %v", err)
    }

    if result.Data.Pool.Token0Price == "" && result.Data.Pool.Token1Price == "" {
        return nil, fmt.Errorf("no price data found in response")
    }

    // Determine which price to use based on token order
    var price float64
    if result.Data.Pool.Token0.Symbol == "USDT" {
        // If USDT is token0, use token1Price (1/ETH price in USDT)
        price, err = parseFloat(result.Data.Pool.Token0Price)
    } else {
        // If ETH is token0, use token0Price (ETH price in USDT)
        price, err = parseFloat(result.Data.Pool.Token1Price)
    }

    if err != nil {
        return nil, fmt.Errorf("failed to parse price: %v", err)
    }

    // Parse volume
    volume := 0.0
    if result.Data.Pool.VolumeUSD != "" {
        volume, _ = parseFloat(result.Data.Pool.VolumeUSD)
    }

    log.Printf("Received Uniswap V3 ETH price: %f, volume: %f", price, volume)

    return &common.PricePoint{
        Price:     price,
        Volume:    volume,
        Timestamp: time.Now(),
    }, nil
}

// calculateMedian calculates the median price from multiple sources
func (a *CryptoAggregator) calculateMedian(prices []*common.PricePoint) *common.PricePoint {
    if len(prices) == 0 {
        return nil
    }

    // Sort prices
    sort.Slice(prices, func(i, j int) bool {
        return prices[i].Price < prices[j].Price
    })

    // Calculate median price and total volume
    medianIdx := len(prices) / 2
    totalVolume := 0.0
    for _, p := range prices {
        totalVolume += p.Volume
    }

    return &common.PricePoint{
        Price:     prices[medianIdx].Price,
        Volume:    totalVolume,
        Timestamp: time.Now(),
    }
}

// parseFloat helper function to parse string to float64
func parseFloat(s string) (float64, error) {
    var f float64
    _, err := fmt.Sscanf(s, "%f", &f)
    return f, err
}