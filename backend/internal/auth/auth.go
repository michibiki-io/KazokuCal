package auth

import (
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
)

type Config struct {
	Enabled      bool
	UserHeader   string
	EmailHeader  string
	GroupsHeader string
	AuthorizedGroups map[string]struct{}
}

type UserInfo struct {
	Authenticated bool     `json:"authenticated"`
	User          string   `json:"user,omitempty"`
	Email         string   `json:"email,omitempty"`
	Groups        []string `json:"groups"`
}

func ConfigFromEnv() Config {
	return Config{
		Enabled:          strings.EqualFold(os.Getenv("AUTH_ENABLED"), "true"),
		UserHeader:       envOrDefault("AUTH_USER_HEADER", "X-Forwarded-User"),
		EmailHeader:      envOrDefault("AUTH_EMAIL_HEADER", "X-Forwarded-Email"),
		GroupsHeader:     envOrDefault("AUTH_GROUPS_HEADER", "X-Forwarded-Groups"),
		AuthorizedGroups: parseAuthorizedGroups(os.Getenv("AUTHORIZED_GROUPS")),
	}
}

func Middleware(cfg Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		info := InfoFromRequest(c.Request, cfg)
		if cfg.Enabled && info.User == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
			return
		}
		c.Set("userInfo", info)
		c.Next()
	}
}

func InfoFromRequest(r *http.Request, cfg Config) UserInfo {
	user := strings.TrimSpace(r.Header.Get(cfg.UserHeader))
	email := strings.TrimSpace(r.Header.Get(cfg.EmailHeader))
	return UserInfo{
		Authenticated: user != "",
		User:          user,
		Email:         email,
		Groups:        filterAuthorizedGroups(splitGroups(r.Header.Get(cfg.GroupsHeader)), cfg.AuthorizedGroups),
	}
}

func ContextUser(c *gin.Context, cfg Config) UserInfo {
	if value, ok := c.Get("userInfo"); ok {
		if info, ok := value.(UserInfo); ok {
			if !cfg.Enabled && info.User == "" {
				info.Authenticated = false
			}
			return info
		}
	}
	return InfoFromRequest(c.Request, cfg)
}

func splitGroups(value string) []string {
	if strings.TrimSpace(value) == "" {
		return []string{}
	}
	parts := strings.FieldsFunc(value, func(r rune) bool { return r == ',' || r == ';' })
	groups := make([]string, 0, len(parts))
	seen := map[string]struct{}{}
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			if _, ok := seen[trimmed]; ok {
				continue
			}
			seen[trimmed] = struct{}{}
			groups = append(groups, trimmed)
		}
	}
	return groups
}

func parseAuthorizedGroups(value string) map[string]struct{} {
	groups := splitGroups(value)
	if len(groups) == 0 {
		return nil
	}
	allowed := make(map[string]struct{}, len(groups))
	for _, group := range groups {
		allowed[group] = struct{}{}
	}
	return allowed
}

func filterAuthorizedGroups(groups []string, allowed map[string]struct{}) []string {
	if len(groups) == 0 {
		return []string{}
	}
	if len(allowed) == 0 {
		return groups
	}
	filtered := make([]string, 0, len(groups))
	for _, group := range groups {
		if _, ok := allowed[group]; ok {
			filtered = append(filtered, group)
		}
	}
	return filtered
}

func envOrDefault(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}
