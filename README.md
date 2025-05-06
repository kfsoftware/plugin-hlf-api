# Hyperledger Fabric API

This is a REST API service that provides endpoints to interact with a Hyperledger Fabric network using the fabric-gateway library.

## Features

- Invoke transactions on the blockchain
- Evaluate transactions (queries) without writing to the blockchain
- Multi-peer support with random peer selection for load balancing
- CLI-based configuration
- Chi router for efficient HTTP routing
- Proper error handling and JSON responses

## Prerequisites

- Go 1.19 or later
- Access to a Hyperledger Fabric network
- Valid certificates and private keys for authentication

## Installation

```bash
go get github.com/kfsoftware/chainlaunch-plugin-hlf
```

## Usage

### Starting the Server

The server requires several parameters to connect to your Fabric network. You can specify multiple peers for load balancing:

```bash
./plugin-hlf-api serve \
  --port 8080 \
  --mspid "Org1MSP" \
  --cert "/path/to/cert.pem" \
  --key "/path/to/key.pem" \
  --peers "localhost:7051,localhost:8051,localhost:9051" \
  --tlscerts "/path/to/peer1-tls.pem,/path/to/peer2-tls.pem,/path/to/peer3-tls.pem" \
  --channel "mychannel" \
  --chaincode "basic"
```

The API will automatically distribute requests across the configured peers using random selection.

### Configuration Parameters

- `--port`: Port to run the API server (default: 8080)
- `--mspid`: MSP ID of the organization
- `--cert`: Path to the client certificate
- `--key`: Path to the client private key
- `--peers`: Comma-separated list of peer endpoints (host:port)
- `--tlscerts`: Comma-separated list of paths to peer TLS certificates (one per peer)
- `--channel`: Channel name
- `--chaincode`: Chaincode name

Note: The number of peer endpoints must match the number of TLS certificates provided.

### API Endpoints

#### Invoke Transaction

```http
POST /api/invoke
Content-Type: application/json

{
  "chaincode_name": "basic",
  "function": "CreateAsset",
  "args": ["asset1", "blue", "5", "tom", "100"]
}
```

#### Evaluate Transaction (Query)

```http
POST /api/evaluate
Content-Type: application/json

{
  "chaincode_name": "basic",
  "function": "GetAllAssets",
  "args": []
}
```

### Response Format

Success Response:
```json
{
  "status": "success",
  "result": "transaction result here"
}
```

Error Response:
```json
{
  "status": "error",
  "error": "error message here"
}
```

## Load Balancing

The API implements a random peer selection strategy for both invoke and evaluate transactions. This helps distribute the load across all available peers in the network. Each request will be randomly assigned to one of the configured peers.

## Development

To build the project:

```bash
go build -o plugin-hlf-api
```

## License

This project is licensed under the Apache License 2.0. 