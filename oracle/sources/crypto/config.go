package crypto

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"

	"yetaXYZ/oracle/common"
)

// loadJSONFile reads and unmarshals a JSON file into the provided interface value.
func loadJSONFile(filePath string, target interface{}) error {
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", filePath, err)
	}
	if err := json.Unmarshal(data, target); err != nil {
		return fmt.Errorf("failed to parse %s: %w", filePath, err)
	}
	return nil
}

// LoadAllConfigs loads all configuration files (chains, assets, sources, pairs)
// from the specified directory and returns a consolidated, validated config object.
func LoadAllConfigs(configDir string) (*common.LoadedConfig, error) {
	loadedCfg := &common.LoadedConfig{
		Chains:  make(map[string]common.Chain),
		Assets:  make(map[string]common.Asset),
		Sources: make(map[string]common.Source),
		Pairs:   make(map[string]common.PairConfig),
	}

	// Load Chains
	if err := loadJSONFile(filepath.Join(configDir, "chains.json"), &loadedCfg.Chains); err != nil {
		return nil, err
	}

	// Load Assets
	if err := loadJSONFile(filepath.Join(configDir, "assets.json"), &loadedCfg.Assets); err != nil {
		return nil, err
	}

	// Load Sources
	tempSources := make(map[string]common.Source) // Temporary map to load into
	if err := loadJSONFile(filepath.Join(configDir, "sources.json"), &tempSources); err != nil {
		return nil, err
	}
	// Assign IDs to sources based on map keys
	for id, source := range tempSources {
		source.ID = id
		loadedCfg.Sources[id] = source
	}

	// Load Pairs
	tempPairs := make(map[string]common.PairConfig) // Temporary map to load into
	if err := loadJSONFile(filepath.Join(configDir, "pairs.json"), &tempPairs); err != nil {
		return nil, err
	}
	// Assign IDs to pairs based on map keys
	for id, pair := range tempPairs {
		pair.ID = id
		loadedCfg.Pairs[id] = pair
	}

	// Validate the loaded configuration
	if err := ValidateLoadedConfig(loadedCfg); err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}

	return loadedCfg, nil
}

// ValidateLoadedConfig checks the consistency and integrity of the loaded configuration.
func ValidateLoadedConfig(config *common.LoadedConfig) error {
	if config == nil {
		return fmt.Errorf("config object is nil")
	}
	if len(config.Chains) == 0 {
		// Allow no chains if only CEX sources are used?
		// return fmt.Errorf("no chains defined in chains.json")
	}
	if len(config.Assets) == 0 {
		return fmt.Errorf("no assets defined in assets.json")
	}
	if len(config.Sources) == 0 {
		return fmt.Errorf("no sources defined in sources.json")
	}
	if len(config.Pairs) == 0 {
		return fmt.Errorf("no pairs defined in pairs.json")
	}

	// Validate sources
	for id, source := range config.Sources {
		if source.Type == common.SourceTypeDEXRPC || source.Type == common.SourceTypeDEXSubgraph {
			if source.ChainID == "" {
				return fmt.Errorf("source '%s' is a DEX type but missing required 'chainId'", id)
			}
			if _, ok := config.Chains[source.ChainID]; !ok {
				return fmt.Errorf("source '%s' references unknown chainId '%s'", id, source.ChainID)
			}
		}
		// Add more source validation (e.g., required fields per type)
	}

	// Validate pairs
	for id, pair := range config.Pairs {
		if _, ok := config.Assets[pair.BaseAsset]; !ok {
			return fmt.Errorf("pair '%s' references unknown baseAsset '%s'", id, pair.BaseAsset)
		}
		if _, ok := config.Assets[pair.QuoteAsset]; !ok {
			return fmt.Errorf("pair '%s' references unknown quoteAsset '%s'", id, pair.QuoteAsset)
		}
		if pair.ChainID != "" && pair.ChainID != "global" { // Allow empty or "global"
			if _, ok := config.Chains[pair.ChainID]; !ok {
				return fmt.Errorf("pair '%s' references unknown chainId '%s'", id, pair.ChainID)
			}
		}
		if len(pair.SourceIDs) == 0 {
			return fmt.Errorf("pair '%s' has no sources defined", id)
		}
		if pair.Aggregation.MinimumSources <= 0 {
			return fmt.Errorf("pair '%s' has invalid minimumSources '%d'", id, pair.Aggregation.MinimumSources)
		}
		if pair.Aggregation.MinimumSources > len(pair.SourceIDs) {
			return fmt.Errorf("pair '%s' minimumSources (%d) exceeds number of defined sources (%d)", id, pair.Aggregation.MinimumSources, len(pair.SourceIDs))
		}
		for _, sourceID := range pair.SourceIDs {
			if _, ok := config.Sources[sourceID]; !ok {
				return fmt.Errorf("pair '%s' references unknown source '%s'", id, sourceID)
			}
			if _, ok := pair.SourceWeights[sourceID]; !ok {
				return fmt.Errorf("pair '%s' is missing weight for source '%s'", id, sourceID)
			}
			// Ensure source chain matches pair chain if applicable
			src := config.Sources[sourceID]
			if src.ChainID != "" && pair.ChainID != "global" && src.ChainID != pair.ChainID {
				return fmt.Errorf("pair '%s' on chain '%s' cannot use source '%s' on chain '%s'", id, pair.ChainID, sourceID, src.ChainID)
			}

		}
		// Check if total weight is reasonable (e.g., sums to 1 or positive)?
	}

	return nil
}

// GetResolvedPairConfig looks up a pair by its ID and resolves all references
// to return a consolidated configuration object ready for the aggregator.
func GetResolvedPairConfig(loadedCfg *common.LoadedConfig, pairID string) (*common.ResolvedPairConfig, error) {
	if loadedCfg == nil {
		return nil, fmt.Errorf("loaded configuration is nil")
	}

	pairCfg, ok := loadedCfg.Pairs[pairID]
	if !ok {
		return nil, fmt.Errorf("pair configuration not found for ID: %s", pairID)
	}

	baseAsset, ok := loadedCfg.Assets[pairCfg.BaseAsset]
	if !ok {
		// Should be caught by validation, but check defensively
		return nil, fmt.Errorf("base asset '%s' not found for pair '%s'", pairCfg.BaseAsset, pairID)
	}

	quoteAsset, ok := loadedCfg.Assets[pairCfg.QuoteAsset]
	if !ok {
		return nil, fmt.Errorf("quote asset '%s' not found for pair '%s'", pairCfg.QuoteAsset, pairID)
	}

	var chain *common.Chain
	if pairCfg.ChainID != "" && pairCfg.ChainID != "global" {
		chainDetails, ok := loadedCfg.Chains[pairCfg.ChainID]
		if !ok {
			return nil, fmt.Errorf("chain '%s' not found for pair '%s'", pairCfg.ChainID, pairID)
		}
		chain = &chainDetails
	}

	resolvedSources := make([]common.ResolvedSource, 0, len(pairCfg.SourceIDs))
	for _, sourceID := range pairCfg.SourceIDs {
		sourceDetails, ok := loadedCfg.Sources[sourceID]
		if !ok {
			// Should be caught by validation
			return nil, fmt.Errorf("source '%s' not found for pair '%s'", sourceID, pairID)
		}
		resolvedSources = append(resolvedSources, common.ResolvedSource{
			SourceID: sourceID,
			Details:  &sourceDetails,
		})
	}

	resolved := &common.ResolvedPairConfig{
		PairID:             pairID,
		BaseAsset:          &baseAsset,
		QuoteAsset:         &quoteAsset,
		Chain:              chain, // nil if global
		AggregationParams:  pairCfg.Aggregation,
		ResolvedSources:    resolvedSources,
		SourceWeights:      pairCfg.SourceWeights,
	}

	return resolved, nil
}

// Note: Removed old LoadConfig, ValidateConfig, Get*Config functions. 