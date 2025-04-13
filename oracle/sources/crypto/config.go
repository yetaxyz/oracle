package crypto

import (
    "encoding/json"
    "fmt"
    "io/ioutil"
    "path/filepath"
    "strings"
    
    "yetaXYZ/oracle/common"
)

var (
    BaseConfig *common.BaseConfig
    PairsConfig map[string]*common.PairConfig
)

// LoadConfig loads the configuration from the specified directory
func LoadConfig(configDir string) error {
    // Load base config
    baseConfigPath := filepath.Join(configDir, "base", "config.json")
    data, err := ioutil.ReadFile(baseConfigPath)
    if err != nil {
        return fmt.Errorf("failed to read base config: %v", err)
    }

    BaseConfig = &common.BaseConfig{}
    if err := json.Unmarshal(data, BaseConfig); err != nil {
        return fmt.Errorf("failed to parse base config: %v", err)
    }

    // Load pairs config
    pairsConfigPath := filepath.Join(configDir, "pairs", "pairs.json")
    data, err = ioutil.ReadFile(pairsConfigPath)
    if err != nil {
        return fmt.Errorf("failed to read pairs config: %v", err)
    }

    var pairsData struct {
        Pairs map[string]*common.PairConfig `json:"pairs"`
    }
    if err := json.Unmarshal(data, &pairsData); err != nil {
        return fmt.Errorf("failed to parse pairs config: %v", err)
    }
    PairsConfig = pairsData.Pairs

    return nil
}

// GetChainConfig returns the configuration for a specific chain
func GetChainConfig(chainID string) (*common.Chain, error) {
    config, ok := BaseConfig.Chains[chainID]
    if !ok {
        return nil, fmt.Errorf("chain config not found for ID: %s", chainID)
    }
    return &config, nil
}

// GetAssetConfig returns the configuration for a specific asset
func GetAssetConfig(symbol string) (*common.Asset, error) {
    config, ok := BaseConfig.Assets[symbol]
    if !ok {
        return nil, fmt.Errorf("asset config not found for symbol: %s", symbol)
    }
    return &config, nil
}

// GetPairConfig returns the configuration for a specific trading pair
func GetPairConfig(symbol string) (*common.PairConfig, error) {
    // Convert symbol format from BTC/USDT to BTCUSDT
    symbol = strings.ReplaceAll(symbol, "/", "")
    
    config, ok := PairsConfig[symbol]
    if !ok {
        return nil, fmt.Errorf("pair config not found for symbol: %s", symbol)
    }
    return config, nil
}

// getExchangesForAssets returns a list of CEX exchanges that support both assets
func getExchangesForAssets(baseAsset, quoteAsset *common.Asset) []string {
    // Get exchanges that support both assets
    exchanges := make([]string, 0)
    for name, details := range BaseConfig.Exchanges.CEX {
        if supportsAssets(details, baseAsset, quoteAsset) {
            exchanges = append(exchanges, name)
        }
    }
    return exchanges
}

// getDEXExchangesForAssets returns a map of chain to DEX list that support both assets
func getDEXExchangesForAssets(baseAsset, quoteAsset *common.Asset) map[string][]string {
    dexMap := make(map[string][]string)
    
    // Check each chain where both assets exist
    for chainID := range baseAsset.Chains {
        if _, ok := quoteAsset.Chains[chainID]; ok {
            // Add DEXes for this chain
            dexes := make([]string, 0)
            for name, details := range BaseConfig.Exchanges.DEX {
                if supportsDEXAssets(details, chainID, baseAsset, quoteAsset) {
                    dexes = append(dexes, name)
                }
            }
            if len(dexes) > 0 {
                dexMap[chainID] = dexes
            }
        }
    }
    
    return dexMap
}

// supportsAssets checks if a CEX supports trading both assets
func supportsAssets(exchange common.CEXDetails, baseAsset, quoteAsset *common.Asset) bool {
    // For now, assume all CEXs support all assets
    // In a real implementation, this would check the exchange's supported pairs
    return true
}

// supportsDEXAssets checks if a DEX supports trading both assets on the given chain
func supportsDEXAssets(exchange common.DEXDetails, chainID string, baseAsset, quoteAsset *common.Asset) bool {
    // For now, assume all DEXs support all assets on their chain
    // In a real implementation, this would check the DEX's supported pairs and liquidity
    return true
}

// ValidateConfig performs validation of the loaded configuration
func ValidateConfig() error {
    if BaseConfig == nil {
        return fmt.Errorf("base configuration not loaded")
    }

    if PairsConfig == nil {
        return fmt.Errorf("pairs configuration not loaded")
    }

    if len(BaseConfig.Exchanges.CEX) == 0 && len(BaseConfig.Exchanges.DEX) == 0 {
        return fmt.Errorf("no exchanges configured")
    }

    if len(BaseConfig.Assets) == 0 {
        return fmt.Errorf("no assets configured")
    }

    if len(PairsConfig) == 0 {
        return fmt.Errorf("no trading pairs configured")
    }

    return nil
} 