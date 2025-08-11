package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/kfsoftware/chainlaunch-plugin-hlf/pkg/fabric"
	"github.com/kfsoftware/chainlaunch-plugin-hlf/pkg/metrics"
)

// TransactionRequest represents the incoming request structure
// @Description Transaction request structure for invoking or evaluating chaincode
type TransactionRequest struct {
	// Name of the chaincode to invoke
	ChaincodeName string `json:"chaincode_name" example:"mycc"`
	// Function name to call in the chaincode
	Function string `json:"function" example:"GetAllAssets"`
	// Arguments to pass to the chaincode function
	Args []string `json:"args" example:"[]"`
}

// TransactionResponse represents the response structure
// @Description Response structure for chaincode transactions
type TransactionResponse struct {
	// Status of the transaction ("success" or "error")
	Status string `json:"status" example:"success"`
	// Result of the transaction (if successful)
	Result interface{} `json:"result,omitempty" example:"{\"key\":\"value\"}" swaggertype:"string"`
	// Error message (if failed)
	Error string `json:"error,omitempty" example:"Invalid arguments"`
	// Transaction ID
	TxID string `json:"tx_id,omitempty" example:"tx123"`
	// Block number where the transaction was committed
	BlockNumber uint64 `json:"block_number,omitempty" example:"123"`
	// Result code from the chaincode
	ResultCode uint32 `json:"result_code,omitempty" example:"200"`
	// Whether the transaction was successful
	Success bool `json:"success,omitempty" example:"true"`
}

type Handler struct {
	fabricClient *fabric.FabricClient
}

func NewHandler(fabricClient *fabric.FabricClient) *Handler {
	return &Handler{
		fabricClient: fabricClient,
	}
}

// InvokeHandler godoc
// @Summary Invoke a chaincode transaction
// @Description Invokes a transaction on the Hyperledger Fabric network
// @Tags transactions
// @Accept json
// @Produce json
// @Param request body TransactionRequest true "Transaction Request"
// @Success 200 {object} TransactionResponse
// @Failure 400 {object} TransactionResponse
// @Failure 500 {object} TransactionResponse
// @Router /api/invoke [post]
// @id invokeChaincode
func (h *Handler) InvokeHandler(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	var req TransactionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		duration := time.Since(start)
		metrics.RecordTransaction("invoke", "error", "unknown", duration)
		SendErrorResponse(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.ChaincodeName == "" {
		duration := time.Since(start)
		metrics.RecordTransaction("invoke", "error", "unknown", duration)
		SendErrorResponse(w, http.StatusBadRequest, "chaincode_name is required")
		return
	}

	// Record chaincode operation
	metrics.RecordChaincodeOperation(req.ChaincodeName, "invoke", req.Function)

	// Record chaincode execution time
	executionStart := time.Now()
	txResult, err := h.fabricClient.InvokeTransaction(r.Context(), req.ChaincodeName, req.Function, req.Args)
	executionDuration := time.Since(executionStart)
	totalDuration := time.Since(start)

	if err != nil {
		metrics.RecordTransaction("invoke", "error", req.ChaincodeName, totalDuration)
		SendErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	status := "success"
	if !txResult.Success {
		status = "failed"
	}
	metrics.RecordTransaction("invoke", status, req.ChaincodeName, totalDuration)

	// Record chaincode execution duration and transaction status
	metrics.RecordChaincodeExecution(req.ChaincodeName, req.Function, "invoke", executionDuration)
	metrics.RecordTransactionStatus(req.ChaincodeName, req.Function, "invoke", txResult.TxID, txResult.Success, txResult.ResultCode)

	response := TransactionResponse{
		Status:      "success",
		Result:      string(txResult.Result),
		TxID:        txResult.TxID,
		Success:     txResult.Success,
		BlockNumber: txResult.BlockNumber,
		ResultCode:  txResult.ResultCode,
	}
	SendJSONResponse(w, http.StatusOK, response)
}

// ChaincodeInfo represents information about a chaincode
type ChaincodeInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Path    string `json:"path"`
}

// ChaincodesResponse represents the response for chaincode discovery
type ChaincodesResponse struct {
	Chaincodes []ChaincodeInfo `json:"chaincodes"`
	Total      int             `json:"total"`
}

// GetChaincodesHandler godoc
// @Summary Get installed chaincodes
// @Description Returns a list of installed chaincodes on the network
// @Tags chaincodes
// @Produce json
// @Success 200 {object} ChaincodesResponse
// @Failure 500 {object} TransactionResponse
// @Router /api/chaincodes [get]
// @id getChaincodes
func (h *Handler) GetChaincodesHandler(w http.ResponseWriter, r *http.Request) {
	// For now, we'll return a mock response since we don't have direct access to chaincode discovery
	// In a real implementation, you would query the network for installed chaincodes
	chaincodes := []ChaincodeInfo{
		{
			Name:    "basic",
			Version: "1.0",
			Path:    "github.com/hyperledger/fabric-samples/asset-transfer-basic/chaincode-go",
		},
		// Add more chaincodes as they are discovered
	}

	response := ChaincodesResponse{
		Chaincodes: chaincodes,
		Total:      len(chaincodes),
	}

	// Update metrics with active chaincodes count
	metrics.SetActiveChaincodes(len(chaincodes))

	SendJSONResponse(w, http.StatusOK, response)
}

// EvaluateHandler godoc
// @Summary Evaluate a chaincode transaction
// @Description Evaluates a transaction on the Hyperledger Fabric network without committing it
// @Tags transactions
// @Accept json
// @Produce json
// @Param request body TransactionRequest true "Transaction Request"
// @Success 200 {object} TransactionResponse
// @Failure 400 {object} TransactionResponse
// @Failure 500 {object} TransactionResponse
// @Router /api/evaluate [post]
// @id evaluateChaincode
func (h *Handler) EvaluateHandler(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	var req TransactionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		duration := time.Since(start)
		metrics.RecordTransaction("evaluate", "error", "unknown", duration)
		SendErrorResponse(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.ChaincodeName == "" {
		duration := time.Since(start)
		metrics.RecordTransaction("evaluate", "error", "unknown", duration)
		SendErrorResponse(w, http.StatusBadRequest, "chaincode_name is required")
		return
	}

	// Record chaincode operation
	metrics.RecordChaincodeOperation(req.ChaincodeName, "evaluate", req.Function)

	// Record chaincode execution time
	executionStart := time.Now()
	result, err := h.fabricClient.EvaluateTransaction(r.Context(), req.ChaincodeName, req.Function, req.Args)
	executionDuration := time.Since(executionStart)
	totalDuration := time.Since(start)

	if err != nil {
		metrics.RecordTransaction("evaluate", "error", req.ChaincodeName, totalDuration)
		SendErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	metrics.RecordTransaction("evaluate", "success", req.ChaincodeName, totalDuration)

	// Record chaincode execution duration (evaluate transactions don't have tx_id or result_code)
	metrics.RecordChaincodeExecution(req.ChaincodeName, req.Function, "evaluate", executionDuration)

	response := TransactionResponse{
		Status: "success",
		Result: string(result),
	}
	SendJSONResponse(w, http.StatusOK, response)
}

// Exported version of sendJSONResponse
func SendJSONResponse(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// Exported version of sendErrorResponse
func SendErrorResponse(w http.ResponseWriter, status int, message string) {
	response := TransactionResponse{
		Status: "error",
		Error:  message,
	}
	SendJSONResponse(w, status, response)
}
