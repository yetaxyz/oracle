# Oracle System PoC

This is a Proof of Concept (PoC) implementation of a decentralized oracle system that demonstrates the following key features:

1. Multi-source price aggregation
2. Median price calculation with confidence scoring
3. Smart contract integration
4. REST API endpoint
5. Concurrent data fetching

## Components

### 1. Smart Contract (`contracts/ModernOracle.sol`)
- Modern Solidity implementation (^0.8.19)
- Multiple data source support
- Confidence scoring
- Median calculation
- Pausable functionality
- Owner controls

### 2. Price Aggregator (`oracle/aggregator.go`)
- Concurrent price fetching from multiple sources
- Configurable data sources
- Custom parser support for different APIs
- Thread-safe operations
- Median price calculation

### 3. API Server (`api/server.go`)
- RESTful endpoint for price queries
- JSON response format
- Error handling
- Real-time price updates

## Getting Started

### Prerequisites
- Go 1.16+
- Node.js 14+
- Solidity 0.8.19+

### Installation
1. Clone the repository
2. Install dependencies:
   ```bash
   go mod init oracle-poc
   go mod tidy
   npm install @openzeppelin/contracts
   ```

### Running the PoC
1. Start the API server:
   ```bash
   cd api
   go run server.go
   ```

2. Query prices:
   ```bash
   curl "http://localhost:8080/price?symbol=BTCUSDT"
   ```

### Smart Contract Deployment
1. Deploy using Hardhat or Truffle
2. Configure data sources
3. Start submitting price updates

## Architecture

```
┌──────────────┐    ┌──────────────┐    ┌──────────────┐
│  Exchange 1  │    │  Exchange 2  │    │  Exchange N  │
└──────┬───────┘    └──────┬───────┘    └──────┬───────┘
       │                   │                    │
       └───────────┬───────┴──────────┬────────┘
                   │                  │
            ┌──────▼──────┐    ┌──────▼──────┐
            │ Aggregator  │    │   Oracle    │
            │  (Go)      │    │  Contract   │
            └──────┬──────┘    └──────┬──────┘
                   │                  │
            ┌──────▼──────┐    ┌──────▼──────┐
            │ REST API    │    │    DApps    │
            └─────────────┘    └─────────────┘
```

## Future Improvements

1. Add more data sources
2. Implement WebSocket support for real-time updates
3. Add authentication for API endpoints
4. Implement caching layer
5. Add more sophisticated aggregation methods
6. Implement historical price storage
7. Add monitoring and alerting
8. Implement rate limiting
9. Add support for more asset types
10. Implement automated tests

## Security Considerations

- API rate limiting
- Input validation
- Error handling
- Access control
- Data validation
- Smart contract security
- Network security
- Data source reliability

## Contributing

Feel free to submit issues and enhancement requests.

## License

MIT 