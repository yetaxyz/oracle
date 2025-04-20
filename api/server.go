package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	// "path/filepath" // Removed unused import
	"regexp"
	"time"

	"github.com/gorilla/mux"
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

// Regex for basic symbol validation (e.g., 3-12 uppercase letters)
var symbolRegex = regexp.MustCompile(`^[A-Z]{3,12}$`)

// Server represents the API server
type Server struct {
	router     *mux.Router
	aggregator *crypto.CryptoAggregator
	config     *common.BaseConfig
}

// NewServer creates a new API server
func NewServer() (*Server, error) {
	// Load configuration (relative to workspace root where command is run)
	// configDir := filepath.Join("..", "config") // Old path relative to api/
	configDir := "config" // Path relative to workspace root
	if err := crypto.LoadConfig(configDir); err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	// Validate configuration
	if err := crypto.ValidateConfig(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	// Create aggregator
	aggregator := crypto.NewCryptoAggregator(crypto.BaseConfig)

	server := &Server{
		router:     mux.NewRouter(),
		aggregator: aggregator,
		config:     crypto.BaseConfig,
	}

	server.routes()
	return server, nil
}

// routes sets up the API routes
func (s *Server) routes() {
	s.router.HandleFunc("/api/v1/prices/{symbol}", s.handleGetPrice()).Methods("GET")
	s.router.HandleFunc("/api/v1/health", s.handleHealth()).Methods("GET")
}

// handleGetPrice handles price requests
func (s *Server) handleGetPrice() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		symbol := vars["symbol"]

		// --- Input Validation ---
		if !symbolRegex.MatchString(symbol) {
			writeJsonError(w, http.StatusBadRequest, ErrCodeInvalidSymbol,
				fmt.Sprintf("Invalid symbol format: '%s'. Expected 3-12 uppercase letters.", symbol))
			return
		}
		// TODO: Add validation against actual list of supported symbols from config

		// Fetch price using the validated symbol
		price, err := s.aggregator.FetchPrice(symbol)
		if err != nil {
			log.Printf("Error fetching price for %s: %v", symbol, err)
			// Use standardized JSON error response
			writeJsonError(w, http.StatusInternalServerError, ErrCodePriceFetchFailed,
				fmt.Sprintf("Failed to fetch price for symbol '%s'.", symbol))
			// Note: Exposing internal error details (err.Error()) is generally discouraged in production.
			return
		}

		if price == nil { // Defensive check in case FetchPrice returns nil without error
			log.Printf("FetchPrice returned nil price for %s without error", symbol)
			writeJsonError(w, http.StatusInternalServerError, ErrCodeInternalError, "Received nil price internally.")
			return
		}

		// Return successful response
		response := map[string]interface{}{
			"symbol":    symbol,
			"price":     price.Price,
			"volume":    price.Volume,       // Include volume from aggregation
			"source":    price.Source,       // Include source info (e.g., "aggregated_vol_weighted_median")
			"timestamp": price.Timestamp.UTC().Format(time.RFC3339Nano), // Use standard ISO 8601 format
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
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