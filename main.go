package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	_ "github.com/kfsoftware/chainlaunch-plugin-hlf/docs" // This will be generated
	"github.com/spf13/cobra"
	httpSwagger "github.com/swaggo/http-swagger"

	"github.com/kfsoftware/chainlaunch-plugin-hlf/pkg/api"
	"github.com/kfsoftware/chainlaunch-plugin-hlf/pkg/fabric"
)

// @title Hyperledger Fabric API
// @version 1.0
// @description API for interacting with Hyperledger Fabric network
// @BasePath /
// @schemes http https

var (
	port          string
	mspID         string
	certPath      string
	keyPath       string
	peerEndpoints string
	tlsCertPaths  string
	channelName   string

	rootCmd  = &cobra.Command{Use: "hlf-api"}
	serveCmd = &cobra.Command{
		Use:   "serve",
		Short: "Start the API server",
		Run:   runServer,
	}
)

func init() {
	// Server flags
	serveCmd.Flags().StringVarP(&port, "port", "p", getEnvOrDefault("PORT_API", "8180"), "Port to run the server on")

	// Fabric connection flags
	serveCmd.Flags().StringVar(&mspID, "mspid", getEnvOrDefault("FABRIC_MSPID", ""), "MSP ID of the organization")
	serveCmd.Flags().StringVar(&certPath, "cert", getEnvOrDefault("FABRIC_CERT_PATH", ""), "Path to the client certificate")
	serveCmd.Flags().StringVar(&keyPath, "key", getEnvOrDefault("FABRIC_KEY_PATH", ""), "Path to the client private key")
	serveCmd.Flags().StringVar(&peerEndpoints, "peers", getEnvOrDefault("FABRIC_PEERS", ""), "Comma-separated list of peer endpoints (host:port)")
	serveCmd.Flags().StringVar(&tlsCertPaths, "tlscerts", getEnvOrDefault("FABRIC_TLS_CERTS", ""), "Comma-separated list of paths to the TLS certificates (one per peer)")
	serveCmd.Flags().StringVar(&channelName, "channel", getEnvOrDefault("FABRIC_CHANNEL", ""), "Channel name")

	// Mark required flags
	serveCmd.MarkFlagRequired("mspid")
	serveCmd.MarkFlagRequired("cert")
	serveCmd.MarkFlagRequired("key")
	serveCmd.MarkFlagRequired("peers")
	serveCmd.MarkFlagRequired("tlscerts")
	serveCmd.MarkFlagRequired("channel")

	rootCmd.AddCommand(serveCmd)
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func runServer(cmd *cobra.Command, args []string) {
	// Log all configuration parameters
	log.Printf("Starting server with the following configuration:")
	log.Printf("Port: %s", port)
	log.Printf("MSP ID: %s", mspID)
	log.Printf("Certificate Path: %s", certPath)
	log.Printf("Key Path: %s", keyPath)
	log.Printf("Peer Endpoints: %s", peerEndpoints)
	log.Printf("TLS Certificate Paths: %s", tlsCertPaths)
	log.Printf("Channel Name: %s", channelName)
	// Parse peer endpoints and TLS cert paths
	peers := strings.Split(peerEndpoints, ",")
	tlsCerts := strings.Split(tlsCertPaths, ",")
	log.Printf("Number of peers: %v", peers)
	log.Printf("Number of TLS certificates: %v", tlsCerts)
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
		MspID:       mspID,
		CertPath:    certPath,
		KeyPath:     keyPath,
		Peers:       peerConfigs,
		ChannelName: channelName,
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

	// Health check endpoint
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Swagger documentation
	r.Get("/swagger/*", httpSwagger.Handler(
		httpSwagger.URL("/swagger/doc.json"),
	))

	// API routes
	r.Route("/api", func(r chi.Router) {
		r.Post("/invoke", handler.InvokeHandler)
		r.Post("/evaluate", handler.EvaluateHandler)
	})

	log.Printf("Server starting on port %s with %d peers configured", port, len(peerConfigs))
	log.Printf("Swagger documentation available at http://localhost:%s/swagger/", port)
	log.Fatal(http.ListenAndServe(":"+port, r))
}
