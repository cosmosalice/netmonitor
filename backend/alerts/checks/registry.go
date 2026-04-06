package checks

import "github.com/netmonitor/backend/alerts"

// AllChecks 返回所有内置检查的实例
func AllChecks() []alerts.Check {
	return []alerts.Check{
		// 流量异常检查
		&BandwidthSpikeCheck{},
		&ElephantFlowCheck{},
		&LongLivedFlowCheck{},
		&HighPacketRateCheck{},

		// 主机行为检查
		&PortScanCheck{},
		&SYNFloodCheck{},
		&HighConnectionCountCheck{},
		&DataExfiltrationCheck{},
		&NewHostCheck{},

		// 协议异常检查
		&NonStandardPortCheck{},
		&CleartextCredentialCheck{},
		&DNSAnomalyCheck{},
		&UnknownProtocolCheck{},
		&EncryptedTrafficRatioCheck{},

		// 网络异常检查
		&BroadcastStormCheck{},
		&HighRetransmissionCheck{},
		&HighLatencyCheck{},
		&PacketLossCheck{},

		// 安全检查
		&ICMPFloodCheck{},
		&SuspiciousPortCheck{},
	}
}
