package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/spf13/cobra"

	"github.com/kfsoftware/chainlaunch-plugin-hlf/pkg/api"
	"github.com/kfsoftware/chainlaunch-plugin-hlf/pkg/fabric"
)

var (
	port          string
	mspID         string
	certPath      string
	keyPath       string
	peerEndpoints string
	tlsCertPaths  string
	channelName   string
	chaincodeName string

	rootCmd  = &cobra.Command{Use: "hlf-api"}
	serveCmd = &cobra.Command{
		Use:   "serve",
		Short: "Start the API server",
		Run:   runServer,
	}
)

func init() {
	// Server flags
	serveCmd.Flags().StringVarP(&port, "port", "p", "8080", "Port to run the server on")

	// Fabric connection flags
	serveCmd.Flags().StringVar(&mspID, "mspid", "", "MSP ID of the organization")
	serveCmd.Flags().StringVar(&certPath, "cert", "", "Path to the client certificate")
	serveCmd.Flags().StringVar(&keyPath, "key", "", "Path to the client private key")
	serveCmd.Flags().StringVar(&peerEndpoints, "peers", "", "Comma-separated list of peer endpoints (host:port)")
	serveCmd.Flags().StringVar(&tlsCertPaths, "tlscerts", "", "Comma-separated list of paths to the TLS certificates (one per peer)")
	serveCmd.Flags().StringVar(&channelName, "channel", "", "Channel name")
	serveCmd.Flags().StringVar(&chaincodeName, "chaincode", "", "Chaincode name")

	// Mark required flags
	serveCmd.MarkFlagRequired("mspid")
	serveCmd.MarkFlagRequired("cert")
	serveCmd.MarkFlagRequired("key")
	serveCmd.MarkFlagRequired("peers")
	serveCmd.MarkFlagRequired("tlscerts")
	serveCmd.MarkFlagRequired("channel")
	serveCmd.MarkFlagRequired("chaincode")

	rootCmd.AddCommand(serveCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func runServer(cmd *cobra.Command, args []string) {
	// Parse peer endpoints and TLS cert paths
	peers := strings.Split(peerEndpoints, ",")
	tlsCerts := strings.Split(tlsCertPaths, ",")

	if len(peers) != len(tlsCerts) {
		log.Fatalf("Number of peer endpoints (%d) must match number of TLS certificates (%d)", len(peers), len(tlsCerts))
	}

	// Create peer configurations
	var peerConfigs []fabric.PeerConfig
	for i := range peers {
		peerConfigs = append(peerConfigs, fabric.PeerConfig{
			Endpoint:    strings.TrimSpace(peers[i]),
			TLSCertPath: strings.TrimSpace(tlsCerts[i]),
		})
	}

	// Initialize Fabric client
	fabricClient, err := fabric.NewFabricClient(&fabric.ClientConfig{
		MspID:         mspID,
		CertPath:      certPath,
		KeyPath:       keyPath,
		Peers:         peerConfigs,
		ChannelName:   channelName,
		ChaincodeName: chaincodeName,
	})
	if err != nil {
		log.Fatalf("Failed to create Fabric client: %v", err)
	}
	defer fabricClient.Close()

	// Initialize API handlers
	handler := api.NewHandler(fabricClient)

	// Set up Chi router
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// API routes
	r.Route("/api", func(r chi.Router) {
		r.Post("/invoke", handler.InvokeHandler)
		r.Post("/evaluate", handler.EvaluateHandler)
	})

	log.Printf("Server starting on port %s with %d peers configured", port, len(peerConfigs))
	log.Fatal(http.ListenAndServe(":"+port, r))
}
