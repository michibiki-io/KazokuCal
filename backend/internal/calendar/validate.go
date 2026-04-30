package calendar

import (
	"fmt"
	"strings"
	"time"
)

func ValidatePDFRequest(req PDFRequest) []ValidationError {
	var errs []ValidationError
	if req.Year < 1900 || req.Year > 2100 {
		errs = append(errs, ValidationError{Field: "year", Message: "year must be between 1900 and 2100"})
	}
	if req.Month < 1 || req.Month > 12 {
		errs = append(errs, ValidationError{Field: "month", Message: "month must be between 1 and 12"})
	}
	if req.WeekStartsOn == "" {
		req.WeekStartsOn = WeekStartMonday
	}
	if req.WeekStartsOn != WeekStartMonday && req.WeekStartsOn != WeekStartSunday {
		errs = append(errs, ValidationError{Field: "weekStartsOn", Message: "weekStartsOn must be monday or sunday"})
	}

	for date := range req.Telework {
		if _, err := parseDate(date); err != nil {
			errs = append(errs, ValidationError{Field: "telework." + date, Message: "date must be YYYY-MM-DD"})
		}
	}

	for i, item := range req.ScheduleItems {
		prefix := fmt.Sprintf("scheduleItems[%d]", i)
		if strings.TrimSpace(item.ID) == "" {
			errs = append(errs, ValidationError{Field: prefix + ".id", Message: "id is required"})
		}
		if _, err := parseDate(item.Date); err != nil {
			errs = append(errs, ValidationError{Field: prefix + ".date", Message: "date must be YYYY-MM-DD"})
		}
		if l := len([]rune(item.Text)); l == 0 || l > 120 {
			errs = append(errs, ValidationError{Field: prefix + ".text", Message: "text length must be 1 to 120 characters"})
		}
		if !validColor(item.Color) {
			errs = append(errs, ValidationError{Field: prefix + ".color", Message: "color must be black, red, or blue"})
		}
	}

	for i, item := range req.MultiDayItems {
		prefix := fmt.Sprintf("multiDayItems[%d]", i)
		if strings.TrimSpace(item.ID) == "" {
			errs = append(errs, ValidationError{Field: prefix + ".id", Message: "id is required"})
		}
		start, startErr := parseDate(item.StartDate)
		end, endErr := parseDate(item.EndDate)
		if startErr != nil {
			errs = append(errs, ValidationError{Field: prefix + ".startDate", Message: "date must be YYYY-MM-DD"})
		}
		if endErr != nil {
			errs = append(errs, ValidationError{Field: prefix + ".endDate", Message: "date must be YYYY-MM-DD"})
		}
		if startErr == nil && endErr == nil && start.After(end) {
			errs = append(errs, ValidationError{Field: prefix + ".endDate", Message: "endDate must be on or after startDate"})
		}
		if len([]rune(item.Text)) > 80 {
			errs = append(errs, ValidationError{Field: prefix + ".text", Message: "text length must be 80 characters or less"})
		}
		if !validColor(item.Color) {
			errs = append(errs, ValidationError{Field: prefix + ".color", Message: "color must be black, red, or blue"})
		}
	}

	return errs
}

func validColor(color Color) bool {
	return color == ColorBlack || color == ColorRed || color == ColorBlue
}

func parseDate(value string) (time.Time, error) {
	return time.Parse("2006-01-02", value)
}
