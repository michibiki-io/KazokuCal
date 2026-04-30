package pdf

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

	"kazokucal/backend/internal/calendar"
)

type Service struct {
	ScriptPath string
	Timeout    time.Duration
}

func NewService(scriptPath string) Service {
	if strings.TrimSpace(scriptPath) == "" {
		scriptPath = os.Getenv("PDFGEN_SCRIPT")
	}
	if strings.TrimSpace(scriptPath) == "" {
		scriptPath = filepath.Clean("../pdfgen/generate_calendar.py")
	}
	return Service{ScriptPath: scriptPath, Timeout: 30 * time.Second}
}

func (s Service) Generate(ctx context.Context, req calendar.PDFRequest) ([]byte, error) {
	payload, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal pdf request: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, s.Timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "python3", s.ScriptPath)
	cmd.Stdin = bytes.NewReader(payload)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("pdf generation timed out")
		}
		return nil, fmt.Errorf("pdf generation failed: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	if stdout.Len() == 0 {
		return nil, fmt.Errorf("pdf generation returned empty output: %s", strings.TrimSpace(stderr.String()))
	}
	return stdout.Bytes(), nil
}
