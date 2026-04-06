package storage

import (
	"database/sql"
	"fmt"
	"log"
	"math"
	"time"
)

// AggregatedPoint represents a single aggregated timeseries data point
type AggregatedPoint struct {
	Timestamp   time.Time `json:"timestamp"`
	MetricType  string    `json:"metric_type"`
	MetricKey   string    `json:"metric_key"`
	AvgValue    float64   `json:"avg_value"`
	MaxValue    float64   `json:"max_value"`
	MinValue    float64   `json:"min_value"`
	P95Value    float64   `json:"p95_value"`
	SampleCount int       `json:"sample_count"`
}

// Retention periods for each granularity
const (
	RetentionRaw  = 24 * time.Hour       // raw data: 24 hours
	Retention5Min = 7 * 24 * time.Hour   // 5min aggregation: 7 days
	Retention1H   = 90 * 24 * time.Hour  // 1h aggregation: 90 days
	Retention1D   = 365 * 24 * time.Hour // 1d aggregation: 365 days
)

// AggregationManager manages multi-granularity aggregation of timeseries data
type AggregationManager struct {
	db   *sql.DB
	done chan struct{}
}

// NewAggregationManager creates a new AggregationManager
func NewAggregationManager(db *Database) *AggregationManager {
	return &AggregationManager{
		db:   db.GetDB(),
		done: make(chan struct{}),
	}
}

// Start launches background goroutines for periodic aggregation
func (am *AggregationManager) Start() {
	log.Println("AggregationManager started")

	// 5-minute aggregation ticker
	go am.runTicker(5*time.Minute, "5min", am.aggregate5Min)

	// 1-hour aggregation ticker
	go am.runTicker(1*time.Hour, "1h", am.aggregate1H)

	// 1-day aggregation ticker
	go am.runTicker(24*time.Hour, "1d", am.aggregate1D)

	// Cleanup ticker (runs every hour)
	go am.runTicker(1*time.Hour, "cleanup", am.cleanupExpired)
}

// Stop gracefully stops all aggregation goroutines
func (am *AggregationManager) Stop() {
	close(am.done)
	log.Println("AggregationManager stopped")
}

// runTicker runs a function on a periodic ticker, stopping when done is closed
func (am *AggregationManager) runTicker(interval time.Duration, name string, fn func() error) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := fn(); err != nil {
				log.Printf("Aggregation [%s] error: %v", name, err)
			}
		case <-am.done:
			return
		}
	}
}

// aggregate5Min aggregates raw timeseries data into 5-minute buckets
func (am *AggregationManager) aggregate5Min() error {
	windowEnd := time.Now().Truncate(5 * time.Minute)
	windowStart := windowEnd.Add(-5 * time.Minute)

	return am.aggregateFromRaw("timeseries_5min", windowStart, windowEnd)
}

// aggregate1H aggregates 5-min data into 1-hour buckets
func (am *AggregationManager) aggregate1H() error {
	windowEnd := time.Now().Truncate(1 * time.Hour)
	windowStart := windowEnd.Add(-1 * time.Hour)

	return am.aggregateFromAggregated("timeseries_5min", "timeseries_1h", windowStart, windowEnd)
}

// aggregate1D aggregates 1-hour data into 1-day buckets
func (am *AggregationManager) aggregate1D() error {
	windowEnd := time.Now().Truncate(24 * time.Hour)
	windowStart := windowEnd.Add(-24 * time.Hour)

	return am.aggregateFromAggregated("timeseries_1h", "timeseries_1d", windowStart, windowEnd)
}

// aggregateFromRaw aggregates raw timeseries data into a destination table
func (am *AggregationManager) aggregateFromRaw(destTable string, windowStart, windowEnd time.Time) error {
	tx, err := am.db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	// Get distinct metric combinations in the window
	rows, err := tx.Query(`
		SELECT DISTINCT metric_type, metric_key
		FROM timeseries
		WHERE timestamp >= ? AND timestamp < ?
	`, windowStart, windowEnd)
	if err != nil {
		return fmt.Errorf("query distinct metrics: %w", err)
	}

	type metricPair struct {
		metricType string
		metricKey  string
	}
	var pairs []metricPair
	for rows.Next() {
		var p metricPair
		if err := rows.Scan(&p.metricType, &p.metricKey); err != nil {
			rows.Close()
			return fmt.Errorf("scan metric pair: %w", err)
		}
		pairs = append(pairs, p)
	}
	rows.Close()

	insertStmt, err := tx.Prepare(fmt.Sprintf(`
		INSERT INTO %s (timestamp, metric_type, metric_key, avg_value, max_value, min_value, p95_value, sample_count)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, destTable))
	if err != nil {
		return fmt.Errorf("prepare insert: %w", err)
	}
	defer insertStmt.Close()

	for _, p := range pairs {
		// Check if already aggregated
		var exists int
		err := tx.QueryRow(fmt.Sprintf(`
			SELECT COUNT(*) FROM %s
			WHERE timestamp = ? AND metric_type = ? AND metric_key = ?
		`, destTable), windowStart, p.metricType, p.metricKey).Scan(&exists)
		if err != nil {
			return fmt.Errorf("check exists: %w", err)
		}
		if exists > 0 {
			continue
		}

		// Get AVG, MAX, MIN, COUNT
		var avgVal, maxVal, minVal float64
		var count int
		err = tx.QueryRow(`
			SELECT AVG(value), MAX(value), MIN(value), COUNT(*)
			FROM timeseries
			WHERE timestamp >= ? AND timestamp < ? AND metric_type = ? AND metric_key = ?
		`, windowStart, windowEnd, p.metricType, p.metricKey).Scan(&avgVal, &maxVal, &minVal, &count)
		if err != nil {
			return fmt.Errorf("query aggregates: %w", err)
		}

		// P95: ORDER BY value, OFFSET count*0.95
		p95Val := avgVal // fallback
		if count > 0 {
			offset := int(math.Floor(float64(count) * 0.95))
			if offset >= count {
				offset = count - 1
			}
			err = tx.QueryRow(`
				SELECT value FROM timeseries
				WHERE timestamp >= ? AND timestamp < ? AND metric_type = ? AND metric_key = ?
				ORDER BY value ASC
				LIMIT 1 OFFSET ?
			`, windowStart, windowEnd, p.metricType, p.metricKey, offset).Scan(&p95Val)
			if err != nil {
				p95Val = maxVal // fallback to max
			}
		}

		_, err = insertStmt.Exec(windowStart, p.metricType, p.metricKey, avgVal, maxVal, minVal, p95Val, count)
		if err != nil {
			return fmt.Errorf("insert aggregated: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	if len(pairs) > 0 {
		log.Printf("Aggregated %d metric(s) into %s [%s ~ %s]", len(pairs), destTable,
			windowStart.Format("15:04:05"), windowEnd.Format("15:04:05"))
	}
	return nil
}

// aggregateFromAggregated aggregates from one aggregated table to another
func (am *AggregationManager) aggregateFromAggregated(srcTable, destTable string, windowStart, windowEnd time.Time) error {
	tx, err := am.db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	// Get distinct metric combinations
	rows, err := tx.Query(fmt.Sprintf(`
		SELECT DISTINCT metric_type, metric_key
		FROM %s
		WHERE timestamp >= ? AND timestamp < ?
	`, srcTable), windowStart, windowEnd)
	if err != nil {
		return fmt.Errorf("query distinct metrics from %s: %w", srcTable, err)
	}

	type metricPair struct {
		metricType string
		metricKey  string
	}
	var pairs []metricPair
	for rows.Next() {
		var p metricPair
		if err := rows.Scan(&p.metricType, &p.metricKey); err != nil {
			rows.Close()
			return fmt.Errorf("scan metric pair: %w", err)
		}
		pairs = append(pairs, p)
	}
	rows.Close()

	insertStmt, err := tx.Prepare(fmt.Sprintf(`
		INSERT INTO %s (timestamp, metric_type, metric_key, avg_value, max_value, min_value, p95_value, sample_count)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, destTable))
	if err != nil {
		return fmt.Errorf("prepare insert: %w", err)
	}
	defer insertStmt.Close()

	for _, p := range pairs {
		// Check if already aggregated
		var exists int
		err := tx.QueryRow(fmt.Sprintf(`
			SELECT COUNT(*) FROM %s
			WHERE timestamp = ? AND metric_type = ? AND metric_key = ?
		`, destTable), windowStart, p.metricType, p.metricKey).Scan(&exists)
		if err != nil {
			return fmt.Errorf("check exists: %w", err)
		}
		if exists > 0 {
			continue
		}

		// Weighted average using sample_count, plus MAX of max, MIN of min, MAX of p95, SUM of count
		var avgVal, maxVal, minVal, p95Val float64
		var totalCount int
		err = tx.QueryRow(fmt.Sprintf(`
			SELECT
				CASE WHEN SUM(sample_count) > 0 THEN SUM(avg_value * sample_count) / SUM(sample_count) ELSE 0 END,
				MAX(max_value),
				MIN(min_value),
				MAX(p95_value),
				SUM(sample_count)
			FROM %s
			WHERE timestamp >= ? AND timestamp < ? AND metric_type = ? AND metric_key = ?
		`, srcTable), windowStart, windowEnd, p.metricType, p.metricKey).Scan(&avgVal, &maxVal, &minVal, &p95Val, &totalCount)
		if err != nil {
			return fmt.Errorf("query aggregates from %s: %w", srcTable, err)
		}

		_, err = insertStmt.Exec(windowStart, p.metricType, p.metricKey, avgVal, maxVal, minVal, p95Val, totalCount)
		if err != nil {
			return fmt.Errorf("insert aggregated: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	if len(pairs) > 0 {
		log.Printf("Aggregated %d metric(s) into %s [%s ~ %s]", len(pairs), destTable,
			windowStart.Format("2006-01-02 15:04"), windowEnd.Format("2006-01-02 15:04"))
	}
	return nil
}

// cleanupExpired removes expired data from all tables
func (am *AggregationManager) cleanupExpired() error {
	now := time.Now()

	tables := []struct {
		name      string
		retention time.Duration
	}{
		{"timeseries", RetentionRaw},
		{"timeseries_5min", Retention5Min},
		{"timeseries_1h", Retention1H},
		{"timeseries_1d", Retention1D},
	}

	for _, t := range tables {
		cutoff := now.Add(-t.retention)
		result, err := am.db.Exec(fmt.Sprintf(`DELETE FROM %s WHERE timestamp < ?`, t.name), cutoff)
		if err != nil {
			log.Printf("Cleanup [%s] error: %v", t.name, err)
			continue
		}
		rows, _ := result.RowsAffected()
		if rows > 0 {
			log.Printf("Cleanup [%s]: removed %d expired records", t.name, rows)
		}
	}
	return nil
}

// SelectGranularity returns the appropriate table name based on the query time span.
//
//	< 6 hours   -> timeseries (raw)
//	6h ~ 3 days -> timeseries_5min
//	3d ~ 30d    -> timeseries_1h
//	> 30 days   -> timeseries_1d
func (am *AggregationManager) SelectGranularity(duration time.Duration) string {
	switch {
	case duration < 6*time.Hour:
		return "timeseries"
	case duration < 3*24*time.Hour:
		return "timeseries_5min"
	case duration < 30*24*time.Hour:
		return "timeseries_1h"
	default:
		return "timeseries_1d"
	}
}

// QueryAggregated queries data from the appropriate granularity table based on the time range.
// For raw data it returns simple points; for aggregated tables it returns full AggregatedPoint data.
func (am *AggregationManager) QueryAggregated(metricType, metricKey string, start, end time.Time) ([]AggregatedPoint, error) {
	duration := end.Sub(start)
	table := am.SelectGranularity(duration)

	if table == "timeseries" {
		return am.queryRaw(metricType, metricKey, start, end)
	}
	return am.queryAggregatedTable(table, metricType, metricKey, start, end)
}

// QueryAggregatedFromTable queries data from a specific granularity table (for manual granularity selection).
func (am *AggregationManager) QueryAggregatedFromTable(table, metricType, metricKey string, start, end time.Time) ([]AggregatedPoint, error) {
	if table == "timeseries" {
		return am.queryRaw(metricType, metricKey, start, end)
	}
	return am.queryAggregatedTable(table, metricType, metricKey, start, end)
}

// queryRaw queries raw timeseries and wraps results as AggregatedPoint
func (am *AggregationManager) queryRaw(metricType, metricKey string, start, end time.Time) ([]AggregatedPoint, error) {
	query := `
		SELECT timestamp, metric_type, metric_key, value
		FROM timeseries
		WHERE metric_type = ? AND timestamp >= ? AND timestamp <= ?
	`
	args := []interface{}{metricType, start, end}
	if metricKey != "" {
		query += ` AND metric_key = ?`
		args = append(args, metricKey)
	}
	query += ` ORDER BY timestamp ASC`

	rows, err := am.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query raw timeseries: %w", err)
	}
	defer rows.Close()

	var points []AggregatedPoint
	for rows.Next() {
		var ts time.Time
		var mt, mk string
		var val float64
		if err := rows.Scan(&ts, &mt, &mk, &val); err != nil {
			return nil, fmt.Errorf("scan raw point: %w", err)
		}
		points = append(points, AggregatedPoint{
			Timestamp:   ts,
			MetricType:  mt,
			MetricKey:   mk,
			AvgValue:    val,
			MaxValue:    val,
			MinValue:    val,
			P95Value:    val,
			SampleCount: 1,
		})
	}
	return points, nil
}

// queryAggregatedTable queries an aggregated table
func (am *AggregationManager) queryAggregatedTable(table, metricType, metricKey string, start, end time.Time) ([]AggregatedPoint, error) {
	query := fmt.Sprintf(`
		SELECT timestamp, metric_type, metric_key, avg_value, max_value, min_value, p95_value, sample_count
		FROM %s
		WHERE metric_type = ? AND timestamp >= ? AND timestamp <= ?
	`, table)
	args := []interface{}{metricType, start, end}
	if metricKey != "" {
		query += ` AND metric_key = ?`
		args = append(args, metricKey)
	}
	query += ` ORDER BY timestamp ASC`

	rows, err := am.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query %s: %w", table, err)
	}
	defer rows.Close()

	var points []AggregatedPoint
	for rows.Next() {
		var pt AggregatedPoint
		if err := rows.Scan(&pt.Timestamp, &pt.MetricType, &pt.MetricKey,
			&pt.AvgValue, &pt.MaxValue, &pt.MinValue, &pt.P95Value, &pt.SampleCount); err != nil {
			return nil, fmt.Errorf("scan aggregated point: %w", err)
		}
		points = append(points, pt)
	}
	return points, nil
}
