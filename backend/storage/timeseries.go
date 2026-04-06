package storage

import (
	"log"
	"time"
)

// TimeseriesWriter handles batch writes of timeseries data
type TimeseriesWriter struct {
	db        *Database
	batch     []TimeseriesPoint
	batchSize int
	ticker    *time.Ticker
	done      chan bool
}

// TimeseriesPoint represents a single timeseries data point
type TimeseriesPoint struct {
	Timestamp  time.Time
	MetricType string
	MetricKey  string
	Value      float64
}

// NewTimeseriesWriter creates a new timeseries writer with batch support
func NewTimeseriesWriter(db *Database, flushInterval time.Duration, batchSize int) *TimeseriesWriter {
	tw := &TimeseriesWriter{
		db:        db,
		batch:     make([]TimeseriesPoint, 0, batchSize),
		batchSize: batchSize,
		ticker:    time.NewTicker(flushInterval),
		done:      make(chan bool),
	}

	// Start flush loop
	go tw.flushLoop()

	return tw
}

// Add adds a data point to the batch
func (tw *TimeseriesWriter) Add(point TimeseriesPoint) {
	tw.batch = append(tw.batch, point)

	// Flush if batch is full
	if len(tw.batch) >= tw.batchSize {
		tw.flush()
	}
}

// Close stops the writer and flushes remaining data
func (tw *TimeseriesWriter) Close() {
	tw.done <- true
	tw.ticker.Stop()
	tw.flush()
}

// flushLoop periodically flushes the batch
func (tw *TimeseriesWriter) flushLoop() {
	for {
		select {
		case <-tw.ticker.C:
			tw.flush()
		case <-tw.done:
			return
		}
	}
}

// flush writes the batch to database
func (tw *TimeseriesWriter) flush() {
	if len(tw.batch) == 0 {
		return
	}

	tx, err := tw.db.db.Begin()
	if err != nil {
		log.Printf("Failed to begin transaction: %v", err)
		return
	}

	stmt, err := tx.Prepare(`
		INSERT INTO timeseries (timestamp, metric_type, metric_key, value)
		VALUES (?, ?, ?, ?)
	`)
	if err != nil {
		log.Printf("Failed to prepare statement: %v", err)
		tx.Rollback()
		return
	}
	defer stmt.Close()

	for _, point := range tw.batch {
		_, err := stmt.Exec(point.Timestamp, point.MetricType, point.MetricKey, point.Value)
		if err != nil {
			log.Printf("Failed to insert timeseries point: %v", err)
		}
	}

	if err := tx.Commit(); err != nil {
		log.Printf("Failed to commit transaction: %v", err)
		return
	}

	tw.batch = tw.batch[:0] // Clear batch
}

// CleanupOldData removes timeseries data older than retention period
func (d *Database) CleanupOldData(retentionHours int) error {
	cutoff := time.Now().Add(time.Duration(-retentionHours) * time.Hour)

	result, err := d.db.Exec(`
		DELETE FROM timeseries
		WHERE timestamp < ?
	`, cutoff)

	if err != nil {
		return err
	}

	rows, _ := result.RowsAffected()
	log.Printf("Cleaned up %d old timeseries records", rows)

	return nil
}

// StartCleanupLoop starts a periodic cleanup task
func (d *Database) StartCleanupLoop(retentionHours int, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for range ticker.C {
			if err := d.CleanupOldData(retentionHours); err != nil {
				log.Printf("Failed to cleanup old data: %v", err)
			}
		}
	}()
}
