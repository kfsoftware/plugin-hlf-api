package fabric

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"time"

	"github.com/hyperledger/fabric-gateway/pkg/client"
	"github.com/hyperledger/fabric-gateway/pkg/identity"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

// PeerConfig holds the configuration for a single peer
type PeerConfig struct {
	Endpoint    string
	TLSCertPath string
}

// ClientConfig holds the configuration for connecting to Fabric
type ClientConfig struct {
	MspID         string
	CertPath      string
	KeyPath       string
	Peers         []PeerConfig
	ChannelName   string
	ChaincodeName string
}

// TransactionResult represents the result of a transaction
type TransactionResult struct {
	Result      []byte
	TxID        string
	Success     bool
	BlockNumber uint64
	ResultCode  uint32
}

// FabricClient represents a connection to the Fabric network
type FabricClient struct {
	config *ClientConfig
	rand   *rand.Rand
}

func ParseX509Certificate(contents []byte) (*x509.Certificate, error) {
	if len(contents) == 0 {
		return nil, errors.New("certificate pem is empty")
	}
	block, _ := pem.Decode(contents)
	if block == nil {
		return nil, errors.New("failed to decode PEM block")
	}
	crt, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, err
	}
	return crt, nil
}

// NewFabricClient creates a new Fabric client instance
func NewFabricClient(config *ClientConfig) (*FabricClient, error) {
	if len(config.Peers) == 0 {
		return nil, fmt.Errorf("at least one peer must be configured")
	}

	// Initialize random number generator with current time
	source := rand.NewSource(time.Now().UnixNano())
	random := rand.New(source)

	return &FabricClient{
		config: config,
		rand:   random,
	}, nil
}

// selectRandomPeer returns a random peer connection from the available peers
func (fc *FabricClient) selectRandomPeer() (*grpc.ClientConn, error) {
	// Select a random peer configuration
	peerConfig := fc.config.Peers[fc.rand.Intn(len(fc.config.Peers))]

	// Load TLS certificate for the peer
	tlsCert, err := os.ReadFile(peerConfig.TLSCertPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read TLS cert file for peer %s: %w", peerConfig.Endpoint, err)
	}

	certPool := x509.NewCertPool()
	certPool.AppendCertsFromPEM(tlsCert)
	transportCreds := credentials.NewClientTLSFromCert(certPool, "")

	// Create gRPC connection
	conn, err := grpc.Dial(peerConfig.Endpoint, grpc.WithTransportCredentials(transportCreds))
	if err != nil {
		return nil, fmt.Errorf("failed to create gRPC connection to peer %s: %w", peerConfig.Endpoint, err)
	}

	return conn, nil
}

// createGatewayConnection creates a new gateway connection for a specific peer
func (fc *FabricClient) createGatewayConnection(conn *grpc.ClientConn) (*client.Gateway, error) {
	certPem, err := os.ReadFile(fc.config.CertPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read certificate file: %w", err)
	}

	cert, err := ParseX509Certificate(certPem)
	if err != nil {
		return nil, fmt.Errorf("failed to parse certificate for the peer: %w", err)
	}

	id, err := identity.NewX509Identity(fc.config.MspID, cert)
	if err != nil {
		return nil, fmt.Errorf("failed to create identity: %w", err)
	}
	keyPem, err := os.ReadFile(fc.config.KeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read private key file: %w", err)
	}
	pk, err := identity.PrivateKeyFromPEM(keyPem)
	if err != nil {
		return nil, fmt.Errorf("failed to create private key: %w", err)
	}

	signer, err := identity.NewPrivateKeySign(pk)
	if err != nil {
		return nil, fmt.Errorf("failed to create signer: %w", err)
	}

	return client.Connect(
		id,
		client.WithSign(signer),
		client.WithClientConnection(conn),
		client.WithEvaluateTimeout(30*time.Second),
		client.WithEndorseTimeout(30*time.Second),
		client.WithSubmitTimeout(30*time.Second),
		client.WithCommitStatusTimeout(30*time.Second),
	)
}

// InvokeTransaction submits a transaction to the ledger
func (fc *FabricClient) InvokeTransaction(ctx context.Context, fcn string, args []string) (*TransactionResult, error) {
	// Select a random peer and create connection
	selectedPeer, err := fc.selectRandomPeer()
	if err != nil {
		return nil, fmt.Errorf("failed to select peer: %w", err)
	}
	defer selectedPeer.Close()

	// Create a new gateway connection
	gw, err := fc.createGatewayConnection(selectedPeer)
	if err != nil {
		selectedPeer.Close()
		return nil, fmt.Errorf("failed to create gateway connection: %w", err)
	}
	defer gw.Close()

	network := gw.GetNetwork(fc.config.ChannelName)
	contract := network.GetContract(fc.config.ChaincodeName)

	result, commit, err := contract.SubmitAsync(fcn, client.WithArguments(args...))
	if err != nil {
		return nil, fmt.Errorf("failed to submit transaction: %w", err)
	}

	status, err := commit.Status()
	if err != nil {
		return nil, fmt.Errorf("failed to get commit status: %w", err)
	}

	return &TransactionResult{
		Result:      result,
		TxID:        commit.TransactionID(),
		BlockNumber: status.BlockNumber,
		ResultCode:  uint32(status.Code.Number()),
		Success:     status.Successful,
	}, nil
}

// EvaluateTransaction evaluates a transaction without submitting to the ledger
func (fc *FabricClient) EvaluateTransaction(ctx context.Context, fcn string, args []string) ([]byte, error) {
	// Select a random peer and create connection
	selectedPeer, err := fc.selectRandomPeer()
	if err != nil {
		return nil, fmt.Errorf("failed to select peer: %w", err)
	}
	defer selectedPeer.Close()
	// Create a new gateway connection
	gw, err := fc.createGatewayConnection(selectedPeer)
	if err != nil {
		selectedPeer.Close()
		return nil, fmt.Errorf("failed to create gateway connection: %w", err)
	}
	defer gw.Close()

	network := gw.GetNetwork(fc.config.ChannelName)
	contract := network.GetContract(fc.config.ChaincodeName)

	result, err := contract.Evaluate(
		fcn,
		client.WithArguments(args...),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to evaluate transaction: %w", err)
	}
	return result, nil
}

// Close closes the client
func (fc *FabricClient) Close() {
	// Nothing to close as connections are created and closed per operation
}
