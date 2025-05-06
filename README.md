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

## Docker Support

### Building the Docker Image

Build the Docker image using:

```bash
docker build -t fabric-api:latest .
```

### Running with Docker

The API can be run in a Docker container. You'll need to mount your certificates and private keys into the container:

```bash
docker run -d \
  --name fabric-api \
  -p 8080:8080 \
  -v /path/to/certs:/app/crypto \
  fabric-api:latest \
  serve \
  --mspid "Org1MSP" \
  --cert "/app/crypto/cert.pem" \
  --key "/app/crypto/key.pem" \
  --peers "peer1:7051,peer2:7051,peer3:7051" \
  --tlscerts "/app/crypto/peer1-tls.pem,/app/crypto/peer2-tls.pem,/app/crypto/peer3-tls.pem" \
  --channel "mychannel" \
  --chaincode "basic"
```

### Environment Variables

You can also configure the API using environment variables:

```bash
docker run -d \
  --name fabric-api \
  -p 8080:8080 \
  -v /path/to/certs:/app/crypto \
  -e FABRIC_MSPID="Org1MSP" \
  -e FABRIC_CERT_PATH="/app/crypto/cert.pem" \
  -e FABRIC_KEY_PATH="/app/crypto/key.pem" \
  -e FABRIC_PEERS="peer1:7051,peer2:7051,peer3:7051" \
  -e FABRIC_TLS_CERTS="/app/crypto/peer1-tls.pem,/app/crypto/peer2-tls.pem,/app/crypto/peer3-tls.pem" \
  -e FABRIC_CHANNEL="mychannel" \
  -e FABRIC_CHAINCODE="basic" \
  fabric-api:latest
```

### Docker Compose Example

Here's an example docker-compose.yml for running the API:

```yaml
version: '3.8'
services:
  fabric-api:
    build: .
    ports:
      - "8080:8080"
    volumes:
      - ./crypto:/app/crypto
    environment:
      - FABRIC_MSPID=Org1MSP
      - FABRIC_CERT_PATH=/app/crypto/cert.pem
      - FABRIC_KEY_PATH=/app/crypto/key.pem
      - FABRIC_PEERS=peer1:7051,peer2:7051,peer3:7051
      - FABRIC_TLS_CERTS=/app/crypto/peer1-tls.pem,/app/crypto/peer2-tls.pem,/app/crypto/peer3-tls.pem
      - FABRIC_CHANNEL=mychannel
      - FABRIC_CHAINCODE=basic
    networks:
      - fabric-network

networks:
  fabric-network:
    external: true
```

Note: Make sure to adjust the volume mounts and network configuration according to your Fabric network setup.

## License

This project is licensed under the Apache License 2.0. 