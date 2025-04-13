package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/gorilla/mux"
	"github.com/rs/cors"
	"yetaXYZ/oracle/common"
	"yetaXYZ/oracle/sources/crypto"
)

// Server represents the API server
type Server struct {
	router     *mux.Router
	aggregator *crypto.CryptoAggregator
	config     *common.BaseConfig
}

// NewServer creates a new API server
func NewServer() (*Server, error) {
	// Load configuration
	configDir := filepath.Join("..", "config")
	if err := crypto.LoadConfig(configDir); err != nil {
		return nil, fmt.Errorf("failed to load config: %v", err)
	}

	// Validate configuration
	if err := crypto.ValidateConfig(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %v", err)
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

		// Fetch price using the original symbol format
		price, err := s.aggregator.FetchPrice(symbol)
		if err != nil {
			log.Printf("Error fetching price for %s: %v", symbol, err)
			http.Error(w, fmt.Sprintf("failed to fetch price: %v", err), http.StatusInternalServerError)
			return
		}

		// Return response
		response := map[string]interface{}{
			"symbol":    symbol,
			"price":     price.Price,
			"volume":    price.Volume,
			"timestamp": price.Timestamp,
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
			"timestamp": time.Now(),
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

	// Setup CORS
	c := cors.New(cors.Options{
		AllowedOrigins: []string{"*"},  // Allow all origins
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{"*"},
		Debug:         false,  // Disable debug mode to remove CORS logging
	})

	// Wrap router with CORS middleware
	handler := c.Handler(server.router)

	log.Printf("Server starting on port %s", port)
	if err := http.ListenAndServe(":"+port, handler); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
} 