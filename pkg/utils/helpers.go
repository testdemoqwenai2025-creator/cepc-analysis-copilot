package utils

import (
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/rand"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ParseJSON parses a JSON byte slice into a map.
func ParseJSON(data []byte) (map[string]any, error) {
	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// HashStrings computes a SHA-256 hash of concatenated strings.
func HashStrings(strs []string) string {
	h := sha256.New()
	for _, s := range strs {
		h.Write([]byte(s))
	}
	return hex.EncodeToString(h.Sum(nil))[:16]
}

// HashBytes computes an MD5 hash of a byte slice.
func HashBytes(data []byte) string {
	h := md5.Sum(data)
	return hex.EncodeToString(h[:])
}

// ShortUUID generates a short 8-character UUID.
func ShortUUID() string {
	id := uuid.New()
	return id.String()[:8]
}

// Slugify converts a string to a URL-friendly slug.
func Slugify(s any) string {
	var str string
	switch v := s.(type) {
	case string:
		str = v
	case fmt.Stringer:
		str = v.String()
	default:
		str = fmt.Sprintf("%v", v)
	}

	// Lowercase, replace non-alphanumeric with dashes
	reg := regexp.MustCompile(`[^a-z0-9]+`)
	str = strings.ToLower(str)
	str = reg.ReplaceAllString(str, "-")
	str = strings.Trim(str, "-")
	if len(str) > 32 {
		str = str[:32]
	}
	return str
}

// FormatDuration formats milliseconds into a human-readable string.
func FormatDuration(ms int64) string {
	d := time.Duration(ms) * time.Millisecond
	if d < time.Second {
		return fmt.Sprintf("%dms", ms)
	}
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	return fmt.Sprintf("%.1fm", d.Minutes())
}

// SafeFloat64 extracts a float64 from an any, returning fallback on failure.
func SafeFloat64(v any, fallback float64) float64 {
	switch val := v.(type) {
	case float64:
		return val
	case int:
		return float64(val)
	case int64:
		return float64(val)
	default:
		return fallback
	}
}

// InitRandomSeed initializes the random number generator.
func InitRandomSeed() {
	rand.Seed(time.Now().UnixNano())
}

// Paginate returns offset and limit for SQL pagination.
func Paginate(page, pageSize int) (offset, limit int) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	return (page - 1) * pageSize, pageSize
}