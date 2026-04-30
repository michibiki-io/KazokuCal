package calendar

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type Holiday struct {
	Date string `json:"date"`
	Name string `json:"name"`
}

type HolidayService struct {
	ScriptPath string
	Timeout    time.Duration
}

func NewHolidayService(scriptPath string) HolidayService {
	if strings.TrimSpace(scriptPath) == "" {
		scriptPath = os.Getenv("HOLIDAY_SCRIPT")
	}
	if strings.TrimSpace(scriptPath) == "" {
		scriptPath = filepath.Clean("../pdfgen/list_holidays.py")
	}
	return HolidayService{ScriptPath: scriptPath, Timeout: 10 * time.Second}
}

func (s HolidayService) List(ctx context.Context, year int) ([]Holiday, error) {
	ctx, cancel := context.WithTimeout(ctx, s.Timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "python3", s.ScriptPath, fmt.Sprintf("%d", year))
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("holiday calculation timed out")
		}
		return nil, fmt.Errorf("holiday calculation failed: %w: %s", err, strings.TrimSpace(stderr.String()))
	}

	var holidays []Holiday
	if err := json.Unmarshal(stdout.Bytes(), &holidays); err != nil {
		return nil, fmt.Errorf("decode holiday output: %w", err)
	}
	return holidays, nil
}
