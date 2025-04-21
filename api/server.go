package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	// "path/filepath" // Not needed anymore
	"regexp"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
	"github.com/rs/cors"
	"yetaXYZ/oracle/common"
	"yetaXYZ/oracle/sources/crypto"
)

// ApiError defines the structure for standard JSON error responses
type ApiError struct {
	Code    string `json:"code"`    // e.g., "INVALID_INPUT", "INTERNAL_ERROR"
	Message string `json:"message"` // User-friendly error message
}

// Define error codes
const (
	ErrCodeInvalidSymbol   = "INVALID_SYMBOL"
	ErrCodePriceFetchFailed = "PRICE_FETCH_FAILED"
	ErrCodeInternalError    = "INTERNAL_ERROR"
)

// writeJsonError is a helper to write standardized JSON errors
func writeJsonError(w http.ResponseWriter, statusCode int, errCode string, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(map[string]ApiError{"error": {Code: errCode, Message: message}})
}

// Regex for basic symbol validation (e.g., BASE_QUOTE like ETH_USDC)
// Updated to allow underscore, adjust if pair ID format differs
var symbolRegex = regexp.MustCompile(`^[A-Z]{2,8}[-_][A-Z]{2,8}$`)
var chainRegex = regexp.MustCompile(`^[a-z0-9_-]{2,20}$`) // Basic chain ID validation

// Server represents the API server
type Server struct {
	router     *mux.Router
	aggregator *crypto.CryptoAggregator
	// config     *common.BaseConfig // Removed old config
	loadedConfig *common.LoadedConfig // Holds all loaded configs
}

// NewServer creates a new API server
func NewServer() (*Server, error) {
	// Load configuration files
	configDir := "config"
	cfg, err := crypto.LoadAllConfigs(configDir)
	if err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}
	log.Println("Configuration loaded and validated successfully.")

	// Create aggregator (no longer takes config directly)
	aggregator := crypto.NewCryptoAggregator()

	server := &Server{
		router:       mux.NewRouter(),
		aggregator:   aggregator,
		loadedConfig: cfg, // Store loaded config
	}

	server.routes()
	return server, nil
}

// routes sets up the API routes
func (s *Server) routes() {
	// Updated route - symbol might include underscore (e.g., ETH_USDC)
	s.router.HandleFunc("/api/v1/prices/{symbol}", s.handleGetPrice()).Methods("GET") // Chain via query param
	s.router.HandleFunc("/api/v1/prices/{symbol}/sources", s.handleGetSourceDetails()).Methods("GET") // Chain via query param
	s.router.HandleFunc("/api/v1/health", s.handleHealth()).Methods("GET")
	// TODO: Add endpoint to list available feeds (/api/v1/feeds)
}

// handleGetPrice handles price requests
func (s *Server) handleGetPrice() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		symbol := strings.ToUpper(vars["symbol"]) // Ensure uppercase
		chainID := strings.ToLower(r.URL.Query().Get("chain")) // Get chain from query, lowercase
		if chainID == "" {
			chainID = "global" // Default to global feed
		}

		// --- Input Validation ---
		if !symbolRegex.MatchString(symbol) {
			writeJsonError(w, http.StatusBadRequest, ErrCodeInvalidSymbol,
				fmt.Sprintf("Invalid symbol format: '%s'. Expected format like BASE_QUOTE.", symbol))
			return
		}
		if !chainRegex.MatchString(chainID) {
			writeJsonError(w, http.StatusBadRequest, "INVALID_CHAIN_ID",
				fmt.Sprintf("Invalid chain ID format: '%s'.", chainID))
			return
		}

		// Construct the Pair ID used in config (e.g., ETHUSDC_Global, SOLUSDC_Solana)
		// Assumes format SYMBOL_ChainID (needs to match keys in pairs.json)
		// Need to handle the symbol format correctly (e.g. ETH_USDC vs ETHUSDC)
		// Let's assume keys in pairs.json are like ETHUSDC_Global
		pairSymbolOnly := strings.ReplaceAll(symbol, "_", "") // Get symbol part like ETHUSDC
		pairID := pairSymbolOnly + "_" + strings.Title(chainID) // Construct ID like ETHUSDC_Global or ETHUSDC_Solana

		// Get the resolved configuration for this specific pair feed
		resolvedConfig, err := crypto.GetResolvedPairConfig(s.loadedConfig, pairID)
		if err != nil {
			log.Printf("Error resolving config for pair %s: %v", pairID, err)
			writeJsonError(w, http.StatusNotFound, "PAIR_NOT_CONFIGURED",
				fmt.Sprintf("Price feed not configured for '%s' on chain '%s'.", symbol, chainID))
			return
		}

		// Fetch price using the resolved config
		price, err := s.aggregator.FetchPrice(resolvedConfig)
		if err != nil {
			// Error already logged in aggregator FetchPrice
			writeJsonError(w, http.StatusInternalServerError, ErrCodePriceFetchFailed,
				fmt.Sprintf("Failed to fetch aggregated price for '%s' on chain '%s'.", symbol, chainID))
			return
		}

		if price == nil { // Should not happen if FetchPrice returns nil error
			log.Printf("Aggregator returned nil price for %s without error", pairID)
			writeJsonError(w, http.StatusInternalServerError, ErrCodeInternalError, "Internal error retrieving price.")
			return
		}

		// Return successful response
		response := map[string]interface{}{
			"feedID":    pairID,
			"symbol":    symbol, // Original requested symbol
			"chain":     chainID,
			"price":     price.Price,
			"volume":    price.Volume,
			"source":    price.Source, // Aggregation method/status
			"timestamp": price.Timestamp.UTC().Format(time.RFC3339Nano),
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}
}

// handleGetSourceDetails handles requests for pre-aggregation source data
func (s *Server) handleGetSourceDetails() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		symbol := strings.ToUpper(vars["symbol"])
		chainID := strings.ToLower(r.URL.Query().Get("chain"))
		if chainID == "" {
			chainID = "global"
		}

		// Basic validation
		if !symbolRegex.MatchString(symbol) {
			writeJsonError(w, http.StatusBadRequest, ErrCodeInvalidSymbol, fmt.Sprintf("Invalid symbol format: '%s'.", symbol))
			return
		}
		if !chainRegex.MatchString(chainID) {
			writeJsonError(w, http.StatusBadRequest, "INVALID_CHAIN_ID", fmt.Sprintf("Invalid chain ID format: '%s'.", chainID))
			return
		}

		// Construct Pair ID
		pairSymbolOnly := strings.ReplaceAll(symbol, "_", "")
		pairID := pairSymbolOnly + "_" + strings.Title(chainID)

		details, err := s.aggregator.GetLastAggregationDetails(pairID) // Use PairID as key
		if err != nil {
			// Error already logged in aggregator if needed
			if strings.Contains(err.Error(), "not found") {
				writeJsonError(w, http.StatusNotFound, "NO_DETAILS_FOUND", fmt.Sprintf("No source details available for feed '%s'. Aggregate first?", pairID))
			} else {
				writeJsonError(w, http.StatusInternalServerError, ErrCodeInternalError, "Failed to retrieve source details.")
			}
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(details)
	}
}

// handleHealth handles health check requests
func (s *Server) handleHealth() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"status":    "ok",
			"timestamp": time.Now().UTC().Format(time.RFC3339Nano), // Use standard ISO 8601 format
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}
}

func main() {
	// Load .env file. Ignore error if file doesn't exist.
	err := godotenv.Load()
	if err != nil {
		log.Println("Info: Error loading .env file, relying on system environment variables:", err)
	}

	server, err := NewServer()
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Setup CORS (Consider making AllowedOrigins configurable for production)
	c := cors.New(cors.Options{
		AllowedOrigins: []string{"*"}, // TODO: Restrict in production
		AllowedMethods: []string{"GET", "OPTIONS"}, // Limit methods if applicable
		AllowedHeaders: []string{"Content-Type", "Authorization"}, // Specify needed headers
		AllowCredentials: true,
		Debug:         false,
	})

	handler := c.Handler(server.router)

	log.Printf("API Server starting on port %s", port)
	if err := http.ListenAndServe(":"+port, handler); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
} 