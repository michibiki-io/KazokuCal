package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"

	"kazokucal/backend/internal/calendar"
)

type MySQLStore struct {
	db *sql.DB
}

func NewMySQLStore(ctx context.Context, dsn string) (*MySQLStore, error) {
	if strings.TrimSpace(dsn) == "" {
		return nil, errors.New("DB_DSN is required")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("open mysql: %w", err)
	}
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)

	deadline := time.Now().Add(90 * time.Second)
	for {
		if err := db.PingContext(ctx); err == nil {
			break
		} else if time.Now().After(deadline) {
			_ = db.Close()
			return nil, fmt.Errorf("ping mysql: %w", err)
		}
		select {
		case <-ctx.Done():
			_ = db.Close()
			return nil, ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}

	store := &MySQLStore{db: db}
	if err := store.migrate(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *MySQLStore) Close() error {
	return s.db.Close()
}

func (s *MySQLStore) LoadCalendar(ctx context.Context, ownerKey string, year int, month int) (calendar.PDFRequest, error) {
	result := calendar.PDFRequest{
		Year:          year,
		Month:         month,
		WeekStartsOn:  calendar.WeekStartMonday,
		Telework:      map[string]calendar.TeleworkStatus{},
		ScheduleItems: []calendar.ScheduleItem{},
		MultiDayItems: []calendar.MultiDayScheduleItem{},
	}

	var calendarID int64
	err := s.db.QueryRowContext(ctx, `
		SELECT id, week_starts_on
		FROM calendars
		WHERE owner_key = ? AND year = ? AND month = ?
	`, ownerKey, year, month).Scan(&calendarID, &result.WeekStartsOn)
	if errors.Is(err, sql.ErrNoRows) {
		calendarID = 0
	} else if err != nil {
		return result, fmt.Errorf("load calendar: %w", err)
	}

	if calendarID != 0 {
		rows, err := s.db.QueryContext(ctx, `
			SELECT date, papa, mama
			FROM telework_days
			WHERE calendar_id = ?
			ORDER BY date
		`, calendarID)
		if err != nil {
			return result, fmt.Errorf("load telework: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var day time.Time
			var status calendar.TeleworkStatus
			if err := rows.Scan(&day, &status.Papa, &status.Mama); err != nil {
				return result, fmt.Errorf("scan telework: %w", err)
			}
			result.Telework[day.Format("2006-01-02")] = status
		}
		if err := rows.Err(); err != nil {
			return result, fmt.Errorf("iterate telework: %w", err)
		}

		rows, err = s.db.QueryContext(ctx, `
			SELECT item_id, date, text, color
			FROM schedule_items
			WHERE calendar_id = ?
			ORDER BY date, sort_order, item_id
		`, calendarID)
		if err != nil {
			return result, fmt.Errorf("load schedule items: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var item calendar.ScheduleItem
			var day time.Time
			if err := rows.Scan(&item.ID, &day, &item.Text, &item.Color); err != nil {
				return result, fmt.Errorf("scan schedule item: %w", err)
			}
			item.Date = day.Format("2006-01-02")
			result.ScheduleItems = append(result.ScheduleItems, item)
		}
		if err := rows.Err(); err != nil {
			return result, fmt.Errorf("iterate schedule items: %w", err)
		}
	}

	visibleStart, visibleEnd := calendarGridBounds(year, month, result.WeekStartsOn)
	rows, err := s.db.QueryContext(ctx, `
		SELECT mdi.item_id, mdi.start_date, mdi.end_date, mdi.text, mdi.color, mdi.arrow
		FROM multi_day_items mdi
		JOIN calendars c ON c.id = mdi.calendar_id
		WHERE c.owner_key = ?
			AND mdi.start_date <= ?
			AND mdi.end_date >= ?
		ORDER BY start_date, end_date, sort_order, item_id
	`, ownerKey, visibleEnd, visibleStart)
	if err != nil {
		return result, fmt.Errorf("load multi-day items: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var item calendar.MultiDayScheduleItem
		var startDate time.Time
		var endDate time.Time
		if err := rows.Scan(&item.ID, &startDate, &endDate, &item.Text, &item.Color, &item.Arrow); err != nil {
			return result, fmt.Errorf("scan multi-day item: %w", err)
		}
		item.StartDate = startDate.Format("2006-01-02")
		item.EndDate = endDate.Format("2006-01-02")
		result.MultiDayItems = append(result.MultiDayItems, item)
	}
	if err := rows.Err(); err != nil {
		return result, fmt.Errorf("iterate multi-day items: %w", err)
	}

	return result, nil
}

func (s *MySQLStore) SaveCalendar(ctx context.Context, ownerKey string, data calendar.PDFRequest) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin save calendar: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	_, err = tx.ExecContext(ctx, `
		INSERT INTO calendars (owner_key, year, month, week_starts_on)
		VALUES (?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			week_starts_on = VALUES(week_starts_on),
			updated_at = CURRENT_TIMESTAMP
	`, ownerKey, data.Year, data.Month, data.WeekStartsOn)
	if err != nil {
		return fmt.Errorf("upsert calendar: %w", err)
	}

	var calendarID int64
	err = tx.QueryRowContext(ctx, `
		SELECT id
		FROM calendars
		WHERE owner_key = ? AND year = ? AND month = ?
	`, ownerKey, data.Year, data.Month).Scan(&calendarID)
	if err != nil {
		return fmt.Errorf("select calendar id: %w", err)
	}

	if _, err = tx.ExecContext(ctx, `DELETE FROM telework_days WHERE calendar_id = ?`, calendarID); err != nil {
		return fmt.Errorf("clear telework: %w", err)
	}
	for day, status := range data.Telework {
		if !status.Papa && !status.Mama {
			continue
		}
		if _, err = tx.ExecContext(ctx, `
			INSERT INTO telework_days (calendar_id, date, papa, mama)
			VALUES (?, ?, ?, ?)
		`, calendarID, day, status.Papa, status.Mama); err != nil {
			return fmt.Errorf("insert telework %s: %w", day, err)
		}
	}

	if _, err = tx.ExecContext(ctx, `DELETE FROM schedule_items WHERE calendar_id = ?`, calendarID); err != nil {
		return fmt.Errorf("clear schedule items: %w", err)
	}
	for i, item := range data.ScheduleItems {
		if _, err = tx.ExecContext(ctx, `
			INSERT INTO schedule_items (calendar_id, item_id, date, text, color, sort_order)
			VALUES (?, ?, ?, ?, ?, ?)
		`, calendarID, item.ID, item.Date, item.Text, item.Color, i); err != nil {
			return fmt.Errorf("insert schedule item %s: %w", item.ID, err)
		}
	}

	if err = s.saveMultiDayItems(ctx, tx, ownerKey, data); err != nil {
		return err
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit save calendar: %w", err)
	}
	return nil
}

func (s *MySQLStore) saveMultiDayItems(ctx context.Context, tx *sql.Tx, ownerKey string, data calendar.PDFRequest) error {
	visibleStart, visibleEnd := calendarGridBounds(data.Year, data.Month, data.WeekStartsOn)
	incomingIDs := make(map[string]struct{}, len(data.MultiDayItems))
	for _, item := range data.MultiDayItems {
		incomingIDs[item.ID] = struct{}{}
	}

	rows, err := tx.QueryContext(ctx, `
		SELECT mdi.item_id
		FROM multi_day_items mdi
		JOIN calendars c ON c.id = mdi.calendar_id
		WHERE c.owner_key = ?
			AND mdi.start_date <= ?
			AND mdi.end_date >= ?
	`, ownerKey, visibleEnd, visibleStart)
	if err != nil {
		return fmt.Errorf("load overlapping multi-day items for save: %w", err)
	}
	var staleIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			_ = rows.Close()
			return fmt.Errorf("scan overlapping multi-day item: %w", err)
		}
		if _, ok := incomingIDs[id]; !ok {
			staleIDs = append(staleIDs, id)
		}
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("close overlapping multi-day items: %w", err)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate overlapping multi-day items: %w", err)
	}

	for _, id := range staleIDs {
		if _, err := tx.ExecContext(ctx, `
			DELETE mdi
			FROM multi_day_items mdi
			JOIN calendars c ON c.id = mdi.calendar_id
			WHERE c.owner_key = ? AND mdi.item_id = ?
		`, ownerKey, id); err != nil {
			return fmt.Errorf("delete removed multi-day item %s: %w", id, err)
		}
	}

	for i, item := range data.MultiDayItems {
		startDate, err := time.Parse("2006-01-02", item.StartDate)
		if err != nil {
			return fmt.Errorf("parse multi-day start date %s: %w", item.StartDate, err)
		}
		homeCalendarID, err := ensureCalendar(ctx, tx, ownerKey, startDate.Year(), int(startDate.Month()), data.WeekStartsOn)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
			DELETE mdi
			FROM multi_day_items mdi
			JOIN calendars c ON c.id = mdi.calendar_id
			WHERE c.owner_key = ? AND mdi.item_id = ?
		`, ownerKey, item.ID); err != nil {
			return fmt.Errorf("replace multi-day item %s: %w", item.ID, err)
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO multi_day_items (calendar_id, item_id, start_date, end_date, text, color, arrow, sort_order)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		`, homeCalendarID, item.ID, item.StartDate, item.EndDate, item.Text, item.Color, item.Arrow, i); err != nil {
			return fmt.Errorf("insert multi-day item %s: %w", item.ID, err)
		}
	}
	return nil
}

func ensureCalendar(ctx context.Context, tx *sql.Tx, ownerKey string, year int, month int, weekStartsOn calendar.WeekStart) (int64, error) {
	if weekStartsOn == "" {
		weekStartsOn = calendar.WeekStartMonday
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO calendars (owner_key, year, month, week_starts_on)
		VALUES (?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			updated_at = CURRENT_TIMESTAMP
	`, ownerKey, year, month, weekStartsOn); err != nil {
		return 0, fmt.Errorf("ensure calendar %04d-%02d: %w", year, month, err)
	}
	var calendarID int64
	if err := tx.QueryRowContext(ctx, `
		SELECT id
		FROM calendars
		WHERE owner_key = ? AND year = ? AND month = ?
	`, ownerKey, year, month).Scan(&calendarID); err != nil {
		return 0, fmt.Errorf("select ensured calendar %04d-%02d: %w", year, month, err)
	}
	return calendarID, nil
}

func calendarGridBounds(year int, month int, weekStartsOn calendar.WeekStart) (string, string) {
	first := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	last := first.AddDate(0, 1, -1)
	startDay := time.Monday
	if weekStartsOn == calendar.WeekStartSunday {
		startDay = time.Sunday
	}
	offset := (int(first.Weekday()) - int(startDay) + 7) % 7
	start := first.AddDate(0, 0, -offset)
	dayCount := int(last.Sub(start).Hours()/24) + 1
	weeks := (dayCount + 6) / 7
	end := start.AddDate(0, 0, weeks*7-1)
	return start.Format("2006-01-02"), end.Format("2006-01-02")
}

func (s *MySQLStore) migrate(ctx context.Context) error {
	for _, statement := range schemaStatements {
		if _, err := s.db.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("migrate schema: %w", err)
		}
	}
	return nil
}

var schemaStatements = []string{
	`CREATE TABLE IF NOT EXISTS calendars (
		id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
		owner_key VARCHAR(191) NOT NULL,
		year SMALLINT NOT NULL,
		month TINYINT NOT NULL,
		week_starts_on ENUM('monday', 'sunday') NOT NULL DEFAULT 'monday',
		created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
		PRIMARY KEY (id),
		UNIQUE KEY ux_calendars_owner_month (owner_key, year, month)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
	`CREATE TABLE IF NOT EXISTS telework_days (
		calendar_id BIGINT UNSIGNED NOT NULL,
		date DATE NOT NULL,
		papa BOOLEAN NOT NULL DEFAULT FALSE,
		mama BOOLEAN NOT NULL DEFAULT FALSE,
		PRIMARY KEY (calendar_id, date),
		CONSTRAINT fk_telework_calendar
			FOREIGN KEY (calendar_id) REFERENCES calendars(id)
			ON DELETE CASCADE
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
	`CREATE TABLE IF NOT EXISTS schedule_items (
		calendar_id BIGINT UNSIGNED NOT NULL,
		item_id VARCHAR(64) NOT NULL,
		date DATE NOT NULL,
		text VARCHAR(255) NOT NULL,
		color ENUM('black', 'red', 'blue') NOT NULL DEFAULT 'black',
		sort_order INT NOT NULL DEFAULT 0,
		PRIMARY KEY (calendar_id, item_id),
		KEY ix_schedule_date (calendar_id, date),
		CONSTRAINT fk_schedule_calendar
			FOREIGN KEY (calendar_id) REFERENCES calendars(id)
			ON DELETE CASCADE
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
	`CREATE TABLE IF NOT EXISTS multi_day_items (
		calendar_id BIGINT UNSIGNED NOT NULL,
		item_id VARCHAR(64) NOT NULL,
		start_date DATE NOT NULL,
		end_date DATE NOT NULL,
		text VARCHAR(255) NOT NULL,
		color ENUM('black', 'red', 'blue') NOT NULL DEFAULT 'black',
		arrow BOOLEAN NOT NULL DEFAULT TRUE,
		sort_order INT NOT NULL DEFAULT 0,
		PRIMARY KEY (calendar_id, item_id),
		KEY ix_multi_day_range (calendar_id, start_date, end_date),
		CONSTRAINT fk_multi_day_calendar
			FOREIGN KEY (calendar_id) REFERENCES calendars(id)
			ON DELETE CASCADE
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
}
