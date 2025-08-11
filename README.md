# Hyperledger Fabric API Plugin

A dynamic API server for Hyperledger Fabric networks that automatically generates REST endpoints for chaincode functions.

## Features

- **Dynamic API Generation**: Automatically creates REST endpoints for chaincode functions
- **Swagger Documentation**: Interactive API documentation with operation IDs
- **Chaincode Playground**: Interactive testing environment for chaincode functions
- **Metrics & Monitoring**: Built-in Prometheus metrics collection
- **TLS Support**: Secure communication with Fabric network
- **Contract Support**: Handles both default and named contracts

## Quick Start

### Prerequisites

- Go 1.23.4 or later
- Hyperledger Fabric network running
- Client certificates and private keys
- Chaincode installed and instantiated

### Installation

```bash
git clone <repository>
cd plugin-hlf-api
go mod download
go build -o hlf-plugin-api .
```

### Configuration

Set required environment variables:

```bash
export FABRIC_MSPID="Org1MSP"
export FABRIC_CERT_PATH="/path/to/client.crt"
export FABRIC_KEY_PATH="/path/to/client.key"
export FABRIC_PEERS="peer0.org1.example.com:7051,peer1.org1.example.com:7051"
export FABRIC_TLS_CERTS="/path/to/peer0.crt,/path/to/peer1.crt"
export FABRIC_CHANNEL="mychannel"
export PORT_API="8180"
```

### Running the Server

```bash
./hlf-plugin-api serve \
  --mspid Org1MSP \
  --cert /path/to/client.crt \
  --key /path/to/client.key \
  --peers peer0.org1.example.com:7051,peer1.org1.example.com:7051 \
  --tlscerts /path/to/peer0.crt,/path/to/peer1.crt \
  --channel mychannel \
  --chaincodes basic,asset \
  --port 8180
```

## API Endpoints

### Core Endpoints

- `POST /api/invoke` - Invoke chaincode transaction
- `POST /api/evaluate` - Evaluate chaincode function (read-only)
- `GET /api/chaincodes` - List available chaincodes

### Dynamic Chaincode Endpoints

For each chaincode, the server automatically generates:

- `POST /api/chaincodes/{chaincode}/invoke` - Generic invoke
- `POST /api/chaincodes/{chaincode}/evaluate` - Generic evaluate
- `POST /api/chaincodes/{chaincode}/{contract}/{function}` - Specific function calls

### Swagger Documentation

- `/swagger/` - Interactive API documentation
- `/swagger/index.json` - List of available chaincodes
- `/swagger/{chaincode}.json` - Chaincode-specific API spec
- `/swagger/index.html` - Traditional Swagger interface

### Debug Endpoints

- `/debug/chaincodes` - Check chaincode configuration
- `/debug/embedded` - Verify embedded HTML files

## 🎮 Chaincode Playground

The Chaincode Playground provides an interactive testing environment for your chaincode functions.

### Access the Playground

The playground is now embedded in the binary and accessible at multiple endpoints:

- **Root URL**: `http://localhost:8180/` (redirects to playground)
- **Playground**: `http://localhost:8180/playground`
- **Swagger Index**: `http://localhost:8180/swagger/index.html`

### Features

- **Chaincode Selection**: Dropdown to select from available chaincodes
- **Interactive Testing**: Use Swagger UI's "Try it out" feature
- **Real-time Status**: Monitor operation status and results
- **Metadata Viewing**: Explore chaincode contracts and functions
- **Request Logging**: Debug API calls with console logging

### How to Use

1. **Select a Chaincode**: Choose from the dropdown menu
2. **Browse Functions**: View available endpoints in the Swagger UI
3. **Test Functions**: Use "Try it out" to execute chaincode functions
4. **Monitor Results**: Check status updates and response logs

### Testing Modes

- **Evaluate Mode** (`mode=evaluate`): Read-only operations, no blockchain changes
- **Submit Mode** (`mode=submit`): Write operations, creates blockchain transactions

### Example Usage

```bash
# Start the server with chaincodes
./hlf-plugin-api serve --chaincodes basic,asset ...

# Access the playground
open http://localhost:8180/playground

# Select a chaincode and test functions interactively
```

## Operation IDs

Each endpoint has a unique operation ID following camelCase convention:

- **Contract functions**: `{chaincode}{contract}{function}` (e.g., `basicAssetGetAllAssets`)
- **Generic operations**: `{chaincode}Invoke`, `{chaincode}Evaluate`
- **API handlers**: `invokeChaincode`, `evaluateChaincode`, `getChaincodes`

## Metrics

The server exposes Prometheus metrics at `/metrics`:

- HTTP request metrics
- Chaincode operation metrics
- Transaction success/failure rates
- Response time histograms

## Development

### Embedded Assets

The server now embeds all necessary HTML files into the binary:

- **Chaincode Playground**: `docs/custom-swagger.html` - Interactive testing environment
- **Swagger Index**: `docs/swagger-index.html` - Traditional Swagger interface

This means:
- ✅ **No external files needed** - Everything is self-contained
- ✅ **Easier deployment** - Single binary with all assets
- ✅ **Better portability** - Works in any environment
- ✅ **Version consistency** - HTML always matches the binary version

### Project Structure

```
.
├── main.go                 # Main server and routing
├── pkg/
│   ├── api/               # HTTP handlers
│   ├── fabric/            # Fabric client
│   └── metrics/           # Metrics collection
├── docs/                  # Swagger documentation
└── README.md
```

### Building

```bash
go build -o hlf-plugin-api .
```

### Testing

```bash
go test ./...
```

## License

[Add your license information here] 