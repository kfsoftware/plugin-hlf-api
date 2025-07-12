package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"io/ioutil"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-openapi/spec"
	_ "github.com/kfsoftware/chainlaunch-plugin-hlf/docs" // This will be generated
	"github.com/kfsoftware/chainlaunch-plugin-hlf/pkg/api"
	"github.com/kfsoftware/chainlaunch-plugin-hlf/pkg/fabric"
	"github.com/spf13/cobra"
	httpSwagger "github.com/swaggo/http-swagger"
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
	chaincodes    []string

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
	serveCmd.Flags().StringSliceVar(&chaincodes, "chaincodes", []string{}, "Comma-separated list of chaincodes to inspect and expose as sub-APIs")

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

// ChaincodeMetadata holds metadata for a chaincode
type ChaincodeMetadata struct {
	Name     string
	Metadata []byte
}

// ChaincodeAPIServer manages dynamic chaincode APIs
type ChaincodeAPIServer struct {
	router          *chi.Mux
	handler         *api.Handler
	fabricClient    *fabric.FabricClient
	chaincodes      []string
	metadata        map[string]*ChaincodeMetadata
	reloadTicker    *time.Ticker
	reloadStop      chan struct{}
	mu              sync.RWMutex
	dynamicHandlers map[string]http.HandlerFunc // path -> handler
}

func NewChaincodeAPIServer(handler *api.Handler, fabricClient *fabric.FabricClient, chaincodes []string) *ChaincodeAPIServer {
	s := &ChaincodeAPIServer{
		router:          chi.NewRouter(),
		handler:         handler,
		fabricClient:    fabricClient,
		chaincodes:      chaincodes,
		metadata:        make(map[string]*ChaincodeMetadata),
		reloadTicker:    time.NewTicker(60 * time.Second),
		reloadStop:      make(chan struct{}),
		dynamicHandlers: make(map[string]http.HandlerFunc),
	}
	s.setupRoutes()
	s.reloadChaincodes() // Register dynamic endpoints at startup
	go s.periodicReload()
	return s
}

func (s *ChaincodeAPIServer) setupRoutes() {
	s.router.Get("/refresh", s.refreshHandler)
	// Catch-all for dynamic endpoints under /api/*
	s.router.HandleFunc("/*", s.ServeHTTP)
	// Optionally, keep NotFound for other unknown paths
	s.router.NotFound(http.NotFound)
}

// ServeHTTP implements http.Handler for dynamic endpoints
func (s *ChaincodeAPIServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	handler, ok := s.dynamicHandlers[r.URL.Path]
	s.mu.RUnlock()
	if ok {
		handler(w, r)
		return
	}
	http.NotFound(w, r)
}

// Update dynamic endpoints based on latest metadata
func (s *ChaincodeAPIServer) updateDynamicEndpoints() {
	// No locking here; locking is handled by reloadChaincodes
	// Build a set of valid paths from metadata
	validPaths := make(map[string]struct{})
	for _, meta := range s.metadata {
		var parsed struct {
			Contracts map[string]struct {
				Transactions []struct {
					Name       string `json:"name"`
					Parameters []struct {
						Name   string                 `json:"name"`
						Schema map[string]interface{} `json:"schema"`
					} `json:"parameters"`
				} `json:"transactions"`
			} `json:"contracts"`
		}
		if err := json.Unmarshal(meta.Metadata, &parsed); err != nil {
			continue
		}
		for contract, contractMeta := range parsed.Contracts {
			for _, tx := range contractMeta.Transactions {
				path := "/api/" + meta.Name + "/" + contract + "/" + tx.Name
				validPaths[path] = struct{}{}
				// Add or update handler
				s.dynamicHandlers[path] = s.makeFunctionHandler(meta.Name, contract, tx.Name, tx.Parameters)
			}
		}
	}
	// Remove handlers for paths no longer valid
	for path := range s.dynamicHandlers {
		if _, ok := validPaths[path]; !ok {
			delete(s.dynamicHandlers, path)
		}
	}
}

// Handler for a specific contract function
func (s *ChaincodeAPIServer) makeFunctionHandler(chaincode, contract, function string, params []struct {
	Name   string                 `json:"name"`
	Schema map[string]interface{} `json:"schema"`
}) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Parse body as map[string]interface{}
		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			api.SendErrorResponse(w, http.StatusBadRequest, "Invalid request body")
			return
		}
		// Build args in order
		args := make([]string, len(params))
		for i, param := range params {
			v, ok := body[param.Name]
			if !ok {
				api.SendErrorResponse(w, http.StatusBadRequest, "Missing parameter: "+param.Name)
				return
			}
			// Convert to string for Fabric
			args[i] = fmt.Sprintf("%v", v)
		}
		// Check mode (evaluate or submit)
		mode := r.URL.Query().Get("mode")
		if mode == "" {
			mode = "evaluate"
		}
		if mode == "submit" {
			txResult, err := s.fabricClient.InvokeTransaction(r.Context(), chaincode, function, args)
			if err != nil {
				api.SendErrorResponse(w, http.StatusInternalServerError, err.Error())
				return
			}
			response := api.TransactionResponse{
				Status:      "success",
				Result:      string(txResult.Result),
				TxID:        txResult.TxID,
				Success:     txResult.Success,
				BlockNumber: txResult.BlockNumber,
				ResultCode:  txResult.ResultCode,
			}
			api.SendJSONResponse(w, http.StatusOK, response)
			return
		}
		// Default: evaluate
		result, err := s.fabricClient.EvaluateTransaction(r.Context(), chaincode, function, args)
		if err != nil {
			api.SendErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
		response := api.TransactionResponse{
			Status: "success",
			Result: string(result),
		}
		api.SendJSONResponse(w, http.StatusOK, response)
	}
}

// refreshHandler godoc
// @Summary Refresh chaincode metadata and dynamic endpoints
// @Description Reloads chaincode metadata and updates dynamic endpoints for all chaincodes
// @Tags admin
// @Produce json
// @Success 200 {object} map[string]string
// @Failure 500 {object} api.TransactionResponse
// @Router /refresh [get]
func (s *ChaincodeAPIServer) refreshHandler(w http.ResponseWriter, r *http.Request) {
	if err := s.reloadChaincodes(); err != nil {
		api.SendErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	api.SendJSONResponse(w, http.StatusOK, map[string]string{"status": "refreshed"})
}

func (s *ChaincodeAPIServer) reloadChaincodes() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, cc := range s.chaincodes {
		meta, err := s.fetchChaincodeMetadata(cc)
		if err != nil {
			log.Printf("Failed to fetch metadata for %s: %v", cc, err)
			continue
		}
		s.metadata[cc] = meta
	}
	// Update dynamic endpoints after metadata reload
	s.updateDynamicEndpoints()
	return nil
}

func (s *ChaincodeAPIServer) fetchChaincodeMetadata(chaincode string) (*ChaincodeMetadata, error) {
	// Try org.hyperledger.fabric:GetMetadata, fallback to _lifecycle if needed
	result, err := s.fabricClient.EvaluateTransaction(context.Background(), chaincode, "org.hyperledger.fabric:GetMetadata", []string{})
	if err != nil {
		return nil, err
	}
	return &ChaincodeMetadata{Name: chaincode, Metadata: result}, nil
}

func (s *ChaincodeAPIServer) periodicReload() {
	for {
		select {
		case <-s.reloadTicker.C:
			log.Println("Auto-refreshing chaincode metadata...")
			s.reloadChaincodes()
		case <-s.reloadStop:
			return
		}
	}
}

func (s *ChaincodeAPIServer) Stop() {
	close(s.reloadStop)
	s.reloadTicker.Stop()
}

func generateSwaggerSpec(chaincode string) *spec.Swagger {
	initialDescription := "API for interacting with Hyperledger Fabric network"
	swagger := &spec.Swagger{
		SwaggerProps: spec.SwaggerProps{
			Swagger: "2.0",
			Info: &spec.Info{
				InfoProps: spec.InfoProps{
					Title:       "Dynamic HLF API for " + chaincode,
					Version:     "1.0",
					Description: initialDescription, // will be updated below
				},
			},
			BasePath:    "/",
			Paths:       &spec.Paths{Paths: map[string]spec.PathItem{}},
			Definitions: spec.Definitions{},
		},
	}

	meta, err := fetchChaincodeMetadataForSwagger(chaincode)
	if err == nil && meta != nil {
		var parsed struct {
			Info struct {
				Title   string `json:"title"`
				Version string `json:"version"`
			} `json:"info"`
			Contracts map[string]struct {
				Info struct {
					Title   string `json:"title"`
					Version string `json:"version"`
				} `json:"info"`
				Name         string `json:"name"`
				Transactions []struct {
					Name       string   `json:"name"`
					Tag        []string `json:"tag"`
					Parameters []struct {
						Name   string                 `json:"name"`
						Schema map[string]interface{} `json:"schema"`
					} `json:"parameters"`
					Returns map[string]interface{} `json:"returns"`
				} `json:"transactions"`
				Default bool `json:"default"`
			} `json:"contracts"`
			Components struct {
				Schemas map[string]map[string]interface{} `json:"schemas"`
			} `json:"components"`
		}
		if err := json.Unmarshal(meta.Metadata, &parsed); err == nil {
			// Add schemas to definitions
			for name, schema := range parsed.Components.Schemas {
				schemaBytes, _ := json.Marshal(schema)
				var s spec.Schema
				if err := json.Unmarshal(schemaBytes, &s); err == nil {
					swagger.Definitions[name] = s
				}
			}
			// Build contract and function links for description
			var contractLinks []string
			for contractName, contract := range parsed.Contracts {
				contractUrl := "/swagger/" + chaincode + ".json#" + contractName
				contractLink := "<a href='" + contractUrl + "'><code>" + chaincode + "/" + contractName + "</code></a>"
				// Add function links for this contract
				var functionLinks []string
				for _, tx := range contract.Transactions {
					// Build the JSON pointer for the endpoint
					jsonPath := "#/paths/~1api~1" + chaincode + "~1" + contractName + "~1" + tx.Name + "/post"
					funcUrl := "/swagger/" + chaincode + ".json" + jsonPath
					functionLinks = append(functionLinks, "<a href='"+funcUrl+"'><code>"+tx.Name+"</code></a>")
				}
				if len(functionLinks) > 0 {
					contractLink += ": [" + strings.Join(functionLinks, ", ") + "]"
				}
				contractLinks = append(contractLinks, contractLink)
			}
			if len(contractLinks) > 0 {
				swagger.Info.Description = initialDescription + "<br/><br/>Supported contracts: <br/>" + strings.Join(contractLinks, "<br/>")
			}
			// For each contract and transaction, add a POST endpoint
			for contractName, contract := range parsed.Contracts {
				for _, tx := range contract.Transactions {
					path := "/api/" + chaincode + "/" + contractName + "/" + tx.Name
					// Build parameters schema
					paramsSchema := spec.Schema{
						SchemaProps: spec.SchemaProps{
							Type:       []string{"object"},
							Properties: map[string]spec.Schema{},
							Required:   []string{},
						},
					}
					for _, param := range tx.Parameters {
						paramSchemaBytes, _ := json.Marshal(param.Schema)
						var paramSchema spec.Schema
						if err := json.Unmarshal(paramSchemaBytes, &paramSchema); err == nil {
							paramsSchema.Properties[param.Name] = paramSchema
							paramsSchema.Required = append(paramsSchema.Required, param.Name)
						}
					}
					// Add the operation
					swagger.Paths.Paths[path] = spec.PathItem{
						PathItemProps: spec.PathItemProps{
							Post: &spec.Operation{
								OperationProps: spec.OperationProps{
									Summary:     "Invoke function '" + tx.Name + "' on contract '" + contractName + "' in " + chaincode,
									Description: "Invoke function '" + tx.Name + "' on contract '" + contractName + "' in chaincode " + chaincode + `. Use the 'mode' query parameter to choose between 'evaluate' (default) and 'submit'.`,
									Consumes:    []string{"application/json"},
									Produces:    []string{"application/json"},
									Parameters: append([]spec.Parameter{
										{
											ParamProps: spec.ParamProps{
												Name:     "body",
												In:       "body",
												Required: true,
												Schema:   &paramsSchema,
											},
										},
										{
											ParamProps: spec.ParamProps{
												Name:        "mode",
												In:          "query",
												Required:    false,
												Description: "Transaction mode: 'evaluate' (default) or 'submit'",
											},
											SimpleSchema: spec.SimpleSchema{
												Type: "string",
											},
										},
									}, []spec.Parameter{}...),
									Responses: &spec.Responses{
										ResponsesProps: spec.ResponsesProps{
											StatusCodeResponses: map[int]spec.Response{
												200: {ResponseProps: spec.ResponseProps{Description: "Success"}},
												400: {ResponseProps: spec.ResponseProps{Description: "Bad Request"}},
												500: {ResponseProps: spec.ResponseProps{Description: "Internal Error"}},
											},
										},
									},
								},
							},
						},
					}
				}
			}
		}
	}

	// Always add generic invoke/evaluate endpoints
	invokePath := "/api/" + chaincode + "/invoke"
	evaluatePath := "/api/" + chaincode + "/evaluate"
	swagger.Paths.Paths[invokePath] = spec.PathItem{
		PathItemProps: spec.PathItemProps{
			Post: &spec.Operation{
				OperationProps: spec.OperationProps{
					Summary:     "Invoke " + chaincode,
					Description: "Invoke transaction on " + chaincode,
					Consumes:    []string{"application/json"},
					Produces:    []string{"application/json"},
					Parameters: []spec.Parameter{
						{
							ParamProps: spec.ParamProps{
								Name:     "body",
								In:       "body",
								Required: true,
							},
							SimpleSchema: spec.SimpleSchema{
								Type: "object",
							},
						},
					},
					Responses: &spec.Responses{
						ResponsesProps: spec.ResponsesProps{
							StatusCodeResponses: map[int]spec.Response{
								200: {ResponseProps: spec.ResponseProps{Description: "Success"}},
								400: {ResponseProps: spec.ResponseProps{Description: "Bad Request"}},
								500: {ResponseProps: spec.ResponseProps{Description: "Internal Error"}},
							},
						},
					},
				},
			},
		},
	}
	swagger.Paths.Paths[evaluatePath] = spec.PathItem{
		PathItemProps: spec.PathItemProps{
			Post: &spec.Operation{
				OperationProps: spec.OperationProps{
					Summary:     "Evaluate " + chaincode,
					Description: "Evaluate transaction on " + chaincode,
					Consumes:    []string{"application/json"},
					Produces:    []string{"application/json"},
					Parameters: []spec.Parameter{
						{
							ParamProps: spec.ParamProps{
								Name:     "body",
								In:       "body",
								Required: true,
							},
							SimpleSchema: spec.SimpleSchema{
								Type: "object",
							},
						},
					},
					Responses: &spec.Responses{
						ResponsesProps: spec.ResponsesProps{
							StatusCodeResponses: map[int]spec.Response{
								200: {ResponseProps: spec.ResponseProps{Description: "Success"}},
								400: {ResponseProps: spec.ResponseProps{Description: "Bad Request"}},
								500: {ResponseProps: spec.ResponseProps{Description: "Internal Error"}},
							},
						},
					},
				},
			},
		},
	}
	return swagger
}

// fetchChaincodeMetadataForSwagger is a helper to get metadata for a chaincode for Swagger generation
func fetchChaincodeMetadataForSwagger(chaincode string) (*ChaincodeMetadata, error) {
	// Use a temporary Fabric client for this context (reuse logic from ChaincodeAPIServer if possible)
	// For simplicity, use the same logic as fetchChaincodeMetadata in ChaincodeAPIServer
	// (Assume fabricClient is globally accessible or refactor as needed)
	if globalFabricClient == nil {
		return nil, fmt.Errorf("fabric client not initialized")
	}
	metadata, err := globalFabricClient.EvaluateChaincodeMetadata(chaincode)
	if err != nil {
		return nil, err
	}
	fmt.Printf("metadata: %s", string(metadata.Metadata))
	return &ChaincodeMetadata{Name: chaincode, Metadata: metadata.Metadata}, nil
}

// Add this method to FabricClient for metadata fetch for Swagger
type fabricClientSwagger interface {
	EvaluateChaincodeMetadata(chaincode string) (*ChaincodeMetadata, error)
}

var globalFabricClient *fabric.FabricClient

func updateStaticSwaggerDescription(chaincodes []string) {
	path := "docs/swagger.json"
	data, err := ioutil.ReadFile(path)
	if err != nil {
		log.Printf("Could not read %s: %v", path, err)
		return
	}
	var doc map[string]interface{}
	if err := json.Unmarshal(data, &doc); err != nil {
		log.Printf("Could not parse %s: %v", path, err)
		return
	}
	info, ok := doc["info"].(map[string]interface{})
	if !ok {
		log.Printf("No info section in %s", path)
		return
	}
	desc := "API for interacting with Hyperledger Fabric network.<br/><br/>Available chaincodes:<br/>"
	for _, cc := range chaincodes {
		desc += "<a href=\"/swagger/" + cc + ".json\">" + cc + "</a> "
	}
	info["description"] = desc
	newData, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		log.Printf("Could not marshal updated swagger.json: %v", err)
		return
	}
	if err := ioutil.WriteFile(path, newData, 0644); err != nil {
		log.Printf("Could not write updated swagger.json: %v", err)
	}
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
	globalFabricClient = fabricClient

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

	// Static API routes
	r.Post("/api/invoke", handler.InvokeHandler)
	r.Post("/api/evaluate", handler.EvaluateHandler)

	// Dynamic chaincode APIs
	ccAPIServer := NewChaincodeAPIServer(handler, fabricClient, chaincodes)
	r.Mount("/api", ccAPIServer.router)

	// Swagger index for all chaincodes
	r.Get("/swagger/index.json", func(w http.ResponseWriter, r *http.Request) {
		var chaincodeDocs []map[string]string
		for _, cc := range chaincodes {
			chaincodeDocs = append(chaincodeDocs, map[string]string{
				"name":            cc,
				"url":             "/swagger/" + cc + ".json",
				"contracts_index": "/swagger/" + cc + "/index.json",
			})
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(chaincodeDocs)
	})

	// Swagger contract index for each chaincode
	for _, cc := range chaincodes {
		chaincode := cc // capture range variable
		r.Get("/swagger/"+chaincode+"/index.json", func(w http.ResponseWriter, r *http.Request) {
			meta, err := fetchChaincodeMetadataForSwagger(chaincode)
			if err != nil || meta == nil {
				w.WriteHeader(http.StatusNotFound)
				w.Write([]byte("{}"))
				return
			}
			var parsed struct {
				Contracts map[string]struct {
					Info struct {
						Title   string `json:"title"`
						Version string `json:"version"`
					} `json:"info"`
				} `json:"contracts"`
			}
			if err := json.Unmarshal(meta.Metadata, &parsed); err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte("{}"))
				return
			}
			var contracts []map[string]string
			for contractName, contract := range parsed.Contracts {
				contracts = append(contracts, map[string]string{
					"name":  contractName,
					"title": contract.Info.Title,
					"url":   "/swagger/" + chaincode + ".json#" + contractName,
				})
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(contracts)
		})
	}

	// Dynamic Swagger docs for each chaincode
	for _, cc := range chaincodes {
		chaincode := cc // capture range variable
		r.Get("/swagger/"+chaincode+".json", func(w http.ResponseWriter, r *http.Request) {
			swagger := generateSwaggerSpec(chaincode)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(swagger)
		})
	}

	updateStaticSwaggerDescription(chaincodes)

	log.Printf("Server starting on port %s with %d peers configured", port, len(peerConfigs))
	log.Printf("Swagger documentation available at http://localhost:%s/swagger/", port)
	log.Fatal(http.ListenAndServe(":"+port, r))
}
