package server

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"kazokucal/backend/internal/auth"
	"kazokucal/backend/internal/calendar"
	pdfsvc "kazokucal/backend/internal/pdf"
	"kazokucal/backend/internal/storage"
)

type Config struct {
	Port          string
	StaticDir     string
	PDFScriptPath string
	HolidayScript string
	DBDSN         string
	BasePath      string
	Auth          auth.Config
}

func ConfigFromEnv() Config {
	return Config{
		Port:          envOrDefault("PORT", "8080"),
		StaticDir:     envOrDefault("STATIC_DIR", "/app/static"),
		PDFScriptPath: envOrDefault("PDFGEN_SCRIPT", "/app/pdfgen/generate_calendar.py"),
		HolidayScript: envOrDefault("HOLIDAY_SCRIPT", "/app/pdfgen/list_holidays.py"),
		DBDSN:         os.Getenv("DB_DSN"),
		BasePath:      normalizeBasePath(envOrDefault("APP_BASE_PATH", os.Getenv("BASE_PATH"))),
		Auth:          auth.ConfigFromEnv(),
	}
}

func NewRouter(cfg Config) *gin.Engine {
	gin.SetMode(envOrDefault("GIN_MODE", gin.ReleaseMode))
	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery())

	basePath := normalizeBasePath(cfg.BasePath)
	api := router.Group(basePath + "/api")
	api.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	api.Use(auth.Middleware(cfg.Auth))
	api.GET("/me", func(c *gin.Context) {
		c.JSON(http.StatusOK, auth.ContextUser(c, cfg.Auth))
	})

	dbCtx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	calendarStore, err := storage.NewMySQLStore(dbCtx, cfg.DBDSN)
	if err != nil {
		log.Panicf("initialize calendar storage: %v", err)
	}

	holidayService := calendar.NewHolidayService(cfg.HolidayScript)
	api.GET("/holidays", func(c *gin.Context) {
		var query struct {
			Year int `form:"year" binding:"required"`
		}
		if err := c.ShouldBindQuery(&query); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "year query parameter is required"})
			return
		}
		if query.Year < 1900 || query.Year > 2100 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "year must be between 1900 and 2100"})
			return
		}
		holidays, err := holidayService.List(c.Request.Context(), query.Year)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"holidays": holidays})
	})

	api.GET("/calendar", func(c *gin.Context) {
		var query struct {
			Year  int `form:"year" binding:"required"`
			Month int `form:"month" binding:"required"`
		}
		if err := c.ShouldBindQuery(&query); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "year and month query parameters are required"})
			return
		}
		if query.Year < 1900 || query.Year > 2100 || query.Month < 1 || query.Month > 12 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "year must be 1900-2100 and month must be 1-12"})
			return
		}
		data, err := calendarStore.LoadCalendar(c.Request.Context(), ownerKey(c, cfg.Auth), query.Year, query.Month)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, data)
	})

	api.PUT("/calendar", func(c *gin.Context) {
		var req calendar.PDFRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON", "details": err.Error()})
			return
		}
		normalizeCalendarData(&req)
		if errs := calendar.ValidatePDFRequest(req); len(errs) > 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "validation failed", "details": errs})
			return
		}
		if err := calendarStore.SaveCalendar(c.Request.Context(), ownerKey(c, cfg.Auth), req); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"saved": true})
	})

	pdfService := pdfsvc.NewService(cfg.PDFScriptPath)
	api.POST("/pdf", func(c *gin.Context) {
		var req calendar.PDFRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON", "details": err.Error()})
			return
		}
		normalizeCalendarData(&req)
		if errs := calendar.ValidatePDFRequest(req); len(errs) > 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "validation failed", "details": errs})
			return
		}
		pdfBytes, err := pdfService.Generate(c.Request.Context(), req)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.Header("Content-Type", "application/pdf")
		c.Header("Content-Disposition", attachmentName(req.Year, req.Month))
		c.Data(http.StatusOK, "application/pdf", pdfBytes)
	})

	staticDir := cfg.StaticDir
	router.NoRoute(func(c *gin.Context) {
		rawPath := c.Request.URL.Path
		requestPath := filepath.ToSlash(filepath.Clean(rawPath))
		if basePath != "" {
			if rawPath == "/" {
				c.Redirect(http.StatusMovedPermanently, basePath+"/")
				return
			}
			if rawPath == basePath {
				c.Redirect(http.StatusMovedPermanently, basePath+"/")
				return
			}
			if requestPath != basePath && !strings.HasPrefix(requestPath, basePath+"/") {
				c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
				return
			}
		}
		if cfg.Auth.Enabled {
			info := auth.InfoFromRequest(c.Request, cfg.Auth)
			if info.User == "" {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
				return
			}
		}
		serveSPA(c, staticDir, basePath)
	})

	return router
}

func serveSPA(c *gin.Context, staticDir string, basePath string) {
	requestPath := filepath.ToSlash(filepath.Clean(c.Request.URL.Path))
	apiPrefix := basePath + "/api/"
	if requestPath == basePath+"/api" || strings.HasPrefix(requestPath, apiPrefix) {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	staticPath := strings.TrimPrefix(requestPath, basePath)
	target := filepath.Join(staticDir, strings.TrimPrefix(staticPath, "/"))
	if info, err := os.Stat(target); err == nil && !info.IsDir() {
		c.File(target)
		return
	}
	index := filepath.Join(staticDir, "index.html")
	if _, err := os.Stat(index); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			c.String(http.StatusNotFound, "frontend build not found")
			return
		}
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	c.File(index)
}

func attachmentName(year int, month int) string {
	return `attachment; filename="calendar-` + formatYearMonth(year, month) + `.pdf"`
}

func formatYearMonth(year int, month int) string {
	return fmt.Sprintf("%04d-%02d", year, month)
}

func normalizeCalendarData(req *calendar.PDFRequest) {
	if req.WeekStartsOn == "" {
		req.WeekStartsOn = calendar.WeekStartMonday
	}
	if req.Telework == nil {
		req.Telework = map[string]calendar.TeleworkStatus{}
	}
	if req.ScheduleItems == nil {
		req.ScheduleItems = []calendar.ScheduleItem{}
	}
	if req.MultiDayItems == nil {
		req.MultiDayItems = []calendar.MultiDayScheduleItem{}
	}
}

func ownerKey(c *gin.Context, cfg auth.Config) string {
	info := auth.ContextUser(c, cfg)
	if info.User != "" {
		return info.User
	}
	if info.Email != "" {
		return info.Email
	}
	return "default"
}

func normalizeBasePath(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || value == "/" {
		return ""
	}
	value = "/" + strings.Trim(value, "/")
	return filepath.ToSlash(filepath.Clean(value))
}

func envOrDefault(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}
