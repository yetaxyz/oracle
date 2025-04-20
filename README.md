# Cryptocurrency Oracle System

A robust and modular cryptocurrency price oracle system that aggregates prices from multiple sources including centralized exchanges (CEX) and decentralized exchanges (DEX).

## Project Structure

```
.
├── api/                    # REST API server implementation
│   └── server.go          # Main API server with price endpoints
├── config/                # Configuration files
│   ├── base/             # Base configurations
│   │   └── chains.json   # Blockchain network configurations
│   ├── pairs/            # Trading pair configurations
│   │   └── pairs.json    # Price pair settings and sources
│   └── assets/           # Asset-specific configurations
├── oracle/               # Core oracle implementation
│   ├── common/          # Shared types and utilities
│   ├── sources/         # Price source implementations
│   │   └── crypto/      # Cryptocurrency price sources
│   └── aggregator/      # Price aggregation logic
├── web/                 # Frontend applications
│   └── dashboard/       # React-based admin dashboard
│       ├── public/      # Static assets
│       └── src/         # React source code
│           ├── components/  # React components
│           └── config.js    # Frontend configuration
├── contracts/           # Smart contract implementations
├── cmd/                 # Command-line tools
└── go.mod              # Go module definition
```

## Components

### API Server (`api/`)
- REST API server built with Go and Gorilla Mux
- Endpoints:
  - `GET /api/v1/prices/{symbol}`: Get current price for a trading pair
  - `GET /api/v1/health`: Health check endpoint
- Features:
  - CORS support (`rs/cors`)
  - Configurable port (default: 8080)
  - **Input Validation:** Basic validation for `{symbol}` format.
  - **Standardized JSON Errors:** Returns errors in a structured JSON format (`{"error":{"code":"...","message":"..."}}`).
  - Structured Logging (`log/slog`) for internal errors.

### Configuration (`config/`)
- Modular configuration system with JSON files
- `base/chains.json`: Blockchain network configurations
  - Network IDs, RPC endpoints, native currencies
- `pairs/pairs.json`: Trading pair configurations
  - Supported trading pairs (e.g., BTCUSDT, ETHUSDT)
  - Price source settings
  - Update frequency and minimum source requirements
- `assets/`: Asset-specific configurations

### Oracle Core (`oracle/`)
- `common/`: Shared types and utilities
  - Price point structures
  - Configuration types
  - Common interfaces
- `sources/crypto/`: Cryptocurrency price sources
  - Support for multiple exchanges:
    - Binance
    - Coinbase
    - Kraken
    - Uniswap V3 (via Subgraph)
  - Configurable weights for each source (`config/pairs/pairs.json`)
- `aggregator/`: Price aggregation logic
  - **Robust Aggregation Pipeline:**
    1.  **Staleness Filtering:** Removes price points older than the configured `MaxPriceAge`.
    2.  **Outlier Rejection (IQR):** Calculates the first (Q1) and third (Q3) quartiles of the remaining prices. Removes prices outside the range `[Q1 - k*IQR, Q3 + k*IQR]`, where `IQR = Q3 - Q1` and `k` is the configured `IQRMultiplier`.
    3.  **Volume-Enhanced Weighted Median Calculation:** Calculates a dynamic weight for each valid price point, incorporating its configured static weight and its reported volume relative to the total volume of the valid set. Sorts prices and finds the price \(p_{(j)}\) such that the sum of *dynamic weights* of prices up to \(p_{(j)}\) is at least 50% of the total dynamic weight.
    4.  **Volume Aggregation:** Sums the volume from all sources that passed the filtering steps.
  - Source validation (Minimum number of sources required after filtering)
  - Error handling and structured logging

### Web Dashboard (`web/dashboard/`)
- React-based admin interface
- Features:
  - Real-time price monitoring
  - Price history visualization
  - Source health monitoring
- Components:
  - PriceTable: Displays current prices
  - Configuration management
  - Error reporting

### Smart Contracts (`contracts/`)
- Smart contract implementations
- Hardhat configuration for deployment

## Configuration

### Trading Pairs
Supported trading pairs are configured in `config/pairs/pairs.json`:
- BTCUSDT (Bitcoin/USDT)
- ETHUSDT (Ethereum/USDT)
- BNBUSDT (Binance Coin/USDT)
- XRPUSDT (Ripple/USDT)
- ADAUSDT (Cardano/USDT)

Each pair configuration includes:
- Base and quote currencies
- Minimum required sources
- Update frequency
- Enabled exchanges and DEX sources
- Per-source weights (`weights` map)
- Aggregation parameters (`maxPriceAgeSeconds`, `iqrMultiplier`)

## Getting Started

1. Install dependencies:
   ```bash
   go mod download  # Backend dependencies
   cd web/dashboard && npm install  # Frontend dependencies
   ```

2. Start the API server:
   ```bash
   cd api
   go run server.go
   ```

3. Start the web dashboard:
   ```bash
   cd web/dashboard
   npm start
   ```

## API Endpoints

### Get Price
```
GET /api/v1/prices/{symbol}
```
Returns current price information for the specified trading pair.

Response:
```json
{
  "symbol": "BTCUSDT",
  "price": 50000.00,
  "volume": 1000.50,
  "timestamp": "2024-04-13T10:30:00Z"
}
```

### Health Check
```
GET /api/v1/health
```
Returns server health status.

Response:
```json
{
  "status": "ok",
  "timestamp": "2024-04-13T10:30:00Z"
}
```

## Development

- Backend: Go 1.21+
- Frontend: Node.js 18+, React
- Configuration: JSON
- Smart Contracts: Solidity, Hardhat

## License

[Add your license information here] 