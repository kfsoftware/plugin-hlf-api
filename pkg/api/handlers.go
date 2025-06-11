package api

import (
	"encoding/json"
	"net/http"

	"github.com/kfsoftware/chainlaunch-plugin-hlf/pkg/fabric"
)

// TransactionRequest represents the incoming request structure
// @Description Transaction request structure for invoking or evaluating chaincode
type TransactionRequest struct {
	// Name of the chaincode to invoke
	ChaincodeName string `json:"chaincode_name" example:"mycc"`
	// Function name to call in the chaincode
	Function string `json:"function" example:"createAsset"`
	// Arguments to pass to the chaincode function
	Args []string `json:"args" example:"[\"asset1\",\"value1\"]"`
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
func (h *Handler) InvokeHandler(w http.ResponseWriter, r *http.Request) {
	var req TransactionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendErrorResponse(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.ChaincodeName == "" {
		sendErrorResponse(w, http.StatusBadRequest, "chaincode_name is required")
		return
	}

	txResult, err := h.fabricClient.InvokeTransaction(r.Context(), req.ChaincodeName, req.Function, req.Args)
	if err != nil {
		sendErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	response := TransactionResponse{
		Status:      "success",
		Result:      string(txResult.Result),
		TxID:        txResult.TxID,
		Success:     txResult.Success,
		BlockNumber: txResult.BlockNumber,
		ResultCode:  txResult.ResultCode,
	}
	sendJSONResponse(w, http.StatusOK, response)
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
func (h *Handler) EvaluateHandler(w http.ResponseWriter, r *http.Request) {
	var req TransactionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendErrorResponse(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.ChaincodeName == "" {
		sendErrorResponse(w, http.StatusBadRequest, "chaincode_name is required")
		return
	}

	result, err := h.fabricClient.EvaluateTransaction(r.Context(), req.ChaincodeName, req.Function, req.Args)
	if err != nil {
		sendErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	response := TransactionResponse{
		Status: "success",
		Result: string(result),
	}
	sendJSONResponse(w, http.StatusOK, response)
}

func sendJSONResponse(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func sendErrorResponse(w http.ResponseWriter, status int, message string) {
	response := TransactionResponse{
		Status: "error",
		Error:  message,
	}
	sendJSONResponse(w, status, response)
}
