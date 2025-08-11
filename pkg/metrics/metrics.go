package metrics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// Transaction metrics
	TransactionTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "hlf_transactions_total",
			Help: "Total number of transactions by type and status",
		},
		[]string{"type", "status", "chaincode"},
	)

	TransactionDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "hlf_transaction_duration_seconds",
			Help:    "Duration of transactions in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"type", "chaincode"},
	)

	// Chaincode execution metrics
	ChaincodeExecutionDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "hlf_chaincode_execution_duration_seconds",
			Help:    "Duration of chaincode function execution in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"chaincode", "function", "type"},
	)

	TransactionStatus = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "hlf_transaction_status",
			Help: "Transaction status (1=success, 0=failed, -1=error)",
		},
		[]string{"chaincode", "function", "type", "tx_id"},
	)

	// HTTP request metrics
	HTTPRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests by method and status",
		},
		[]string{"method", "status", "endpoint"},
	)

	HTTPRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "Duration of HTTP requests in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "endpoint"},
	)

	// Chaincode metrics
	ChaincodeTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "hlf_chaincode_operations_total",
			Help: "Total number of chaincode operations by name, operation, and function",
		},
		[]string{"chaincode_name", "operation", "function"},
	)

	// Active chaincodes gauge
	ActiveChaincodes = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "hlf_active_chaincodes",
			Help: "Number of active chaincodes",
		},
	)

	// Enhanced metrics for better monitoring

	// Peer connectivity metrics
	PeerConnectivityStatus = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "hlf_peer_connectivity_status",
			Help: "Peer connectivity status (1=connected, 0=disconnected)",
		},
		[]string{"peer_endpoint", "channel"},
	)

	PeerResponseTime = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "hlf_peer_response_time_seconds",
			Help:    "Response time from peers in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"peer_endpoint", "operation"},
	)

	// Certificate metrics
	CertificateExpirationTimestamp = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "hlf_certificate_expiration_timestamp_seconds",
			Help: "Certificate expiration timestamp in seconds since unix epoch",
		},
		[]string{"cert_type", "peer_endpoint"},
	)

	CertificateDaysUntilExpiration = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "hlf_certificate_days_until_expiration",
			Help: "Number of days until certificate expiration",
		},
		[]string{"cert_type", "peer_endpoint"},
	)

	// Error rate metrics
	ErrorRate = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "hlf_error_rate",
			Help: "Error rate as a percentage (0-100)",
		},
		[]string{"operation", "chaincode"},
	)

	// Performance metrics
	TransactionThroughput = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "hlf_transaction_throughput_tps",
			Help: "Transaction throughput in transactions per second",
		},
		[]string{"chaincode", "type"},
	)

	// Memory and resource metrics
	MemoryUsageBytes = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "hlf_memory_usage_bytes",
			Help: "Current memory usage in bytes",
		},
	)

	CPUUsagePercentage = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "hlf_cpu_usage_percentage",
			Help: "Current CPU usage percentage",
		},
	)

	// Channel metrics
	ChannelBlockHeight = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "hlf_channel_block_height",
			Help: "Current block height for each channel",
		},
		[]string{"channel_name"},
	)

	ChannelTransactionCount = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "hlf_channel_transaction_count",
			Help: "Total number of transactions per channel",
		},
		[]string{"channel_name", "type"},
	)

	// API endpoint specific metrics
	APIEndpointLatency = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "hlf_api_endpoint_latency_seconds",
			Help:    "API endpoint latency in seconds",
			Buckets: []float64{0.01, 0.05, 0.1, 0.5, 1, 2, 5, 10},
		},
		[]string{"endpoint", "method"},
	)

	APIEndpointSuccessRate = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "hlf_api_endpoint_success_rate",
			Help: "API endpoint success rate as percentage (0-100)",
		},
		[]string{"endpoint", "method"},
	)

	// Chaincode version metrics
	ChaincodeVersion = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "hlf_chaincode_version_info",
			Help: "Chaincode version information (1 if version exists, 0 otherwise)",
		},
		[]string{"chaincode_name", "version", "channel"},
	)

	// MSP and organization metrics
	MSPIdentityCount = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "hlf_msp_identity_count",
			Help: "Number of identities per MSP",
		},
		[]string{"msp_id"},
	)

	// Network health metrics
	NetworkHealthStatus = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "hlf_network_health_status",
			Help: "Overall network health status (1=healthy, 0=unhealthy)",
		},
	)

	// Queue metrics for async operations
	TransactionQueueSize = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "hlf_transaction_queue_size",
			Help: "Current size of transaction processing queue",
		},
	)

	TransactionQueueLatency = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "hlf_transaction_queue_latency_seconds",
			Help:    "Time transactions spend in queue before processing",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"chaincode", "type"},
	)
)

// RecordTransaction records a transaction metric
func RecordTransaction(txType, status, chaincode string, duration time.Duration) {
	TransactionTotal.WithLabelValues(txType, status, chaincode).Inc()
	TransactionDuration.WithLabelValues(txType, chaincode).Observe(duration.Seconds())
}

// RecordChaincodeExecution records chaincode execution duration
func RecordChaincodeExecution(chaincode, function, txType string, duration time.Duration) {
	ChaincodeExecutionDuration.WithLabelValues(chaincode, function, txType).Observe(duration.Seconds())
}

// RecordTransactionStatus records the final status of a transaction
func RecordTransactionStatus(chaincode, function, txType, txID string, success bool, resultCode uint32) {
	statusValue := 0.0 // failed
	if success {
		statusValue = 1.0 // success
	} else if resultCode == 0 {
		statusValue = -1.0 // error
	}
	TransactionStatus.WithLabelValues(chaincode, function, txType, txID).Set(statusValue)
}

// RecordHTTPRequest records an HTTP request metric
func RecordHTTPRequest(method, status, endpoint string, duration time.Duration) {
	HTTPRequestsTotal.WithLabelValues(method, status, endpoint).Inc()
	HTTPRequestDuration.WithLabelValues(method, endpoint).Observe(duration.Seconds())
}

// RecordChaincodeOperation records a chaincode operation
func RecordChaincodeOperation(chaincodeName, operation, function string) {
	ChaincodeTotal.WithLabelValues(chaincodeName, operation, function).Inc()
}

// SetActiveChaincodes sets the number of active chaincodes
func SetActiveChaincodes(count int) {
	ActiveChaincodes.Set(float64(count))
}

// Enhanced metric recording functions

// RecordPeerConnectivity records peer connectivity status
func RecordPeerConnectivity(peerEndpoint, channel string, connected bool) {
	status := 0.0
	if connected {
		status = 1.0
	}
	PeerConnectivityStatus.WithLabelValues(peerEndpoint, channel).Set(status)
}

// RecordPeerResponseTime records peer response time
func RecordPeerResponseTime(peerEndpoint, operation string, duration time.Duration) {
	PeerResponseTime.WithLabelValues(peerEndpoint, operation).Observe(duration.Seconds())
}

// RecordCertificateExpiration records certificate expiration information
func RecordCertificateExpiration(certType, peerEndpoint string, expirationTime time.Time) {
	expirationTimestamp := float64(expirationTime.Unix())
	CertificateExpirationTimestamp.WithLabelValues(certType, peerEndpoint).Set(expirationTimestamp)

	daysUntilExpiration := time.Until(expirationTime).Hours() / 24
	CertificateDaysUntilExpiration.WithLabelValues(certType, peerEndpoint).Set(daysUntilExpiration)
}

// RecordErrorRate records error rate for operations
func RecordErrorRate(operation, chaincode string, errorRate float64) {
	ErrorRate.WithLabelValues(operation, chaincode).Set(errorRate)
}

// RecordTransactionThroughput records transaction throughput
func RecordTransactionThroughput(chaincode, txType string, tps float64) {
	TransactionThroughput.WithLabelValues(chaincode, txType).Set(tps)
}

// RecordMemoryUsage records memory usage
func RecordMemoryUsage(bytes int64) {
	MemoryUsageBytes.Set(float64(bytes))
}

// RecordCPUUsage records CPU usage
func RecordCPUUsage(percentage float64) {
	CPUUsagePercentage.Set(percentage)
}

// RecordChannelBlockHeight records channel block height
func RecordChannelBlockHeight(channelName string, blockHeight uint64) {
	ChannelBlockHeight.WithLabelValues(channelName).Set(float64(blockHeight))
}

// RecordChannelTransaction records channel transaction
func RecordChannelTransaction(channelName, txType string) {
	ChannelTransactionCount.WithLabelValues(channelName, txType).Inc()
}

// RecordAPIEndpointLatency records API endpoint latency
func RecordAPIEndpointLatency(endpoint, method string, duration time.Duration) {
	APIEndpointLatency.WithLabelValues(endpoint, method).Observe(duration.Seconds())
}

// RecordAPIEndpointSuccessRate records API endpoint success rate
func RecordAPIEndpointSuccessRate(endpoint, method string, successRate float64) {
	APIEndpointSuccessRate.WithLabelValues(endpoint, method).Set(successRate)
}

// RecordChaincodeVersion records chaincode version information
func RecordChaincodeVersion(chaincodeName, version, channel string) {
	ChaincodeVersion.WithLabelValues(chaincodeName, version, channel).Set(1.0)
}

// RecordMSPIdentityCount records MSP identity count
func RecordMSPIdentityCount(mspID string, count int) {
	MSPIdentityCount.WithLabelValues(mspID).Set(float64(count))
}

// RecordNetworkHealth records network health status
func RecordNetworkHealth(healthy bool) {
	status := 0.0
	if healthy {
		status = 1.0
	}
	NetworkHealthStatus.Set(status)
}

// RecordTransactionQueueSize records transaction queue size
func RecordTransactionQueueSize(size int) {
	TransactionQueueSize.Set(float64(size))
}

// RecordTransactionQueueLatency records transaction queue latency
func RecordTransactionQueueLatency(chaincode, txType string, duration time.Duration) {
	TransactionQueueLatency.WithLabelValues(chaincode, txType).Observe(duration.Seconds())
}

// HTTPMiddleware creates middleware for tracking HTTP metrics
func HTTPMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Create a response writer wrapper to capture status code
		wrapped := &responseWriter{ResponseWriter: w, statusCode: 200}

		next.ServeHTTP(wrapped, r)

		duration := time.Since(start)
		status := strconv.Itoa(wrapped.statusCode)

		// Don't track metrics for /metrics endpoint to avoid infinite loops
		if r.URL.Path != "/metrics" {
			RecordHTTPRequest(r.Method, status, r.URL.Path, duration)
			RecordAPIEndpointLatency(r.URL.Path, r.Method, duration)
		}
	})
}

// responseWriter wraps http.ResponseWriter to capture status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	return rw.ResponseWriter.Write(b)
}
