package metrics

import (
	"context"
	"runtime"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/mem"
)

// MetricsCollector handles periodic collection of system metrics
type MetricsCollector struct {
	stopChan chan struct{}
	ticker   *time.Ticker
}

// NewMetricsCollector creates a new metrics collector
func NewMetricsCollector(interval time.Duration) *MetricsCollector {
	return &MetricsCollector{
		stopChan: make(chan struct{}),
		ticker:   time.NewTicker(interval),
	}
}

// Start begins collecting metrics
func (mc *MetricsCollector) Start() {
	go mc.collectLoop()
}

// Stop stops collecting metrics
func (mc *MetricsCollector) Stop() {
	close(mc.stopChan)
	mc.ticker.Stop()
}

// collectLoop runs the main collection loop
func (mc *MetricsCollector) collectLoop() {
	for {
		select {
		case <-mc.ticker.C:
			mc.collectSystemMetrics()
		case <-mc.stopChan:
			return
		}
	}
}

// collectSystemMetrics collects and records system metrics
func (mc *MetricsCollector) collectSystemMetrics() {
	// Collect memory metrics
	if vmstat, err := mem.VirtualMemory(); err == nil {
		RecordMemoryUsage(int64(vmstat.Used))
	}

	// Collect CPU metrics
	if cpuPercentages, err := cpu.Percent(0, false); err == nil && len(cpuPercentages) > 0 {
		RecordCPUUsage(cpuPercentages[0])
	}

	// Collect Go runtime metrics
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	// Record additional memory metrics if needed
	// RecordMemoryUsage(int64(m.Alloc))
}

// CollectPeerMetrics collects peer-specific metrics
func CollectPeerMetrics(ctx context.Context, peerEndpoint, channel string, connected bool, responseTime time.Duration) {
	RecordPeerConnectivity(peerEndpoint, channel, connected)
	if connected {
		RecordPeerResponseTime(peerEndpoint, "health_check", responseTime)
	}
}

// CollectCertificateMetrics collects certificate metrics
func CollectCertificateMetrics(certType, peerEndpoint string, expirationTime time.Time) {
	RecordCertificateExpiration(certType, peerEndpoint, expirationTime)
}

// CollectChannelMetrics collects channel-specific metrics
func CollectChannelMetrics(channelName string, blockHeight uint64, txType string) {
	RecordChannelBlockHeight(channelName, blockHeight)
	RecordChannelTransaction(channelName, txType)
}

// CollectChaincodeMetrics collects chaincode-specific metrics
func CollectChaincodeMetrics(chaincodeName, version, channel string) {
	RecordChaincodeVersion(chaincodeName, version, channel)
}

// CollectMSPMetrics collects MSP-specific metrics
func CollectMSPMetrics(mspID string, identityCount int) {
	RecordMSPIdentityCount(mspID, identityCount)
}

// CollectNetworkHealthMetrics collects overall network health metrics
func CollectNetworkHealthMetrics(healthy bool) {
	RecordNetworkHealth(healthy)
}

// CollectQueueMetrics collects transaction queue metrics
func CollectQueueMetrics(queueSize int, chaincode, txType string, queueLatency time.Duration) {
	RecordTransactionQueueSize(queueSize)
	if queueLatency > 0 {
		RecordTransactionQueueLatency(chaincode, txType, queueLatency)
	}
}

// CollectPerformanceMetrics collects performance-related metrics
func CollectPerformanceMetrics(operation, chaincode string, errorRate float64, throughput float64, txType string) {
	RecordErrorRate(operation, chaincode, errorRate)
	RecordTransactionThroughput(chaincode, txType, throughput)
}

// CollectAPIMetrics collects API-specific metrics
func CollectAPIMetrics(endpoint, method string, latency time.Duration, successRate float64) {
	RecordAPIEndpointLatency(endpoint, method, latency)
	RecordAPIEndpointSuccessRate(endpoint, method, successRate)
}

// CalculateErrorRate calculates error rate from success and total counts
func CalculateErrorRate(successCount, totalCount int64) float64 {
	if totalCount == 0 {
		return 0.0
	}
	return float64(totalCount-successCount) / float64(totalCount) * 100.0
}

// CalculateThroughput calculates transactions per second
func CalculateThroughput(transactionCount int64, timeWindow time.Duration) float64 {
	if timeWindow.Seconds() == 0 {
		return 0.0
	}
	return float64(transactionCount) / timeWindow.Seconds()
}

// CalculateSuccessRate calculates success rate as percentage
func CalculateSuccessRate(successCount, totalCount int64) float64 {
	if totalCount == 0 {
		return 0.0
	}
	return float64(successCount) / float64(totalCount) * 100.0
}
