package fabric

import (
	"context"
	"crypto/x509"
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

// FabricClient represents a connection to the Fabric network
type FabricClient struct {
	config   *ClientConfig
	contract *client.Contract
	peers    []*grpc.ClientConn
	rand     *rand.Rand
}

// NewFabricClient creates a new Fabric client instance
func NewFabricClient(config *ClientConfig) (*FabricClient, error) {
	if len(config.Peers) == 0 {
		return nil, fmt.Errorf("at least one peer must be configured")
	}

	// Load client identity
	certPEM, err := os.ReadFile(config.CertPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read certificate file: %w", err)
	}

	keyPEM, err := os.ReadFile(config.KeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read private key file: %w", err)
	}

	cert, err := x509.ParseCertificate(certPEM)
	if err != nil {
		return nil, fmt.Errorf("failed to parse certificate: %w", err)
	}

	id, err := identity.NewX509Identity(config.MspID, cert)
	if err != nil {
		return nil, fmt.Errorf("failed to create identity: %w", err)
	}

	pk, err := identity.PrivateKeyFromPEM(keyPEM)
	if err != nil {
		return nil, fmt.Errorf("failed to create private key: %w", err)
	}

	signer, err := identity.NewPrivateKeySign(pk)
	if err != nil {
		return nil, fmt.Errorf("failed to create signer: %w", err)
	}

	// Initialize random number generator with current time
	source := rand.NewSource(time.Now().UnixNano())
	random := rand.New(source)

	// Create connections to all peers
	var peers []*grpc.ClientConn
	for _, peerConfig := range config.Peers {
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
		peers = append(peers, conn)
	}

	// Randomly select initial peer
	selectedPeer := peers[random.Intn(len(peers))]

	// Create Gateway connection with the selected peer
	gw, err := client.Connect(
		id,
		client.WithSign(signer),
		client.WithClientConnection(selectedPeer),
		client.WithEvaluateTimeout(5),
		client.WithEndorseTimeout(15),
		client.WithSubmitTimeout(5),
		client.WithCommitStatusTimeout(1),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create gateway: %w", err)
	}

	network := gw.GetNetwork(config.ChannelName)
	contract := network.GetContract(config.ChaincodeName)

	return &FabricClient{
		config:   config,
		contract: contract,
		peers:    peers,
		rand:     random,
	}, nil
}

// selectRandomPeer returns a random peer connection from the available peers
func (fc *FabricClient) selectRandomPeer() *grpc.ClientConn {
	return fc.peers[fc.rand.Intn(len(fc.peers))]
}

// reconnectWithPeer creates a new gateway connection with the specified peer
func (fc *FabricClient) reconnectWithPeer(conn *grpc.ClientConn) error {
	cert, err := x509.ParseCertificate([]byte(fc.config.CertPath))
	if err != nil {
		return fmt.Errorf("failed to parse certificate: %w", err)
	}

	id, err := identity.NewX509Identity(fc.config.MspID, cert)
	if err != nil {
		return fmt.Errorf("failed to create identity: %w", err)
	}

	pk, err := identity.PrivateKeyFromPEM([]byte(fc.config.KeyPath))
	if err != nil {
		return fmt.Errorf("failed to create private key: %w", err)
	}

	signer, err := identity.NewPrivateKeySign(pk)
	if err != nil {
		return fmt.Errorf("failed to create signer: %w", err)
	}

	gw, err := client.Connect(
		id,
		client.WithSign(signer),
		client.WithClientConnection(conn),
		client.WithEvaluateTimeout(5),
		client.WithEndorseTimeout(15),
		client.WithSubmitTimeout(5),
		client.WithCommitStatusTimeout(1),
	)
	if err != nil {
		return fmt.Errorf("failed to create gateway: %w", err)
	}

	network := gw.GetNetwork(fc.config.ChannelName)
	fc.contract = network.GetContract(fc.config.ChaincodeName)
	return nil
}

// InvokeTransaction submits a transaction to the ledger
func (fc *FabricClient) InvokeTransaction(ctx context.Context, fcn string, args []string) ([]byte, error) {
	// Select a random peer for this transaction
	selectedPeer := fc.selectRandomPeer()
	if err := fc.reconnectWithPeer(selectedPeer); err != nil {
		return nil, fmt.Errorf("failed to connect to peer: %w", err)
	}

	result, err := fc.contract.Submit(
		fcn,
		client.WithArguments(args...),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to submit transaction: %w", err)
	}
	return result, nil
}

// EvaluateTransaction evaluates a transaction without submitting to the ledger
func (fc *FabricClient) EvaluateTransaction(ctx context.Context, fcn string, args []string) ([]byte, error) {
	// Select a random peer for this query
	selectedPeer := fc.selectRandomPeer()
	if err := fc.reconnectWithPeer(selectedPeer); err != nil {
		return nil, fmt.Errorf("failed to connect to peer: %w", err)
	}

	result, err := fc.contract.Evaluate(
		fcn,
		client.WithArguments(args...),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to evaluate transaction: %w", err)
	}
	return result, nil
}

// Close closes all peer connections
func (fc *FabricClient) Close() {
	for _, conn := range fc.peers {
		conn.Close()
	}
}
