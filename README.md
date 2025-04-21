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
│   └── pairs.json        # Price feed definitions (asset pair, chain context, sources, weights, aggregation params)
├── oracle/               # Core Oracle Engine (Go)
│   ├── common/          # Shared types and utilities
│   ├── sources/         # Source fetching implementations & Aggregation logic
│   │   └── crypto/      # Cryptocurrency-specific fetchers, config loader, aggregator
│   └── ...              # Placeholder for potential future data types (e.g., fetching stock prices).
├── web/                 # Optional Frontend Dashboard (React)
│   └── dashboard/
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
├── api/                    # Contains the code for the REST API Server
│   └── server.go          # The main entry point and logic for the API server that handles incoming requests.
├── config/                # **Crucial:** Holds all configuration files that define how the oracle behaves.
│   ├── chains.json       # Defines the blockchain networks the oracle knows about (e.g., Ethereum, Solana). Includes details like network ID and RPC endpoints.
│   ├── assets.json       # Defines the cryptocurrencies and tokens the oracle supports (e.g., ETH, BTC, USDC). Includes symbol, name, decimals.
│   ├── sources.json      # Defines *where* the oracle can get price data from (e.g., Binance API, Coinbase API, a specific Uniswap subgraph). Each source has a unique ID, type, and connection details.
│   └── pairs.json        # **Core Logic Definition:** Defines the actual price feeds (e.g., ETH/USDC for the global market, SOL/USDC on Solana). It links assets, specifies sources from `sources.json`, sets weights, and configures the aggregation method.
├── oracle/               # Contains the core logic for fetching and calculating prices.
│   ├── common/          # Shared Go data structures (like `PricePoint`) and utility functions used across the oracle and API components to avoid code duplication.
│   ├── sources/         # Logic for interacting with specific types of price sources.
│   │   └── crypto/      # Implementation for fetching cryptocurrency prices. This is where fetchers for Binance, Coinbase, Uniswap, etc., reside, along with the main aggregation logic (`aggregator.go`) and configuration loading (`config.go`).
│   └── ...              # Placeholder for potential future data types (e.g., fetching stock prices).
├── web/                 # Optional frontend applications.
│   └── dashboard/       # A sample React-based web dashboard to monitor prices visually. Useful for testing and demonstration.
│       └── ...
├── contracts/           # Optional: Placeholder for smart contracts related to the oracle (e.g., for publishing prices on-chain).
├── cmd/                 # Optional: Placeholder for command-line tools related to the oracle.
├── .env                 # **Important & Sensitive:** Stores environment variables like API keys for exchanges. This file is *not* committed to Git (see `.gitignore`). You'll need to create this file locally.
├── .gitignore           # Tells Git which files or directories to ignore (like `.env`, build artifacts, etc.).
├── go.mod              # Go's module file. Declares the project's module path and its dependencies (the external libraries it uses).
├── go.sum              # Stores the cryptographic checksums of the project's direct and indirect dependencies, ensuring build reproducibility.
└── README.md           # This file: Your guide to the project!
```

## Components Deep Dive

Let's look at the main pieces of the system:

### 1. API Server (`api/`)

*   **Purpose:** Acts as the "front door" for users or applications wanting to get price data. It listens for HTTP requests and provides responses in a standard format (JSON).
*   **Technology:** Built using Go and the popular `gorilla/mux` library for routing incoming requests to the correct handler functions.
*   **Key Features:**
    *   **Endpoints:** Provides specific URLs (see API Endpoints section) to request prices (`/prices/...`) or check system health (`/health`).
    *   **Input Validation:** Checks if requests are well-formed (e.g., is the symbol format correct? Is the chain ID valid?).
    *   **Configuration Loading:** Reads the `config/` files and the `.env` file at startup to know which price feeds are available and how to connect to sources.
    *   **Interaction with Oracle Core:** When a price request comes in, the API server asks the Oracle Core component to fetch and calculate the required price.
    *   **Standard Responses:** Returns data and potential errors in a consistent JSON format.
    *   **CORS:** Includes Cross-Origin Resource Sharing headers, allowing web browsers from different domains to interact with the API.
    *   **Logging:** Uses Go's standard `log/slog` for structured logging, which helps in debugging and monitoring.

### 2. Configuration (`config/`)

*   **Purpose:** Defines *everything* about the oracle's behavior: which assets and chains are supported, which data sources to use, how to combine data from those sources for specific pairs, and parameters for the aggregation logic.
*   **Format:** Uses simple JSON files, making it human-readable and easy to modify without changing Go code.
*   **Relational Model:** The files are designed to work together:
    *   `pairs.json` references assets from `assets.json`.
    *   `pairs.json` references sources from `sources.json`.
    *   `sources.json` (for DEXs) references chains from `chains.json`.
    *   This structure makes the configuration modular and avoids repetition.

### 3. Oracle Core (`oracle/`)

*   **Purpose:** This is the "engine" of the system. It contains the logic responsible for actually fetching data from external sources (like exchange APIs) and performing the complex aggregation process to arrive at a single, reliable price point.
*   **Key Parts:**
    *   **Fetchers (`oracle/sources/crypto/`):** Specific functions tailored to get data from different source types (e.g., one function for Binance REST API, another for a GraphQL Subgraph). New sources require new fetchers here.
    *   **Configuration Loading (`oracle/sources/crypto/config.go`):** Reads, validates, and processes the JSON configuration files from the `config/` directory, creating an in-memory representation the aggregator can use.
    *   **Aggregator (`oracle/sources/crypto/aggregator.go`):** Orchestrates the process for a given price feed (like `ETHUSDC_Global`):
        1.  Looks up the feed's configuration (sources, weights, parameters).
        2.  Calls the relevant fetcher functions concurrently to get data from all sources.
        3.  Applies the **Robust Aggregation Pipeline** (see below).
        4.  Caches the results and the raw source data (for the `/sources` API endpoint).
*   **Robust Aggregation Pipeline:** This is the core algorithm used to ensure price quality:
    1.  **Concurrent Fetching:** Gets data from sources simultaneously for speed.
    2.  **Staleness Filtering:** Ignores price points that are too old (older than `maxPriceAgeSeconds` defined in `pairs.json`).
    3.  **Outlier Rejection (IQR):** Removes statistically anomalous prices that deviate too much from the median, using the Interquartile Range (IQR) method (configurable via `iqrMultiplier` in `pairs.json`). This prevents a single faulty source from skewing the result.
    4.  **Volume-Enhanced Weighted Median:** Calculates the final price. It considers the pre-configured `weights` from `pairs.json` but also boosts the influence of sources reporting higher trading volume, assuming higher volume indicates higher confidence. It uses a weighted median approach for robustness against remaining outliers.
    5.  **Volume Aggregation:** Calculates the total volume from all valid sources that contributed to the final price.

### 4. Web Dashboard (`web/dashboard/`) (Optional)

*   **Purpose:** Provides a simple, visual interface to monitor the prices being generated by the oracle's API. Useful for quick checks and demonstrations.
*   **Technology:** Built with React and Material UI.
*   **Functionality:** Fetches data from the API server and displays it in a table. Allows viewing the raw source data behind an aggregated price.

## How It Works: Request Flow

Here's a simplified view of what happens when you request a price:

1.  **Request:** Your application (or `curl`, or the web dashboard) sends an HTTP GET request to the API Server, e.g., `http://localhost:8080/api/v1/prices/ETH_USDC?chain=global`.
2.  **API Server Receives:** The Go application (`api/server.go`) receives the request.
3.  **Input Parsing & Validation:** The server extracts the symbol (`ETH_USDC`) and chain (`global`). It checks if the format is valid.
4.  **Feed Identification:** The server determines the required internal Feed ID (e.g., `ETHUSDC_Global`).
5.  **API Asks Oracle:** The API server calls the relevant function within the Oracle Core (`oracle/sources/crypto/aggregator.go`), asking for the latest price of `ETHUSDC_Global`.
6.  **Oracle Loads Config:** The Oracle Core ensures it has the latest configuration loaded from the `config/*.json` files. It finds the entry for `ETHUSDC_Global` in `pairs.json`.
7.  **Oracle Fetches Data:** Based on the configuration, the Oracle Core identifies the necessary sources (e.g., `binance_cex`, `kraken_cex`, `the_graph_eth_bundle` from `sources.json`). It calls the specific fetcher functions to get current price and volume data from each source concurrently. This might involve making HTTP requests to Binance's API, Kraken's API, and The Graph's API (using API keys from the `.env` file if needed).
8.  **Oracle Aggregates:** The Oracle Core takes all the fetched price points and applies the Robust Aggregation Pipeline (staleness filtering, outlier rejection, volume-weighted median calculation).
9.  **Oracle Returns Price:** The Oracle Core returns the final calculated price (e.g., 1646.96), total volume, and timestamp back to the API Server.
10. **API Server Responds:** The API server formats this information into the standard JSON response structure.
11. **Response Sent:** The API Server sends the JSON response back to your application.

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

## Configuration System In Detail

The configuration lives in the `/config` directory and uses JSON files. This separation makes it easy to manage and update oracle behavior without touching the Go code.

1.  **`chains.json`**: Defines blockchain networks.
    *   **Purpose:** Lists the blockchains the oracle might interact with (especially for DEX sources).
    *   **Structure:** A map where keys are unique chain identifiers (e.g., `"ethereum"`, `"solana"`) and values are objects containing chain details.
    *   **Example Key Fields:** `"id"` (e.g., `"1"` for Ethereum), `"name"`, `"rpcUrl"` (for potential direct node interaction).
    ```json
    // config/chains.json
    {
      "ethereum": {
        "id": "1",
        "name": "Ethereum",
        "rpcUrl": "https://mainnet.infura.io/v3/YOUR_INFURA_KEY"
      },
      "solana": {
        "id": "solana",
        "name": "Solana",
        "rpcUrl": "https://api.mainnet-beta.solana.com"
      }
      // ... other chains
    }
    ```

2.  **`assets.json`**: Defines crypto assets.
    *   **Purpose:** Lists the tokens and cryptocurrencies the oracle understands.
    *   **Structure:** A map where keys are asset symbols (e.g., `"ETH"`, `"USDC"`) and values are objects containing asset details.
    *   **Example Key Fields:** `"symbol"`, `"name"`, `"decimals"` (important for correct price representation).
    ```json
    // config/assets.json
    {
      "ETH": {
        "symbol": "ETH",
        "name": "Ethereum",
        "decimals": 18
      },
      "USDC": {
        "symbol": "USDC",
        "name": "USD Coin",
        "decimals": 6
      }
      // ... other assets
    }
    ```

3.  **`sources.json`**: Defines individual data sources.
    *   **Purpose:** Lists every specific place the oracle can get price data from. This could be a CEX API, a DEX subgraph, or a direct RPC call.
    *   **Structure:** A map where keys are *unique source instance IDs* (e.g., `"binance_cex"`, `"uniswap_v3_eth_mainnet"`) and values define the source.
    *   **Example Key Fields:**
        *   `"name"`: Human-readable name (e.g., "Binance CEX API").
        *   `"type"`: The kind of source (e.g., `"cex"`, `"dex_subgraph"`, `"dex_rpc"`). The Oracle Core uses this to know which fetcher function to use.
        *   `"apiUrl"`: The base URL for the API or subgraph.
        *   `"apiKeyEnvVar"`: (Optional) The name of the environment variable in `.env` holding the API key for this source (e.g., `"BINANCE_API_KEY"`).
        *   `"chainId"`: **Required for DEX sources.** Links to an ID in `chains.json` (e.g., `"ethereum"`).
        *   `"address"`: (Optional) E.g., the specific pool address for a DEX source.
        *   `"cexSymbolFormat"`: (Optional) How to format the pair for this CEX (e.g., `BASEQUOTE` -> `ETHUSDC`, `BASE-QUOTE` -> `ETH-USDC`).
    ```json
    // config/sources.json
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
        "cexSymbolFormat": "BASEQUOTE" // Might differ per CEX
      },
      "the_graph_eth_bundle": {
          "name": "The Graph ETH Bundle",
          "type": "the_graph_eth_bundle",
          "chainId": "ethereum", // Links to chains.json
          "apiUrl": "https://api.thegraph.com/subgraphs/name/uniswap/uniswap-v3", // Example
          "apiKeyEnvVar": "THE_GRAPH_API_KEY"
      }
      // ... other sources like specific DEX pools via subgraph/RPC
    }
    ```

4.  **`pairs.json`**: Defines the actual price feeds. **This ties everything together.**
    *   **Purpose:** Defines each specific price feed the oracle provides (e.g., a global ETH/USDC price, or a SOL/USDC price specifically from Solana sources).
    *   **Structure:** A map where keys are *unique Feed IDs* (e.g., `"ETHUSDC_Global"`, `"SOLUSDC_Solana"`). These IDs are constructed internally by the API/Oracle based on the requested symbol and chain. Values define the feed's configuration.
    *   **Example Key Fields:**
        *   `"baseAsset"`, `"quoteAsset"`: Symbols linking to `assets.json` (e.g., `"ETH"`, `"USDC"`).
        *   `"chainId"`: The context for this feed. `"global"` means aggregate across all relevant sources regardless of chain. A specific chain ID (e.g., `"solana"`, linking to `chains.json`) means only use sources associated with that chain.
        *   `"aggregation"`: An object containing parameters for the aggregation pipeline:
            *   `"minimumSources"`: Minimum valid sources needed after filtering to produce a price.
            *   `"maxPriceAgeSeconds"`: Maximum age (in seconds) for a price point to be considered valid.
            *   `"iqrMultiplier"`: The 'k' factor used in IQR outlier rejection. Higher values are more tolerant of deviations.
        *   `"sources"`: A list of *source instance IDs* (linking to `sources.json`) to use for *this specific feed*.
        *   `"weights"`: A map assigning a static weight to each source listed in `"sources"`. Weights should ideally sum to 1.0 but are normalized during calculation. Sources not listed here get a default weight (or zero).
    ```json
    // config/pairs.json
    {
      "ETHUSDC_Global": {
        "baseAsset": "ETH",
        "quoteAsset": "USDC",
        "chainId": "global", // Aggregate across multiple chains/sources
        "aggregation": {
          "minimumSources": 3,
          "maxPriceAgeSeconds": 60,
          "iqrMultiplier": 1.5
        },
        "sources": [ // Which sources from sources.json to use
          "binance_cex",
          "kraken_cex",
          "coinbase_cex", // Assumed defined in sources.json
          "the_graph_eth_bundle"
        ],
        "weights": { // How much influence each source has (before volume boost)
          "binance_cex": 0.4,
          "kraken_cex": 0.3,
          "coinbase_cex": 0.2,
          "the_graph_eth_bundle": 0.1
        }
      },
      "SOLUSDC_Solana": {
        "baseAsset": "SOL", // Assumed defined in assets.json
        "quoteAsset": "USDC",
        "chainId": "solana", // Only use sources linked to Solana
        "aggregation": {
          "minimumSources": 1,
          "maxPriceAgeSeconds": 90,
          "iqrMultiplier": 2.0
        },
        "sources": [
          "raydium_solana" // Assumed defined in sources.json with chainId: "solana"
          // Maybe others like Jupiter RPC etc.
        ],
        "weights": {
          "raydium_solana": 1.0
        }
      }
      // ... other feeds
    }
    ```

## Getting Started: Step-by-Step

Ready to run the oracle? Follow these steps:

1.  **Install Go:**
    *   **Why?** The backend API server and oracle core are written in Go.
    *   **How?** Download and install Go (version 1.21 or later) from the official Go website: [https://go.dev/doc/install](https://go.dev/doc/install). Follow the instructions for your operating system. Verify the installation by opening a terminal and typing `go version`.

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
    "volume": 0, // Some sources might not report volume
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
    "source": "faulty_source_example", // Fictional
    "price": 1805.10,
    "volume": 10.0,
    "timestamp": "2025-04-21T07:35:44.100Z",
    "status": "outlier" // Was rejected by IQR
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

*   **Backend:** Go (1.21+)
*   **Frontend (Optional Dashboard):** Node.js (18+), React, Material UI
*   **Configuration:** JSON
*   **API Keys/Secrets:** `.env` file (requires `godotenv` Go package)
*   **Dependencies:** Managed by Go Modules (`go.mod`, `go.sum`) and NPM (`package.json`, `package-lock.json` for frontend).

## Contributing

(Optional: Add guidelines here if you want others to contribute - e.g., fork the repo, create a branch, submit a pull request, coding standards, issue reporting).

## License

[Add your license information here - e.g., MIT, Apache 2.0] 