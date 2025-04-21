# Cryptocurrency Price Oracle

This system provides a robust and modular cryptocurrency price oracle, aggregating data from multiple sources (CEX, DEX) to generate reliable global and chain-specific price feeds via a REST API.

## Features

*   **Multi-Source Aggregation:** Fetches price data concurrently from various sources (e.g., Binance, Kraken, Coinbase, The Graph Subgraphs).
*   **Configurable Feeds:** Define price feeds (`BASE_QUOTE`) for global or specific blockchain contexts (`chains.json`, `assets.json`, `sources.json`, `pairs.json`).
*   **Robust Aggregation Pipeline:** Employs a multi-stage process including staleness filtering, Interquartile Range (IQR) outlier rejection, and a volume-enhanced weighted median calculation.
*   **REST API:** Exposes endpoints for retrieving aggregated prices, raw source data, and health status.
*   **Environment-Based Configuration:** Utilizes `.env` files for secure management of API keys.
*   **Modular Design:** Separates API, configuration loading, and core oracle logic.
*   **Optional Frontend:** Includes a basic React dashboard for monitoring.

## Architecture Overview

```
.
├── api/                    # REST API Server (Go, gorilla/mux)
│   └── server.go
├── config/                # JSON Configuration Files
│   ├── chains.json       # Blockchain definitions
│   ├── assets.json       # Asset definitions
│   ├── sources.json      # Price source instances (CEX/DEX definitions, API endpoints, keys)
│   ├── pairs.json        # Price feed definitions (asset pair, chain context, sources, weights, aggregation params)
│   ├── assets/           # Additional asset configurations
│   ├── base/             # Base configurations
│   └── pairs/            # Additional pair configurations
├── oracle/               # Core Oracle Engine (Go)
│   ├── aggregator/       # Aggregation logic and pipeline
│   ├── common/          # Shared types and utilities
│   ├── sources/         # Source fetching implementations
│   │   └── crypto/      # Cryptocurrency-specific fetchers, config loader
├── web/                 # Optional Frontend Applications
│   ├── dashboard/       # React dashboard for monitoring 
│   └── frontend/        # Additional frontend components
├── contracts/           # Optional: Smart Contract interfaces/implementations
├── cmd/                 # Optional: Command-line utilities
├── .env                 # Environment variables (API Keys - Gitignored)
├── go.mod / go.sum      # Go module definitions & checksums
└── README.md
```

*   **API Server (`api/`):** Handles incoming HTTP requests, validates input, interacts with the Oracle Engine, and returns JSON responses. Loads configuration and API keys on startup.
*   **Configuration (`config/`):** Defines system behavior through interrelated JSON files. See Configuration section for details.
*   **Oracle Engine (`oracle/`):** Contains the core logic for fetching data from configured sources and executing the aggregation algorithm. Manages source-specific interactions and calculations.
*   **Web Dashboard (`web/`):** An optional React application for visualizing price data from the API.

## Configuration

The system utilizes a set of JSON files within the `/config` directory to define its operational parameters:

1.  **`chains.json`**: Defines supported blockchain networks, including their identifiers, names, and potentially RPC endpoints.
    ```json
    { "ethereum": { "id": "1", "name": "Ethereum", ... }, ... }
    ```
2.  **`assets.json`**: Defines supported cryptographic assets, including symbols, names, and decimal precision.
    ```json
    { "ETH": { "symbol": "ETH", "name": "Ethereum", "decimals": 18 }, ... }
    ```
3.  **`sources.json`**: Defines individual data source instances. Each source has a unique ID (e.g., `"binance_cex"`) and specifies its type (`cex`, `dex_subgraph`, etc.), connection details (`apiUrl`), optional API key environment variable (`apiKeyEnvVar`), and, for DEX sources, the relevant `chainId` from `chains.json`.
    ```json
    {
      "binance_cex": {
        "name": "Binance CEX",
        "type": "cex",
        "apiUrl": "https://api.binance.com",
        "cexSymbolFormat": "BASEQUOTE"
      },
      "kraken_cex": {
        "name": "Kraken CEX",
        "type": "cex",
        "apiUrl": "https://api.kraken.com",
        "cexSymbolFormat": "BASEQUOTE"
      },
      "the_graph_eth_bundle": {
          "name": "The Graph ETH Bundle",
          "type": "the_graph_eth_bundle",
          "chainId": "ethereum",
          "apiUrl": "https://api.thegraph.com/subgraphs/name/uniswap/uniswap-v3",
          "apiKeyEnvVar": "THE_GRAPH_API_KEY"
      }
    }
    ```

4.  **`pairs.json`**: Defines the actual price feeds. **This ties everything together.**
    ```json
    {
      "ETHUSDC_Global": {
        "baseAsset": "ETH",
        "quoteAsset": "USDC",
        "chainId": "global",
        "aggregation": {
          "minimumSources": 3,
          "maxPriceAgeSeconds": 60,
          "iqrMultiplier": 1.5
        },
        "sources": [
          "binance_cex",
          "kraken_cex",
          "coinbase_cex",
          "the_graph_eth_bundle"
        ],
        "weights": {
          "binance_cex": 0.4,
          "kraken_cex": 0.3,
          "coinbase_cex": 0.2,
          "the_graph_eth_bundle": 0.1
        }
      },
      "SOLUSDC_Solana": {
        "baseAsset": "SOL",
        "quoteAsset": "USDC",
        "chainId": "solana",
        "aggregation": {
          "minimumSources": 1,
          "maxPriceAgeSeconds": 90,
          "iqrMultiplier": 2.0
        },
        "sources": [
          "raydium_solana"
        ],
        "weights": {
          "raydium_solana": 1.0
        }
      }
    }
    ```

## Getting Started: Step-by-Step

Ready to run the oracle? Follow these steps:

1.  **Install Go:**
    *   **Why?** The backend API server and oracle core are written in Go.
    *   **How?** Download and install Go (version 1.19 or later) from the official Go website: [https://go.dev/doc/install](https://go.dev/doc/install). Follow the instructions for your operating system. Verify the installation by opening a terminal and typing `go version`.

2.  **Install Node.js (Optional):**
    *   **Why?** Needed only if you want to run the optional web dashboard frontend.
    *   **How?** Download and install Node.js (version 18 or later) from the official Node.js website: [https://nodejs.org/](https://nodejs.org/). Verify installation with `node -v` and `npm -v`.

3.  **Clone the Repository:**
    *   **Why?** To get a copy of the project code onto your local machine.
    *   **How?** Open your terminal, navigate to where you want to store the project, and run:
```bash
git clone <repository_url> # Replace <repository_url> with the actual URL
cd <repository_directory> # Navigate into the newly cloned project folder
```

4.  **Configure Environment Variables (`.env`):**
    *   **Why?** The oracle needs API keys to access data from private APIs or rate-limited sources (like exchanges or The Graph). Storing these keys directly in code or configuration is insecure. The `.env` file keeps them separate and private.
    *   **How?**
        *   Look for a `.env.example` file in the project root. If it exists, copy it to a new file named `.env`:
            ```bash
            cp .env.example .env
            ```
        *   If `.env.example` doesn't exist, create an empty file named `.env`.
        *   Open the `.env` file in a text editor.
        *   Add the required API keys, one per line, based on the sources you've configured in `config/sources.json` that specify an `"apiKeyEnvVar"`. For example:
            ```dotenv
            # .env file content
            THE_GRAPH_API_KEY=your_actual_the_graph_api_key
            BINANCE_API_KEY=your_actual_binance_key
            BINANCE_SECRET_KEY=your_actual_binance_secret
            # Add other keys as needed
            ```
        *   **Important:** The `.gitignore` file should already list `.env`. Double-check this to ensure you don't accidentally commit your secret keys to version control.

5.  **Install Go Dependencies:**
    *   **Why?** The Go backend uses external libraries (defined in `go.mod`). This command downloads and installs them.
    *   **How?** Run this command in the project's root directory:
        ```bash
        go mod download
        ```
        You should see Go downloading the necessary packages.

6.  **Install Frontend Dependencies (Optional):**
    *   **Why?** If you want to run the web dashboard, you need to install its Node.js library dependencies.
    *   **How?** Navigate to the dashboard directory and use `npm`:
```bash
cd web/dashboard
npm install
cd ../.. # Go back to the project root directory
```

7.  **Review Configuration:**
    *   **Why?** Before running, it's wise to look at the default settings in the `config/` directory (`chains.json`, `assets.json`, `sources.json`, `pairs.json`) to understand which feeds are configured and how.
    *   **How?** Open the JSON files in your editor and review the settings. You might want to add/remove sources or pairs, or adjust aggregation parameters based on your needs.

8.  **Run the API Server:**
    *   **Why?** This starts the backend process that listens for requests and serves price data.
    *   **How?** Run this command from the project's root directory:
        ```bash
        go run api/server.go
        ```
        You should see log output indicating the server has started, typically listening on port 8080. The server will automatically load the configuration and the `.env` file. Keep this terminal window open while the server is running.

9.  **Run the Dashboard (Optional):**
    *   **Why?** To visually monitor the prices served by the API.
    *   **How?** Open a *new* terminal window, navigate to the dashboard directory, and run:
```bash
cd web/dashboard
npm start
```
This usually opens the dashboard in your web browser automatically (often at `http://localhost:3000`).

You should now have the oracle backend running and potentially the frontend dashboard displaying prices fetched from the backend API!

## API Endpoints Explained

The API provides a simple way to interact with the oracle over HTTP.

### Get Aggregated Price

Retrieves the latest calculated price for a specific feed.

```
GET /api/v1/prices/{symbol}?chain={chainID}
```

*   **Method:** `GET`
*   **Path Parameters:**
    *   `{symbol}`: The trading pair symbol using an underscore separator (e.g., `ETH_USDC`, `BTC_USDT`). **Required.**
*   **Query Parameters:**
    *   `chain`: Specifies the context for the price feed. **Required.**
        *   Use `global` to request the globally aggregated price (combining sources across different chains as configured in `pairs.json`).
        *   Use a specific chain ID (e.g., `solana`, `ethereum`, matching an ID in `chains.json`) to request a price feed specific to that chain (using only sources associated with that chain in `pairs.json`).
*   **Success Response (200 OK):** Returns a JSON object with the aggregated price details.
    *   `feedID`: The unique internal ID of the feed requested (e.g., `ETHUSDC_Global`).
    *   `symbol`: The requested trading pair symbol.
    *   `chain`: The requested chain context (`global` or specific chain ID).
    *   `price`: The final aggregated price (as a floating-point number).
    *   `volume`: The total aggregated volume from valid sources (as a floating-point number).
    *   `source`: Indicates how the price was derived (e.g., `"aggregated_vol_weighted_median"`).
    *   `timestamp`: The time the aggregation was performed (ISO 8601 format, UTC).
*   **Error Responses:**
    *   `400 Bad Request`: Invalid input format (e.g., malformed symbol `ETHUSDC`, missing `chain` query parameter). JSON body contains `{"error":{"code":"INVALID_SYMBOL", "message":"..."}}` or similar.
    *   `404 Not Found`: The requested feed (combination of symbol and chain) is not configured in `config/pairs.json`. JSON body contains `{"error":{"code":"PAIR_NOT_CONFIGURED", "message":"..."}}`.
    *   `500 Internal Server Error`: An issue occurred during fetching or aggregation (e.g., failed to contact sources, not enough valid sources after filtering). JSON body contains `{"error":{"code":"PRICE_FETCH_FAILED", "message":"..."}}`.

**Example Request (Global ETH/USDC):**
```bash
curl http://localhost:8080/api/v1/prices/ETH_USDC?chain=global
```

**Example Success Response:**
```json
{
  "feedID": "ETHUSDC_Global",
  "symbol": "ETH_USDC",
  "chain": "global",
  "price": 1646.96,
  "volume": 356119.22,
  "source": "aggregated_vol_weighted_median",
  "timestamp": "2025-04-21T07:35:44.211Z"
}
```

**Example Request (Solana SOL/USDC):**
```bash
curl http://localhost:8080/api/v1/prices/SOL_USDC?chain=solana
```

### Get Source Details

Retrieves the raw data points fetched from individual sources during the *last* successful aggregation for a specific feed. Useful for debugging and understanding which sources contributed and their status.

```
GET /api/v1/prices/{symbol}/sources?chain={chainID}
```

*   **Method:** `GET`
*   **Path Parameters:** Same as Get Price (`{symbol}`). **Required.**
*   **Query Parameters:** Same as Get Price (`chain`). **Required.**
*   **Success Response (200 OK):** Returns a JSON array where each object represents a price point from a single source.
    *   `source`: The unique ID of the source (from `sources.json`, e.g., `kraken_cex`).
    *   `price`: The raw price reported by this source.
    *   `volume`: The raw volume reported by this source.
    *   `timestamp`: The time this specific data point was fetched or reported (ISO 8601 format, UTC).
    *   `status`: The status of this data point during the last aggregation:
        *   `"valid"`: Used in the final calculation.
        *   `"stale"`: Ignored because it was too old (`maxPriceAgeSeconds`).
        *   `"outlier"`: Ignored because it failed the IQR check.
        *   `"fetch_error"`: Could not be fetched successfully.
*   **Error Responses:** Similar to Get Price (400, 404, 500). If the main price aggregation failed, this endpoint might also return an error or empty data.

**Example Request (Sources for Global ETH/USDC):**
```bash
curl http://localhost:8080/api/v1/prices/ETH_USDC/sources?chain=global
```

**Example Success Response:**
```json
[
  {
    "source": "kraken_cex",
    "price": 1646.07,
    "volume": 504.20,
    "timestamp": "2025-04-21T07:35:43.988Z",
    "status": "valid"
  },
  {
    "source": "the_graph_eth_bundle",
    "price": 1646.68,
    "volume": 0,
    "timestamp": "2025-04-21T07:35:44.071Z",
    "status": "valid"
  },
  {
    "source": "binance_cex",
    "price": 1646.96,
    "volume": 355615.01,
    "timestamp": "2025-04-21T07:35:44.010Z",
    "status": "valid"
  },
  {
    "source": "faulty_source_example",
    "price": 1805.10,
    "volume": 10.0,
    "timestamp": "2025-04-21T07:35:44.100Z",
    "status": "outlier"
  }
]
```

### Health Check

A simple endpoint to check if the API server is running and responsive.

```
GET /api/v1/health
```

*   **Method:** `GET`
*   **Success Response (200 OK):**
    *   `status`: Should be `"ok"`.
    *   `timestamp`: Current server time (ISO 8601 format, UTC).

**Example Request:**
```bash
curl http://localhost:8080/api/v1/health
```

**Example Success Response:**
```json
{
  "status": "ok",
  "timestamp": "2025-04-21T08:00:00.000Z"
}
```

## Development Stack

*   **Backend:** Go (1.19+)
*   **Frontend (Optional Dashboard):** Node.js (18+), React, Material UI
*   **Configuration:** JSON
*   **API Keys/Secrets:** `.env` file (requires `godotenv` Go package)
*   **Dependencies:** Managed by Go Modules (`go.mod`, `go.sum`) and NPM (`package.json`, `package-lock.json` for frontend).

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

This project is licensed under the MIT License - see the LICENSE file for details.