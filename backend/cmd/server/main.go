package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/netmonitor/backend/alerts"
	"github.com/netmonitor/backend/alerts/checks"
	"github.com/netmonitor/backend/alerts/notifiers"
	"github.com/netmonitor/backend/analysis"
	"github.com/netmonitor/backend/auth"
	"github.com/netmonitor/backend/capture"
	"github.com/netmonitor/backend/discovery"
	"github.com/netmonitor/backend/monitoring"
	"github.com/netmonitor/backend/pkg/config"
	"github.com/netmonitor/backend/reports"
	"github.com/netmonitor/backend/snmp"
	"github.com/netmonitor/backend/storage"
)

// Server holds all dependencies for the API handlers
type Server struct {
	captureEngine     *capture.CaptureEngine
	flowManager       *analysis.FlowManager
	hostManager       *analysis.HostManager
	protocolManager   *analysis.ProtocolManager
	metricsCalc       *analysis.MetricsCalculator
	protocolDetector  *analysis.ProtocolDetector
	tcpMetricsTracker *analysis.TCPMetricsTracker
	db                *storage.Database
	tsWriter          *storage.TimeseriesWriter
	aggManager        *storage.AggregationManager
	alertEngine       *alerts.AlertEngine
	blacklistMgr      *alerts.BlacklistManager
	threatIntelMgr    *alerts.ThreatIntelManager
	reportGen         *reports.ReportGenerator
	geoIPMgr          *analysis.GeoIPManager
	dnsStats          *analysis.DNSStats
	riskScorer        *analysis.RiskScorer
	httpStats         *analysis.HTTPStats
	tlsStats          *analysis.TLSStats
	osFingerprint     *analysis.OSFingerprint
	pcapWriter        *capture.PCAPWriter
	cfg               *config.Config
	wsHub             *WSHub
	macTracker        *discovery.MACTracker
	snmpManager       *snmp.SNMPManager
	monitor           *monitoring.ActiveMonitor
	authManager       *auth.AuthManager
	jwtManager        *auth.JWTManager
	integrationMgr    *IntegrationManager
	dashboardMgr      *DashboardManager

	// Multi-interface management
	interfaceMgr *capture.InterfaceManager

	// Flow collectors
	netflowCollector *capture.NetFlowCollector
	sflowCollector   *capture.SFlowCollector
	netflowPort      int
	sflowPort        int

	// Packet counter maintained via atomic operations
	capturedPacketCount uint64
	captureStartTime    string
}

// ---------------------------------------------------------------------------
// WebSocket Hub
// ---------------------------------------------------------------------------

// WSHub manages WebSocket client connections
type WSHub struct {
	mu       sync.RWMutex
	clients  map[*websocket.Conn]bool
	upgrader websocket.Upgrader
}

func NewWSHub() *WSHub {
	return &WSHub{
		clients: make(map[*websocket.Conn]bool),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
	}
}

func (h *WSHub) Register(conn *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.clients[conn] = true
}

func (h *WSHub) Unregister(conn *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.clients[conn]; ok {
		delete(h.clients, conn)
		conn.Close()
	}
}

func (h *WSHub) Broadcast(msg []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for conn := range h.clients {
		if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
			log.Printf("ws write error: %v", err)
			go h.Unregister(conn)
		}
	}
}

func (h *WSHub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// ---------------------------------------------------------------------------
// main
// ---------------------------------------------------------------------------

func main() {
	// Load config
	cfg, err := config.LoadConfig("config.json")
	if err != nil {
		log.Printf("Warning: failed to load config, using defaults: %v", err)
		cfg = config.DefaultConfig()
	}

	// Init SQLite database
	db, err := storage.NewDatabase(cfg.DatabasePath)
	if err != nil {
		log.Fatalf("Failed to init database: %v", err)
	}
	defer db.Close()

	// Init timeseries writer (flush every 5s, batch 100)
	tsWriter := storage.NewTimeseriesWriter(db, 5*time.Second, 100)
	defer tsWriter.Close()

	// Init aggregation manager (handles multi-granularity aggregation & cleanup)
	aggManager := storage.NewAggregationManager(db)
	aggManager.Start()
	defer aggManager.Stop()

	// Init analysis managers
	hostManager := analysis.NewHostManager()
	protocolManager := analysis.NewProtocolManager()
	metricsCalc := analysis.NewMetricsCalculator(30) // 30s sliding window

	// Init protocol detector (port-based fallback)
	protocolDetector, err := analysis.NewProtocolDetector()
	if err != nil {
		log.Fatalf("Failed to init protocol detector: %v", err)
	}
	defer protocolDetector.Close()

	// Init TCP metrics tracker
	tcpMetricsTracker := analysis.NewTCPMetricsTracker()

	// Init HTTP/TLS analyzers
	httpStats := analysis.NewHTTPStats()
	tlsStats := analysis.NewTLSStats()

	// Init DNS stats tracker
	dnsStats := analysis.NewDNSStats()

	// Init GeoIP manager
	geoIPMgr, err := analysis.NewGeoIPManager(cfg.GeoIPCityDB, cfg.GeoIPASNDB)
	if err != nil {
		log.Printf("Warning: GeoIP init failed: %v", err)
	}
	if geoIPMgr != nil {
		defer geoIPMgr.Close()
	}

	// Init flow manager with onFlowEnd callback (persist to DB)
	flowManager := analysis.NewFlowManager(100000, nil)

	// Init capture engine
	captureEngine := capture.NewCaptureEngine()

	// Init OS fingerprint analyzer
	osFingerprint := analysis.NewOSFingerprint()

	// Init PCAP writer
	pcapWriter := capture.NewPCAPWriter()

	// WebSocket hub
	wsHub := NewWSHub()

	// Init alert engine
	alertEngine := alerts.NewAlertEngine(db.GetDB())

	// Init notification manager
	notificationMgr := alerts.NewNotificationManager(db.GetDB())

	// Register notifiers
	wsNotifier := notifiers.NewWebSocketNotifier(wsHub)
	notificationMgr.RegisterNotifier(wsNotifier)

	emailNotifier := notifiers.NewEmailNotifier(notifiers.EmailConfig{})
	notificationMgr.RegisterNotifier(emailNotifier)

	webhookNotifier := notifiers.NewWebhookNotifier(notifiers.WebhookConfig{})
	notificationMgr.RegisterNotifier(webhookNotifier)

	// Wire notification manager into alert engine
	alertEngine.SetNotificationManager(notificationMgr)

	// Build server
	srv := &Server{
		captureEngine:     captureEngine,
		flowManager:       flowManager,
		hostManager:       hostManager,
		protocolManager:   protocolManager,
		metricsCalc:       metricsCalc,
		protocolDetector:  protocolDetector,
		tcpMetricsTracker: tcpMetricsTracker,
		geoIPMgr:          geoIPMgr,
		dnsStats:          dnsStats,
		osFingerprint:     osFingerprint,
		pcapWriter:        pcapWriter,
		db:                db,
		tsWriter:          tsWriter,
		aggManager:        aggManager,
		alertEngine:       alertEngine,
		httpStats:         httpStats,
		tlsStats:          tlsStats,
		cfg:               cfg,
		wsHub:             wsHub,
	}

	// Register all built-in checks
	for _, check := range checks.AllChecks() {
		alertEngine.RegisterCheck(check)
	}

	// Start alert engine
	alertEngine.Start()
	defer alertEngine.Stop()

	// Init and start blacklist manager
	blacklistMgr := alerts.NewBlacklistManager(alertEngine)
	blacklistMgr.Start()
	defer blacklistMgr.Stop()

	// Init threat intel manager
	threatIntelMgr := alerts.NewThreatIntelManager(alertEngine)

	srv.blacklistMgr = blacklistMgr
	srv.threatIntelMgr = threatIntelMgr

	// Init report generator
	reportGen := reports.NewReportGenerator(db.GetDB(), "reports")
	reportGen.Start()
	defer reportGen.Stop()
	srv.reportGen = reportGen

	// Init risk scorer
	riskScorer := analysis.NewRiskScorer()
	srv.riskScorer = riskScorer

	// Init SNMP manager
	snmpManager := snmp.NewSNMPManager(db)
	if err := snmpManager.Start(); err != nil {
		log.Printf("Warning: Failed to start SNMP manager: %v", err)
	}
	defer snmpManager.Stop()
	srv.snmpManager = snmpManager

	// Init active monitor
	monitor := monitoring.NewActiveMonitor(db)
	if err := monitor.Start(); err != nil {
		log.Printf("Warning: Failed to start active monitor: %v", err)
	}
	defer monitor.Stop()
	srv.monitor = monitor

	// Init MAC tracker with database persistence
	macTracker := discovery.NewMACTrackerWithDB(db.GetDB())
	srv.macTracker = macTracker

	// Init auth manager
	authManager := auth.NewAuthManager(db.GetDB())
	srv.authManager = authManager

	// Init JWT manager with a secret key (in production, use environment variable)
	jwtManager := auth.NewJWTManager("netmonitor-secret-key-change-in-production")
	srv.jwtManager = jwtManager

	// Init dashboard manager
	dashboardMgr := NewDashboardManager(db.GetDB())
	if err := dashboardMgr.Init(); err != nil {
		log.Printf("Warning: Failed to init dashboard manager: %v", err)
	}
	srv.dashboardMgr = dashboardMgr

	// Init integration manager
	integrationMgr := NewIntegrationManager(db.GetDB())
	if err := integrationMgr.Init(); err != nil {
		log.Printf("Warning: Failed to init integration manager: %v", err)
	}
	defer integrationMgr.Shutdown()
	srv.integrationMgr = integrationMgr

	// Init interface manager
	interfaceMgr := capture.NewInterfaceManager()
	srv.interfaceMgr = interfaceMgr

	// Init flow collectors with callbacks to process flows
	netflowCollector := capture.NewNetFlowCollector(func(flow *capture.NetFlowFlow) {
		// Process NetFlow flow - convert to internal format and update flow manager
		srv.processNetFlowFlow(flow)
	})
	srv.netflowCollector = netflowCollector
	srv.netflowPort = 2055

	sflowCollector := capture.NewSFlowCollector(func(flow *capture.SFlowFlow) {
		// Process sFlow flow - convert to internal format and update flow manager
		srv.processSFlowFlow(flow)
	})
	srv.sflowCollector = sflowCollector
	srv.sflowPort = 6343

	// Start packet processing goroutine
	go srv.packetProcessingLoop()

	// Start risk scoring loop (every 30s)
	go srv.riskScoringLoop()

	// Start WebSocket push scheduler (every 1s)
	go srv.wsPushLoop()

	// Start timeseries recording loop (every 5s)
	go srv.timeseriesRecordLoop()

	// Setup routes
	r := mux.NewRouter()

	// CORS middleware
	r.Use(corsMiddleware)

	// Auth routes (no authentication required for login)
	r.HandleFunc("/api/v1/auth/login", srv.handleLogin).Methods("POST", "OPTIONS")

	// Create authenticated router
	authRouter := r.PathPrefix("/api/v1").Subrouter()
	authRouter.Use(jwtManager.AuthMiddleware)

	// Auth routes (require authentication)
	authRouter.HandleFunc("/auth/logout", srv.handleLogout).Methods("POST", "OPTIONS")
	authRouter.HandleFunc("/auth/me", srv.handleGetMe).Methods("GET", "OPTIONS")
	authRouter.HandleFunc("/auth/password", srv.handleChangePassword).Methods("PUT", "OPTIONS")

	// User management routes (admin only)
	authRouter.HandleFunc("/users", srv.handleListUsers).Methods("GET", "OPTIONS")
	authRouter.HandleFunc("/users", srv.handleCreateUser).Methods("POST", "OPTIONS")
	authRouter.HandleFunc("/users/{id}", srv.handleUpdateUser).Methods("PUT", "OPTIONS")
	authRouter.HandleFunc("/users/{id}", srv.handleDeleteUser).Methods("DELETE", "OPTIONS")
	authRouter.HandleFunc("/users/{id}/reset-password", srv.handleResetPassword).Methods("POST", "OPTIONS")

	// API routes
	authRouter.HandleFunc("/interfaces", srv.getInterfaces).Methods("GET", "OPTIONS")
	authRouter.HandleFunc("/capture/start", srv.startCapture).Methods("POST", "OPTIONS")
	authRouter.HandleFunc("/capture/stop", srv.stopCapture).Methods("POST", "OPTIONS")
	authRouter.HandleFunc("/capture/status", srv.getCaptureStatus).Methods("GET", "OPTIONS")
	authRouter.HandleFunc("/stats/summary", srv.getSummaryStats).Methods("GET", "OPTIONS")
	authRouter.HandleFunc("/stats/hosts", srv.getHostStats).Methods("GET", "OPTIONS")
	authRouter.HandleFunc("/stats/protocols", srv.getProtocolStats).Methods("GET", "OPTIONS")
	authRouter.HandleFunc("/flows/active", srv.getActiveFlows).Methods("GET", "OPTIONS")
	authRouter.HandleFunc("/flows/{id}", srv.getFlowDetail).Methods("GET", "OPTIONS")
	authRouter.HandleFunc("/timeseries", srv.getTimeseries).Methods("GET", "OPTIONS")
	authRouter.HandleFunc("/config", srv.getConfig).Methods("GET", "OPTIONS")
	authRouter.HandleFunc("/config", srv.updateConfig).Methods("POST", "OPTIONS")

	// Alert API
	authRouter.HandleFunc("/alerts", srv.getAlerts).Methods("GET", "OPTIONS")
	authRouter.HandleFunc("/alerts/stats", srv.getAlertStats).Methods("GET", "OPTIONS")
	authRouter.HandleFunc("/alerts/rules", srv.getAlertRules).Methods("GET", "OPTIONS")
	authRouter.HandleFunc("/alerts/rules", srv.saveAlertRule).Methods("POST", "OPTIONS")
	authRouter.HandleFunc("/alerts/rules/{id}", srv.deleteAlertRule).Methods("DELETE", "OPTIONS")
	authRouter.HandleFunc("/alerts/notification-endpoints", srv.getNotificationEndpoints).Methods("GET", "OPTIONS")
	authRouter.HandleFunc("/alerts/notification-endpoints", srv.saveNotificationEndpoint).Methods("POST", "OPTIONS")
	authRouter.HandleFunc("/alerts/notification-endpoints/{id}", srv.deleteNotificationEndpoint).Methods("DELETE", "OPTIONS")
	authRouter.HandleFunc("/alerts/{id}/acknowledge", srv.acknowledgeAlert).Methods("POST", "OPTIONS")
	authRouter.HandleFunc("/alerts/{id}/resolve", srv.resolveAlert).Methods("POST", "OPTIONS")

	// Traffic matrix API
	authRouter.HandleFunc("/stats/traffic-matrix", srv.handleTrafficMatrix).Methods("GET", "OPTIONS")

	// Historical data API
	authRouter.HandleFunc("/historical/traffic", srv.handleHistoricalTraffic).Methods("GET", "OPTIONS")
	authRouter.HandleFunc("/historical/hosts", srv.handleHistoricalHosts).Methods("GET", "OPTIONS")
	authRouter.HandleFunc("/historical/protocols", srv.handleHistoricalProtocols).Methods("GET", "OPTIONS")
	authRouter.HandleFunc("/historical/compare", srv.handleHistoricalCompare).Methods("GET", "OPTIONS")
	authRouter.HandleFunc("/flows/historical", srv.handleHistoricalFlows).Methods("GET", "OPTIONS")

	// GeoIP API
	authRouter.HandleFunc("/geo/hosts", srv.getGeoHosts).Methods("GET", "OPTIONS")
	authRouter.HandleFunc("/stats/countries", srv.getCountryStats).Methods("GET", "OPTIONS")
	authRouter.HandleFunc("/stats/asn", srv.getASNStats).Methods("GET", "OPTIONS")

	// Export API
	authRouter.HandleFunc("/export/flows", srv.exportFlows).Methods("GET", "OPTIONS")
	authRouter.HandleFunc("/export/hosts", srv.exportHosts).Methods("GET", "OPTIONS")
	authRouter.HandleFunc("/export/timeseries", srv.exportTimeseries).Methods("GET", "OPTIONS")

	// DNS Analysis API
	authRouter.HandleFunc("/dns/summary", srv.getDNSSummary).Methods("GET", "OPTIONS")
	authRouter.HandleFunc("/dns/domains", srv.getDNSDomains).Methods("GET", "OPTIONS")
	authRouter.HandleFunc("/dns/servers", srv.getDNSServers).Methods("GET", "OPTIONS")
	authRouter.HandleFunc("/dns/response-codes", srv.getDNSResponseCodes).Methods("GET", "OPTIONS")
	authRouter.HandleFunc("/dns/query-types", srv.getDNSQueryTypes).Methods("GET", "OPTIONS")

	// Risk scoring API
	authRouter.HandleFunc("/hosts/risks", srv.getHostRisks).Methods("GET", "OPTIONS")
	authRouter.HandleFunc("/hosts/{ip}/risk", srv.getHostRiskDetail).Methods("GET", "OPTIONS")

	// Report API
	authRouter.HandleFunc("/reports", srv.getReports).Methods("GET", "OPTIONS")
	authRouter.HandleFunc("/reports/generate", srv.generateReport).Methods("POST", "OPTIONS")
	authRouter.HandleFunc("/reports/configs", srv.getReportConfigs).Methods("GET", "OPTIONS")
	authRouter.HandleFunc("/reports/configs", srv.saveReportConfig).Methods("POST", "OPTIONS")
	authRouter.HandleFunc("/reports/{id}/download", srv.downloadReport).Methods("GET", "OPTIONS")

	// PCAP API
	authRouter.HandleFunc("/pcap/live", srv.handlePCAPLive).Methods("GET", "OPTIONS")
	authRouter.HandleFunc("/pcap/flow/{id}", srv.handlePCAPFlow).Methods("GET", "OPTIONS")
	authRouter.HandleFunc("/pcap/host/{ip}", srv.handlePCAPHost).Methods("GET", "OPTIONS")

	// OS fingerprint API
	authRouter.HandleFunc("/hosts/{ip}/os", srv.handleHostOS).Methods("GET", "OPTIONS")

	// Topology API
	authRouter.HandleFunc("/topology", srv.getTopology).Methods("GET", "OPTIONS")

	// VLAN API
	authRouter.HandleFunc("/vlans", srv.getVLANs).Methods("GET", "OPTIONS")
	authRouter.HandleFunc("/vlans/{id}/hosts", srv.getVLANHosts).Methods("GET", "OPTIONS")
	authRouter.HandleFunc("/vlans/{id}/flows", srv.getVLANFlows).Methods("GET", "OPTIONS")

	// HTTP/TLS Analysis API
	authRouter.HandleFunc("/http/summary", srv.getHTTPSummary).Methods("GET", "OPTIONS")
	authRouter.HandleFunc("/http/hosts", srv.getHTTPHosts).Methods("GET", "OPTIONS")
	authRouter.HandleFunc("/http/user-agents", srv.getHTTPUserAgents).Methods("GET", "OPTIONS")
	authRouter.HandleFunc("/http/methods", srv.getHTTPMethods).Methods("GET", "OPTIONS")
	authRouter.HandleFunc("/http/status-codes", srv.getHTTPStatusCodes).Methods("GET", "OPTIONS")
	authRouter.HandleFunc("/tls/summary", srv.getTLSSummary).Methods("GET", "OPTIONS")
	authRouter.HandleFunc("/tls/sni", srv.getTLSSNI).Methods("GET", "OPTIONS")
	authRouter.HandleFunc("/tls/ja3", srv.getTLSJA3).Methods("GET", "OPTIONS")
	authRouter.HandleFunc("/tls/versions", srv.getTLSVersions).Methods("GET", "OPTIONS")

	// Device Discovery API (MAC tracking)
	authRouter.HandleFunc("/devices", srv.getDevices).Methods("GET", "OPTIONS")
	authRouter.HandleFunc("/devices/stats", srv.getDeviceStats).Methods("GET", "OPTIONS")
	authRouter.HandleFunc("/devices/{mac}", srv.getDevice).Methods("GET", "OPTIONS")
	authRouter.HandleFunc("/devices/{mac}/flows", srv.getDeviceFlows).Methods("GET", "OPTIONS")

	// SNMP API routes
	authRouter.HandleFunc("/snmp/devices", srv.getSNMPDevices).Methods("GET", "OPTIONS")
	authRouter.HandleFunc("/snmp/devices", srv.addSNMPDevice).Methods("POST", "OPTIONS")
	authRouter.HandleFunc("/snmp/devices/{id}", srv.getSNMPDevice).Methods("GET", "OPTIONS")
	authRouter.HandleFunc("/snmp/devices/{id}", srv.deleteSNMPDevice).Methods("DELETE", "OPTIONS")
	authRouter.HandleFunc("/snmp/devices/{id}/poll", srv.pollSNMPDevice).Methods("POST", "OPTIONS")

	// Monitoring API routes
	authRouter.HandleFunc("/monitoring/probes", srv.getMonitoringProbes).Methods("GET", "OPTIONS")
	authRouter.HandleFunc("/monitoring/probes", srv.createMonitoringProbe).Methods("POST", "OPTIONS")
	authRouter.HandleFunc("/monitoring/probes/{id}", srv.getMonitoringProbe).Methods("GET", "OPTIONS")
	authRouter.HandleFunc("/monitoring/probes/{id}", srv.deleteMonitoringProbe).Methods("DELETE", "OPTIONS")
	authRouter.HandleFunc("/monitoring/probes/{id}/results", srv.getMonitoringProbeResults).Methods("GET", "OPTIONS")
	authRouter.HandleFunc("/monitoring/probes/{id}/test", srv.testMonitoringProbe).Methods("POST", "OPTIONS")

	// Interface Management API routes
	authRouter.HandleFunc("/interfaces/all", srv.listInterfacesHandler).Methods("GET", "OPTIONS")
	authRouter.HandleFunc("/interfaces/active", srv.getActiveInterfacesHandler).Methods("GET", "OPTIONS")
	authRouter.HandleFunc("/interfaces/{name}/enable", srv.enableInterfaceHandler).Methods("POST", "OPTIONS")
	authRouter.HandleFunc("/interfaces/{name}/disable", srv.disableInterfaceHandler).Methods("POST", "OPTIONS")
	authRouter.HandleFunc("/interfaces/{name}/stats", srv.getInterfaceStatsHandler).Methods("GET", "OPTIONS")
	authRouter.HandleFunc("/interfaces/stats/aggregate", srv.getInterfacesAggregateStats).Methods("GET", "OPTIONS")

	// Flow Collector API routes
	authRouter.HandleFunc("/collectors", srv.listCollectorsHandler).Methods("GET", "OPTIONS")
	authRouter.HandleFunc("/collectors/stats", srv.getCollectorStatsHandler).Methods("GET", "OPTIONS")
	authRouter.HandleFunc("/collectors/flows", srv.getCollectorFlowsHandler).Methods("GET", "OPTIONS")
	authRouter.HandleFunc("/collectors/netflow/start", srv.startNetFlowCollectorHandler).Methods("POST", "OPTIONS")
	authRouter.HandleFunc("/collectors/netflow/stop", srv.stopNetFlowCollectorHandler).Methods("POST", "OPTIONS")
	authRouter.HandleFunc("/collectors/sflow/start", srv.startSFlowCollectorHandler).Methods("POST", "OPTIONS")
	authRouter.HandleFunc("/collectors/sflow/stop", srv.stopSFlowCollectorHandler).Methods("POST", "OPTIONS")

	// Integrations API routes
	authRouter.HandleFunc("/integrations", srv.getIntegrations).Methods("GET", "OPTIONS")
	authRouter.HandleFunc("/integrations/syslog", srv.getSyslogConfig).Methods("GET", "OPTIONS")
	authRouter.HandleFunc("/integrations/syslog", srv.updateSyslogConfig).Methods("PUT", "OPTIONS")
	authRouter.HandleFunc("/integrations/syslog/test", srv.testSyslogConnection).Methods("POST", "OPTIONS")
	authRouter.HandleFunc("/integrations/elasticsearch", srv.getESConfig).Methods("GET", "OPTIONS")
	authRouter.HandleFunc("/integrations/elasticsearch", srv.updateESConfig).Methods("PUT", "OPTIONS")
	authRouter.HandleFunc("/integrations/elasticsearch/test", srv.testESConnection).Methods("POST", "OPTIONS")

	// Custom Dashboard API routes
	authRouter.HandleFunc("/dashboards", srv.getDashboards).Methods("GET", "OPTIONS")
	authRouter.HandleFunc("/dashboards", srv.createDashboard).Methods("POST", "OPTIONS")
	authRouter.HandleFunc("/dashboards/{id}", srv.getDashboard).Methods("GET", "OPTIONS")
	authRouter.HandleFunc("/dashboards/{id}", srv.updateDashboard).Methods("PUT", "OPTIONS")
	authRouter.HandleFunc("/dashboards/{id}", srv.deleteDashboard).Methods("DELETE", "OPTIONS")

	// WebSocket
	r.HandleFunc("/ws/realtime", srv.handleWebSocket)

	port := cfg.APIPort
	if port == 0 {
		port = 8080
	}
	addr := fmt.Sprintf(":%d", port)
	fmt.Printf("NetMonitor Backend starting on %s\n", addr)
	log.Fatal(http.ListenAndServe(addr, r))
}

// ---------------------------------------------------------------------------
// CORS middleware
// ---------------------------------------------------------------------------

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// ---------------------------------------------------------------------------
// Packet processing pipeline
// ---------------------------------------------------------------------------

func (s *Server) packetProcessingLoop() {
	pktCh := s.captureEngine.GetPacketChannel()
	for pkt := range pktCh {
		s.processPacket(pkt)
	}
}

func (s *Server) processPacket(pkt capture.Packet) {
	// Increment our own packet counter (atomic, lock-free)
	atomic.AddUint64(&s.capturedPacketCount, 1)

	srcIP, dstIP, srcPort, dstPort, l4Proto, pktLen, err := analysis.ParsePacketInfo(pkt)
	if err != nil {
		return // skip non-IP packets
	}

	// 1. Flow tracking (with VLAN ID from packet)
	flow := s.flowManager.GetOrCreateFlow(srcIP, dstIP, srcPort, dstPort, l4Proto, pkt.VLANID)
	s.flowManager.UpdateFlow(flow.ID, pktLen, true)

	// 2. Host tracking
	s.hostManager.UpdateHost(srcIP, "", pktLen, true)
	s.hostManager.UpdateHost(dstIP, "", pktLen, false)

	// 3. L7 protocol detection
	proto, err := s.protocolDetector.DetectProtocol(pkt)
	if err == nil && proto != nil {
		s.protocolManager.UpdateProtocol(proto.Name, proto.Category, pktLen)
		// Update flow L7 info if not yet set
		if flow.L7Protocol == "" {
			flow.L7Protocol = proto.Name
			flow.L7Category = proto.Category
		}
		// Update host protocol stats
		s.hostManager.UpdateHostProtocol(srcIP, proto.Name, pktLen)
	}

	// 4. TCP metrics tracking
	if l4Proto == "TCP" {
		s.tcpMetricsTracker.ProcessPacket(flow.ID, pkt)
		// Attach current TCP metrics to the flow
		flow.TCPMetrics = s.tcpMetricsTracker.GetMetrics(flow.ID)
	}

	// 5. Blacklist check on src/dst IP
	s.blacklistMgr.CheckIP(srcIP)
	s.blacklistMgr.CheckIP(dstIP)

	// 6. GeoIP enrichment
	if s.geoIPMgr != nil {
		for _, ip := range []string{srcIP, dstIP} {
			if !analysis.IsPrivateIP(ip) {
				geo := s.geoIPMgr.Lookup(ip)
				if geo != nil && geo.Country != "" {
					host := s.hostManager.GetHostStats(ip)
					if host != nil && host.Country == "" {
						host.Country = geo.Country
						host.City = geo.City
						host.Latitude = geo.Latitude
						host.Longitude = geo.Longitude
						host.ASN = geo.ASN
						host.ASOrg = geo.ASOrg
					}
				}
			}
		}
	}

	// 7. DNS analysis
	gpkt := gopacket.NewPacket(pkt.Data, layers.LayerTypeEthernet, gopacket.NoCopy)
	if dnsLayer := gpkt.Layer(layers.LayerTypeDNS); dnsLayer != nil {
		if dns, ok := dnsLayer.(*layers.DNS); ok {
			s.dnsStats.ProcessDNSPacket(dns, srcIP, dstIP)
		}
	}

	// 8. HTTP/TLS deep analysis
	if l4Proto == "TCP" {
		if tcpLayer2 := gpkt.Layer(layers.LayerTypeTCP); tcpLayer2 != nil {
			tcp2, _ := tcpLayer2.(*layers.TCP)
			if len(tcp2.Payload) > 0 {
				if analysis.IsHTTPPort(srcPort) || analysis.IsHTTPPort(dstPort) || analysis.IsHTTPPayload(tcp2.Payload) {
					s.httpStats.ProcessPacket(pkt, srcPort, dstPort, pktLen)
				} else if analysis.IsTLSClientHello(tcp2.Payload) {
					s.tlsStats.ProcessPacket(pkt)
				}
			}
		}
	}

	// 9. OS fingerprint (only on SYN packets)
	if l4Proto == "TCP" && s.osFingerprint != nil {
		if tcpLayer3 := gpkt.Layer(layers.LayerTypeTCP); tcpLayer3 != nil {
			tcp3, _ := tcpLayer3.(*layers.TCP)
			if tcp3 != nil && tcp3.SYN && !tcp3.ACK {
				var ttl uint8
				var df bool
				if ipv4Layer := gpkt.Layer(layers.LayerTypeIPv4); ipv4Layer != nil {
					ipv4, _ := ipv4Layer.(*layers.IPv4)
					if ipv4 != nil {
						ttl = ipv4.TTL
						df = ipv4.Flags&layers.IPv4DontFragment != 0
					}
				}
				if ttl > 0 {
					s.osFingerprint.AnalyzePacket(srcIP, ttl, tcp3.Window, tcp3.Options, df)
					// Attach OS info to host
					if osInfo := s.osFingerprint.GetOSInfo(srcIP); osInfo != nil {
						host := s.hostManager.GetHostStats(srcIP)
						if host != nil {
							host.OS = osInfo
						}
					}
				}
			}
		}
	}

	// 10. PCAP buffering
	if s.pcapWriter != nil {
		s.pcapWriter.AddPacket(gpkt)
	}

	// 11. Metrics
	s.metricsCalc.Update(pktLen, 1)
}

// ---------------------------------------------------------------------------
// WebSocket push scheduler
// ---------------------------------------------------------------------------

func (s *Server) wsPushLoop() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		if s.wsHub.ClientCount() == 0 {
			continue
		}
		data := s.buildRealtimePayload()
		msg, err := json.Marshal(data)
		if err != nil {
			log.Printf("ws marshal error: %v", err)
			continue
		}
		s.wsHub.Broadcast(msg)
	}
}

func (s *Server) buildRealtimePayload() map[string]interface{} {
	metrics := s.metricsCalc.GetMetrics(
		s.flowManager.GetFlowCount(),
		s.hostManager.GetHostCount(),
		s.protocolManager.GetProtocolCount(),
	)

	// Build flow info for check context
	activeFlows := s.flowManager.GetActiveFlows()
	flowInfos := make([]alerts.FlowInfo, 0, len(activeFlows))
	for _, f := range activeFlows {
		fi := alerts.FlowInfo{
			FlowID:      f.ID,
			SrcIP:       f.SrcIP,
			DstIP:       f.DstIP,
			SrcPort:     f.SrcPort,
			DstPort:     f.DstPort,
			Protocol:    f.Protocol,
			L7Protocol:  f.L7Protocol,
			BytesSent:   f.BytesSent,
			BytesRecv:   f.BytesRecv,
			PacketsSent: f.PacketsSent,
			PacketsRecv: f.PacketsRecv,
			StartTime:   f.StartTime,
			LastSeen:    f.LastSeen,
			IsActive:    f.IsActive,
		}
		if f.TCPMetrics != nil {
			fi.Retransmissions = f.TCPMetrics.Retransmissions
			fi.RTTMs = f.TCPMetrics.AvgRTT
		}
		flowInfos = append(flowInfos, fi)
	}

	// Build host info for check context
	allHosts := s.hostManager.GetAllHosts()
	hostInfos := make([]alerts.HostInfo, 0, len(allHosts))
	for _, h := range allHosts {
		hostInfos = append(hostInfos, alerts.HostInfo{
			IP:          h.IP,
			BytesSent:   h.BytesSent,
			BytesRecv:   h.BytesRecv,
			PacketsSent: h.PacketsSent,
			PacketsRecv: h.PacketsRecv,
			FlowCount:   h.ActiveFlows,
			Protocols:   h.Protocols,
			FirstSeen:   h.FirstSeen,
			LastSeen:    h.LastSeen,
		})
	}

	// Update alert engine check context
	s.alertEngine.UpdateCheckContext(&alerts.CheckContext{
		ActiveFlows:   metrics.ActiveFlows,
		ActiveHosts:   metrics.ActiveHosts,
		BytesPerSec:   float64(metrics.BytesPerSec),
		PacketsPerSec: float64(metrics.PacketsPerSec),
		Flows:         flowInfos,
		Hosts:         hostInfos,
	})

	topProtocols := s.protocolManager.GetTopProtocols(10)
	topHosts := s.hostManager.GetTopTalkers(10, "total")

	return map[string]interface{}{
		"type": "stats_update",
		"data": map[string]interface{}{
			"bandwidth": map[string]interface{}{
				"bytes_per_sec":   metrics.BytesPerSec,
				"packets_per_sec": metrics.PacketsPerSec,
			},
			"pps":             metrics.PacketsPerSec,
			"activeFlows":     metrics.ActiveFlows,
			"activeHosts":     metrics.ActiveHosts,
			"activeProtocols": metrics.ActiveProtocols,
			"topProtocols":    topProtocols,
			"topHosts":        topHosts,
			"alertCount":      s.alertEngine.GetActiveAlertCount(),
			"timestamp":       time.Now(),
		},
	}
}

// ---------------------------------------------------------------------------
// Timeseries recording (periodic snapshot to DB)
// ---------------------------------------------------------------------------

func (s *Server) timeseriesRecordLoop() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		now := time.Now()
		metrics := s.metricsCalc.GetMetrics(
			s.flowManager.GetFlowCount(),
			s.hostManager.GetHostCount(),
			s.protocolManager.GetProtocolCount(),
		)
		s.tsWriter.Add(storage.TimeseriesPoint{Timestamp: now, MetricType: "bandwidth", MetricKey: "bytes_per_sec", Value: float64(metrics.BytesPerSec)})
		s.tsWriter.Add(storage.TimeseriesPoint{Timestamp: now, MetricType: "bandwidth", MetricKey: "packets_per_sec", Value: float64(metrics.PacketsPerSec)})
		s.tsWriter.Add(storage.TimeseriesPoint{Timestamp: now, MetricType: "counts", MetricKey: "active_flows", Value: float64(metrics.ActiveFlows)})
		s.tsWriter.Add(storage.TimeseriesPoint{Timestamp: now, MetricType: "counts", MetricKey: "active_hosts", Value: float64(metrics.ActiveHosts)})
		s.tsWriter.Add(storage.TimeseriesPoint{Timestamp: now, MetricType: "counts", MetricKey: "active_protocols", Value: float64(metrics.ActiveProtocols)})
	}
}

// ---------------------------------------------------------------------------
// HTTP Handlers
// ---------------------------------------------------------------------------

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

// GET /api/v1/interfaces
func (s *Server) getInterfaces(w http.ResponseWriter, r *http.Request) {
	interfaces, err := capture.ListInterfaces()
	if err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]interface{}{"interfaces": interfaces})
}

// POST /api/v1/capture/start
func (s *Server) startCapture(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Interface string `json:"interface"`
		BPFFilter string `json:"bpf_filter"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, err.Error(), http.StatusBadRequest)
		return
	}

	cfg := capture.CaptureConfig{
		Interface: req.Interface,
		BPFFilter: req.BPFFilter,
		Snaplen:   65536,
		Promisc:   true,
		Timeout:   1 * time.Second, // short timeout so captureLoop can check cancellation
	}
	if err := s.captureEngine.Start(cfg); err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Record start time and reset packet counter
	s.captureStartTime = time.Now().Format(time.RFC3339)
	atomic.StoreUint64(&s.capturedPacketCount, 0)

	writeJSON(w, map[string]string{"status": "capture started"})
}

// POST /api/v1/capture/stop
func (s *Server) stopCapture(w http.ResponseWriter, r *http.Request) {
	if err := s.captureEngine.Stop(); err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// Reset packet counter on stop
	atomic.StoreUint64(&s.capturedPacketCount, 0)
	writeJSON(w, map[string]string{"status": "capture stopped"})
}

// GET /api/v1/capture/status
func (s *Server) getCaptureStatus(w http.ResponseWriter, r *http.Request) {
	result := map[string]interface{}{
		"is_running": s.captureEngine.IsRunning(),
	}

	// Always provide our own stats so the frontend can read packets_captured
	statsMap := map[string]interface{}{
		"packets_captured": atomic.LoadUint64(&s.capturedPacketCount),
		"start_time":       s.captureStartTime,
	}

	// Merge pcap-level stats when available
	pcapStats, err := s.captureEngine.GetStats()
	if err == nil {
		for k, v := range pcapStats {
			statsMap[k] = v
		}
	}

	result["stats"] = statsMap
	writeJSON(w, result)
}

// GET /api/v1/stats/summary
func (s *Server) getSummaryStats(w http.ResponseWriter, r *http.Request) {
	metrics := s.metricsCalc.GetMetrics(
		s.flowManager.GetFlowCount(),
		s.hostManager.GetHostCount(),
		s.protocolManager.GetProtocolCount(),
	)
	writeJSON(w, map[string]interface{}{
		"bandwidth": map[string]interface{}{
			"bytes_per_sec":   metrics.BytesPerSec,
			"packets_per_sec": metrics.PacketsPerSec,
		},
		"pps":              metrics.PacketsPerSec,
		"active_flows":     metrics.ActiveFlows,
		"active_hosts":     metrics.ActiveHosts,
		"active_protocols": metrics.ActiveProtocols,
		"total_bytes":      s.protocolManager.GetTotalBytes(),
		"timestamp":        metrics.Timestamp,
	})
}

// GET /api/v1/stats/hosts
func (s *Server) getHostStats(w http.ResponseWriter, r *http.Request) {
	limitStr := r.URL.Query().Get("limit")
	limit := 100
	if limitStr != "" {
		if v, err := strconv.Atoi(limitStr); err == nil && v > 0 {
			limit = v
		}
	}

	sortBy := r.URL.Query().Get("sort")
	if sortBy == "" {
		sortBy = "total"
	}

	hosts := s.hostManager.GetTopTalkers(limit, sortBy)
	writeJSON(w, map[string]interface{}{"hosts": hosts})
}

// GET /api/v1/stats/protocols
func (s *Server) getProtocolStats(w http.ResponseWriter, r *http.Request) {
	protocols := s.protocolManager.GetAllProtocols()
	writeJSON(w, map[string]interface{}{
		"protocols":   protocols,
		"total_bytes": s.protocolManager.GetTotalBytes(),
	})
}

// GET /api/v1/flows/active
func (s *Server) getActiveFlows(w http.ResponseWriter, r *http.Request) {
	flows := s.flowManager.GetActiveFlows()
	writeJSON(w, map[string]interface{}{"flows": flows})
}

// GET /api/v1/flows/{id}
func (s *Server) getFlowDetail(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	flowID := vars["id"]
	if flowID == "" {
		writeError(w, "flow id is required", http.StatusBadRequest)
		return
	}

	// First try to get from in-memory flow manager (for active flows)
	activeFlows := s.flowManager.GetActiveFlows()
	for _, f := range activeFlows {
		if f.ID == flowID {
			// Build response with live TCP metrics
			result := map[string]interface{}{
				"flow_id":      f.ID,
				"src_ip":       f.SrcIP,
				"dst_ip":       f.DstIP,
				"src_port":     f.SrcPort,
				"dst_port":     f.DstPort,
				"protocol":     f.Protocol,
				"l7_protocol":  f.L7Protocol,
				"l7_category":  f.L7Category,
				"bytes_sent":   f.BytesSent,
				"bytes_recv":   f.BytesRecv,
				"packets_sent": f.PacketsSent,
				"packets_recv": f.PacketsRecv,
				"start_time":   f.StartTime,
				"last_seen":    f.LastSeen,
				"is_active":    f.IsActive,
			}

			// Attach live TCP metrics
			if tcpMetrics := s.tcpMetricsTracker.GetMetrics(flowID); tcpMetrics != nil {
				result["tcp_metrics"] = tcpMetrics
			}

			writeJSON(w, result)
			return
		}
	}

	// Fall back to database for historical flows
	flowDetail, err := s.db.GetFlowByID(flowID)
	if err != nil {
		writeError(w, "flow not found", http.StatusNotFound)
		return
	}

	writeJSON(w, flowDetail)
}

// GET /api/v1/timeseries
func (s *Server) getTimeseries(w http.ResponseWriter, r *http.Request) {
	metricType := r.URL.Query().Get("type")
	if metricType == "" {
		metricType = "bandwidth"
	}

	// Default: last 1 hour
	endTime := time.Now()
	startTime := endTime.Add(-1 * time.Hour)

	if st := r.URL.Query().Get("start"); st != "" {
		if t, err := time.Parse(time.RFC3339, st); err == nil {
			startTime = t
		}
	}
	if et := r.URL.Query().Get("end"); et != "" {
		if t, err := time.Parse(time.RFC3339, et); err == nil {
			endTime = t
		}
	}

	data, err := s.db.QueryTimeseries(metricType, startTime, endTime)
	if err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if data == nil {
		data = []map[string]interface{}{}
	}
	writeJSON(w, map[string]interface{}{"data": data})
}

// GET /api/v1/config
func (s *Server) getConfig(w http.ResponseWriter, r *http.Request) {
	c := s.cfg.Get()
	writeJSON(w, map[string]interface{}{
		"interface":       c.Interface,
		"bpf_filter":      c.BPFFilter,
		"promisc_mode":    c.PromiscMode,
		"snaplen":         c.Snaplen,
		"database_path":   c.DatabasePath,
		"retention_hours": c.RetentionHours,
		"api_port":        c.APIPort,
		"theme":           c.Theme,
	})
}

// POST /api/v1/config
func (s *Server) updateConfig(w http.ResponseWriter, r *http.Request) {
	var updates map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		writeError(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := s.cfg.Update(updates); err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]string{"status": "ok"})
}

// WebSocket handler
func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := s.wsHub.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("WebSocket upgrade error:", err)
		return
	}

	s.wsHub.Register(conn)
	log.Printf("WebSocket client connected (total: %d)", s.wsHub.ClientCount())

	// Keep connection alive; read messages to detect disconnect
	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			s.wsHub.Unregister(conn)
			log.Printf("WebSocket client disconnected (total: %d)", s.wsHub.ClientCount())
			break
		}
	}
}

// processNetFlowFlow processes a NetFlow flow and updates the flow manager
func (s *Server) processNetFlowFlow(flow *capture.NetFlowFlow) {
	// Convert protocol number to string
	protoStr := "UNKNOWN"
	switch flow.Protocol {
	case 6:
		protoStr = "TCP"
	case 17:
		protoStr = "UDP"
	case 1:
		protoStr = "ICMP"
	}

	// Get or create flow
	f := s.flowManager.GetOrCreateFlow(flow.SrcIP, flow.DstIP, flow.SrcPort, flow.DstPort, protoStr, 0)

	// Update flow statistics
	s.flowManager.UpdateFlow(f.ID, flow.Bytes, true)

	// Update host statistics
	s.hostManager.UpdateHost(flow.SrcIP, "", flow.Bytes, true)
	s.hostManager.UpdateHost(flow.DstIP, "", flow.Bytes, false)
}

// processSFlowFlow processes an sFlow flow and updates the flow manager
func (s *Server) processSFlowFlow(flow *capture.SFlowFlow) {
	// Convert protocol number to string
	protoStr := "UNKNOWN"
	switch flow.Protocol {
	case 6:
		protoStr = "TCP"
	case 17:
		protoStr = "UDP"
	case 1:
		protoStr = "ICMP"
	}

	// Get or create flow
	f := s.flowManager.GetOrCreateFlow(flow.SrcIP, flow.DstIP, flow.SrcPort, flow.DstPort, protoStr, 0)

	// Update flow statistics
	s.flowManager.UpdateFlow(f.ID, flow.Bytes, true)

	// Update host statistics
	s.hostManager.UpdateHost(flow.SrcIP, "", flow.Bytes, true)
	s.hostManager.UpdateHost(flow.DstIP, "", flow.Bytes, false)
}
