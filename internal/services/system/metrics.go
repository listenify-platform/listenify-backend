// Package system provides system-level services for monitoring and maintenance.
package system

import (
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"norelock.dev/listenify/backend/internal/utils"
)

// MetricsService provides application metrics collection functionality.
type MetricsService struct {
	logger *utils.Logger

	// HTTP metrics
	httpRequestsTotal      *prometheus.CounterVec
	httpRequestDuration    *prometheus.HistogramVec
	httpRequestsInProgress *prometheus.GaugeVec

	// WebSocket metrics
	wsConnectionsTotal    prometheus.Counter
	wsConnectionsActive   prometheus.Gauge
	wsMessagesTotal       *prometheus.CounterVec
	wsMessageSizeBytes    prometheus.Histogram
	wsConnectionDuration  prometheus.Histogram

	// Room metrics
	roomsTotal     prometheus.Gauge
	roomsActive    prometheus.Gauge
	roomUsers      *prometheus.GaugeVec
	roomMediaPlays *prometheus.CounterVec

	// User metrics
	usersTotal       prometheus.Gauge
	usersActive      prometheus.Gauge
	userRegistrations prometheus.Counter
	userLogins       prometheus.Counter

	// Media metrics
	mediaItemsTotal     prometheus.Gauge
	mediaPlaybackTotal  *prometheus.CounterVec
	mediaSearchesTotal  prometheus.Counter
	mediaResolvedTotal  *prometheus.CounterVec
	mediaProxiedTotal   *prometheus.CounterVec
	mediaProxyBytesTotal prometheus.Counter

	// System metrics
	systemMemoryUsage    prometheus.Gauge
	systemCPUUsage       prometheus.Gauge
	systemGoroutines     prometheus.Gauge
	systemGCPauseNs      prometheus.Histogram
	databaseOperations   *prometheus.CounterVec
	databaseErrors       *prometheus.CounterVec
	databaseLatency      *prometheus.HistogramVec
}

// NewMetricsService creates a new metrics service.
func NewMetricsService(logger *utils.Logger) *MetricsService {
	m := &MetricsService{
		logger: logger.Named("metrics_service"),
	}

	// Initialize metrics
	m.initHTTPMetrics()
	m.initWebSocketMetrics()
	m.initRoomMetrics()
	m.initUserMetrics()
	m.initMediaMetrics()
	m.initSystemMetrics()

	return m
}

// Handler returns an HTTP handler for exposing metrics.
func (m *MetricsService) Handler() http.Handler {
	return promhttp.Handler()
}

// initHTTPMetrics initializes HTTP-related metrics.
func (m *MetricsService) initHTTPMetrics() {
	m.httpRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "listenify_http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "path", "status"},
	)

	m.httpRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "listenify_http_request_duration_seconds",
			Help:    "Duration of HTTP requests in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path"},
	)

	m.httpRequestsInProgress = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "listenify_http_requests_in_progress",
			Help: "Number of HTTP requests currently in progress",
		},
		[]string{"method", "path"},
	)
}

// initWebSocketMetrics initializes WebSocket-related metrics.
func (m *MetricsService) initWebSocketMetrics() {
	m.wsConnectionsTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "listenify_ws_connections_total",
			Help: "Total number of WebSocket connections",
		},
	)

	m.wsConnectionsActive = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "listenify_ws_connections_active",
			Help: "Number of active WebSocket connections",
		},
	)

	m.wsMessagesTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "listenify_ws_messages_total",
			Help: "Total number of WebSocket messages",
		},
		[]string{"direction", "type"},
	)

	m.wsMessageSizeBytes = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "listenify_ws_message_size_bytes",
			Help:    "Size of WebSocket messages in bytes",
			Buckets: prometheus.ExponentialBuckets(64, 2, 10),
		},
	)

	m.wsConnectionDuration = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "listenify_ws_connection_duration_seconds",
			Help:    "Duration of WebSocket connections in seconds",
			Buckets: prometheus.ExponentialBuckets(10, 2, 10),
		},
	)
}

// initRoomMetrics initializes room-related metrics.
func (m *MetricsService) initRoomMetrics() {
	m.roomsTotal = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "listenify_rooms_total",
			Help: "Total number of rooms",
		},
	)

	m.roomsActive = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "listenify_rooms_active",
			Help: "Number of active rooms",
		},
	)

	m.roomUsers = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "listenify_room_users",
			Help: "Number of users in rooms",
		},
		[]string{"room_id"},
	)

	m.roomMediaPlays = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "listenify_room_media_plays_total",
			Help: "Total number of media plays in rooms",
		},
		[]string{"room_id", "media_source"},
	)
}

// initUserMetrics initializes user-related metrics.
func (m *MetricsService) initUserMetrics() {
	m.usersTotal = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "listenify_users_total",
			Help: "Total number of registered users",
		},
	)

	m.usersActive = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "listenify_users_active",
			Help: "Number of active users",
		},
	)

	m.userRegistrations = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "listenify_user_registrations_total",
			Help: "Total number of user registrations",
		},
	)

	m.userLogins = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "listenify_user_logins_total",
			Help: "Total number of user logins",
		},
	)
}

// initMediaMetrics initializes media-related metrics.
func (m *MetricsService) initMediaMetrics() {
	m.mediaItemsTotal = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "listenify_media_items_total",
			Help: "Total number of media items",
		},
	)

	m.mediaPlaybackTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "listenify_media_playback_total",
			Help: "Total number of media playbacks",
		},
		[]string{"source"},
	)

	m.mediaSearchesTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "listenify_media_searches_total",
			Help: "Total number of media searches",
		},
	)

	m.mediaResolvedTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "listenify_media_resolved_total",
			Help: "Total number of media items resolved",
		},
		[]string{"source"},
	)

	m.mediaProxiedTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "listenify_media_proxied_total",
			Help: "Total number of media items proxied",
		},
		[]string{"source"},
	)

	m.mediaProxyBytesTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "listenify_media_proxy_bytes_total",
			Help: "Total bytes transferred through media proxy",
		},
	)
}

// initSystemMetrics initializes system-related metrics.
func (m *MetricsService) initSystemMetrics() {
	m.systemMemoryUsage = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "listenify_system_memory_usage_bytes",
			Help: "Memory usage in bytes",
		},
	)

	m.systemCPUUsage = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "listenify_system_cpu_usage",
			Help: "CPU usage percentage",
		},
	)

	m.systemGoroutines = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "listenify_system_goroutines",
			Help: "Number of goroutines",
		},
	)

	m.systemGCPauseNs = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "listenify_system_gc_pause_ns",
			Help:    "GC pause time in nanoseconds",
			Buckets: prometheus.ExponentialBuckets(1000, 2, 20),
		},
	)

	m.databaseOperations = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "listenify_database_operations_total",
			Help: "Total number of database operations",
		},
		[]string{"database", "operation"},
	)

	m.databaseErrors = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "listenify_database_errors_total",
			Help: "Total number of database errors",
		},
		[]string{"database", "operation"},
	)

	m.databaseLatency = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "listenify_database_latency_seconds",
			Help:    "Database operation latency in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"database", "operation"},
	)
}

// ObserveHTTPRequest records metrics for an HTTP request.
func (m *MetricsService) ObserveHTTPRequest(method, path string, status int, duration time.Duration) {
	m.httpRequestsTotal.WithLabelValues(method, path, http.StatusText(status)).Inc()
	m.httpRequestDuration.WithLabelValues(method, path).Observe(duration.Seconds())
}

// IncHTTPRequestsInProgress increments the in-progress HTTP requests counter.
func (m *MetricsService) IncHTTPRequestsInProgress(method, path string) {
	m.httpRequestsInProgress.WithLabelValues(method, path).Inc()
}

// DecHTTPRequestsInProgress decrements the in-progress HTTP requests counter.
func (m *MetricsService) DecHTTPRequestsInProgress(method, path string) {
	m.httpRequestsInProgress.WithLabelValues(method, path).Dec()
}

// ObserveWSConnection records metrics for a WebSocket connection.
func (m *MetricsService) ObserveWSConnection(duration time.Duration) {
	m.wsConnectionsTotal.Inc()
	m.wsConnectionDuration.Observe(duration.Seconds())
}

// IncWSConnectionsActive increments the active WebSocket connections counter.
func (m *MetricsService) IncWSConnectionsActive() {
	m.wsConnectionsActive.Inc()
}

// DecWSConnectionsActive decrements the active WebSocket connections counter.
func (m *MetricsService) DecWSConnectionsActive() {
	m.wsConnectionsActive.Dec()
}

// ObserveWSMessage records metrics for a WebSocket message.
func (m *MetricsService) ObserveWSMessage(direction, msgType string, size int) {
	m.wsMessagesTotal.WithLabelValues(direction, msgType).Inc()
	m.wsMessageSizeBytes.Observe(float64(size))
}

// SetRoomsTotal sets the total number of rooms.
func (m *MetricsService) SetRoomsTotal(count int) {
	m.roomsTotal.Set(float64(count))
}

// SetRoomsActive sets the number of active rooms.
func (m *MetricsService) SetRoomsActive(count int) {
	m.roomsActive.Set(float64(count))
}

// SetRoomUsers sets the number of users in a room.
func (m *MetricsService) SetRoomUsers(roomID string, count int) {
	m.roomUsers.WithLabelValues(roomID).Set(float64(count))
}

// IncRoomMediaPlays increments the media plays counter for a room.
func (m *MetricsService) IncRoomMediaPlays(roomID, mediaSource string) {
	m.roomMediaPlays.WithLabelValues(roomID, mediaSource).Inc()
}

// SetUsersTotal sets the total number of registered users.
func (m *MetricsService) SetUsersTotal(count int) {
	m.usersTotal.Set(float64(count))
}

// SetUsersActive sets the number of active users.
func (m *MetricsService) SetUsersActive(count int) {
	m.usersActive.Set(float64(count))
}

// IncUserRegistrations increments the user registrations counter.
func (m *MetricsService) IncUserRegistrations() {
	m.userRegistrations.Inc()
}

// IncUserLogins increments the user logins counter.
func (m *MetricsService) IncUserLogins() {
	m.userLogins.Inc()
}

// SetMediaItemsTotal sets the total number of media items.
func (m *MetricsService) SetMediaItemsTotal(count int) {
	m.mediaItemsTotal.Set(float64(count))
}

// IncMediaPlayback increments the media playback counter.
func (m *MetricsService) IncMediaPlayback(source string) {
	m.mediaPlaybackTotal.WithLabelValues(source).Inc()
}

// IncMediaSearches increments the media searches counter.
func (m *MetricsService) IncMediaSearches() {
	m.mediaSearchesTotal.Inc()
}

// IncMediaResolved increments the media resolved counter.
func (m *MetricsService) IncMediaResolved(source string) {
	m.mediaResolvedTotal.WithLabelValues(source).Inc()
}

// IncMediaProxied increments the media proxied counter.
func (m *MetricsService) IncMediaProxied(source string) {
	m.mediaProxiedTotal.WithLabelValues(source).Inc()
}

// AddMediaProxyBytes adds bytes to the media proxy bytes counter.
func (m *MetricsService) AddMediaProxyBytes(bytes int) {
	m.mediaProxyBytesTotal.Add(float64(bytes))
}

// SetSystemMemoryUsage sets the system memory usage.
func (m *MetricsService) SetSystemMemoryUsage(bytes uint64) {
	m.systemMemoryUsage.Set(float64(bytes))
}

// SetSystemCPUUsage sets the system CPU usage.
func (m *MetricsService) SetSystemCPUUsage(percentage float64) {
	m.systemCPUUsage.Set(percentage)
}

// SetSystemGoroutines sets the number of goroutines.
func (m *MetricsService) SetSystemGoroutines(count int) {
	m.systemGoroutines.Set(float64(count))
}

// ObserveSystemGCPause records a GC pause time.
func (m *MetricsService) ObserveSystemGCPause(pauseNs uint64) {
	m.systemGCPauseNs.Observe(float64(pauseNs))
}

// ObserveDatabaseOperation records metrics for a database operation.
func (m *MetricsService) ObserveDatabaseOperation(database, operation string, duration time.Duration, err error) {
	m.databaseOperations.WithLabelValues(database, operation).Inc()
	m.databaseLatency.WithLabelValues(database, operation).Observe(duration.Seconds())
	
	if err != nil {
		m.databaseErrors.WithLabelValues(database, operation).Inc()
	}
}