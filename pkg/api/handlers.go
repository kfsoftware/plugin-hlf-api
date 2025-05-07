package api

import (
	"encoding/json"
	"net/http"

	"github.com/kfsoftware/chainlaunch-plugin-hlf/pkg/fabric"
)

// TransactionRequest represents the incoming request structure
type TransactionRequest struct {
	ChaincodeName string   `json:"chaincode_name"`
	Function      string   `json:"function"`
	Args          []string `json:"args"`
}

// TransactionResponse represents the response structure
type TransactionResponse struct {
	Status      string      `json:"status"`
	Result      interface{} `json:"result,omitempty"`
	Error       string      `json:"error,omitempty"`
	TxID        string      `json:"tx_id,omitempty"`
	BlockNumber uint64      `json:"block_number,omitempty"`
	ResultCode  uint32      `json:"result_code,omitempty"`
	Success     bool        `json:"success,omitempty"`
}

type Handler struct {
	fabricClient *fabric.FabricClient
}

func NewHandler(fabricClient *fabric.FabricClient) *Handler {
	return &Handler{
		fabricClient: fabricClient,
	}
}

func (h *Handler) InvokeHandler(w http.ResponseWriter, r *http.Request) {
	var req TransactionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendErrorResponse(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	txResult, err := h.fabricClient.InvokeTransaction(r.Context(), req.Function, req.Args)
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

func (h *Handler) EvaluateHandler(w http.ResponseWriter, r *http.Request) {
	var req TransactionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendErrorResponse(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	result, err := h.fabricClient.EvaluateTransaction(r.Context(), req.Function, req.Args)
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
