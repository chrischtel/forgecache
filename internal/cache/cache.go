package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bmatcuk/doublestar/v4"
)

// CacheEntry represents a cached build result
type CacheEntry struct {
	InputHash  string    `json:"input_hash"`
	OutputHash string    `json:"output_hash"`
	Timestamp  time.Time `json:"timestamp"`
	BuildCmd   string    `json:"build_cmd"`
	Success    bool      `json:"success"`
	ExitCode   int       `json:"exit_code"`
	Duration   string    `json:"duration"`
}

// Cache manages the ForgeCache storage
type Cache struct {
	cacheDir string
}

// NewCache creates a new cache instance
func NewCache(projectRoot string) *Cache {
	cacheDir := filepath.Join(projectRoot, ".forge", "cache")
	return &Cache{
		cacheDir: cacheDir,
	}
}

// Initialize creates the cache directory structure
func (c *Cache) Initialize() error {
	return os.MkdirAll(c.cacheDir, 0755)
}

// HashInputs computes a hash of all input files/directories
func (c *Cache) HashInputs(inputs []string) (string, error) {
	hasher := sha256.New()

	for _, pattern := range inputs {
		// Check if it's a glob pattern
		if strings.Contains(pattern, "*") {
			// Handle glob pattern
			matches, err := doublestar.FilepathGlob(pattern)
			if err != nil {
				return "", fmt.Errorf("glob pattern error for %s: %v", pattern, err)
			}

			for _, match := range matches {
				if err := c.hashPath(match, hasher); err != nil {
					return "", err
				}
			}
		} else {
			// Handle regular path
			if err := c.hashPath(pattern, hasher); err != nil {
				return "", err
			}
		}
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}

// hashPath recursively hashes a file or directory
func (c *Cache) hashPath(path string, hasher io.Writer) error {
	return filepath.Walk(path, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories and hidden files
		if info.IsDir() || filepath.Base(filePath)[0] == '.' {
			return nil
		}

		file, err := os.Open(filePath)
		if err != nil {
			return err
		}
		defer file.Close()

		// Write file path and content to hasher
		hasher.Write([]byte(filePath))
		_, err = io.Copy(hasher, file)
		return err
	})
}

// IsCached checks if a build result is cached for the given input hash
func (c *Cache) IsCached(inputHash string) bool {
	cacheFile := filepath.Join(c.cacheDir, inputHash+".json")
	_, err := os.Stat(cacheFile)
	return err == nil
}

// GetCacheDir returns the cache directory path
func (c *Cache) GetCacheDir() string {
	return c.cacheDir
}

// StoreCacheEntry stores a cache entry for future lookups
func (c *Cache) StoreCacheEntry(entry *CacheEntry) error {
	cacheFile := filepath.Join(c.cacheDir, entry.InputHash+".json")

	// Serialize cache entry to JSON
	data, err := json.MarshalIndent(entry, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal cache entry: %v", err)
	}

	// Write JSON to cache file
	if err := os.WriteFile(cacheFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write cache file: %v", err)
	}

	return nil
}

// LoadCacheEntry loads a cache entry from disk
func (c *Cache) LoadCacheEntry(inputHash string) (*CacheEntry, error) {
	cacheFile := filepath.Join(c.cacheDir, inputHash+".json")

	data, err := os.ReadFile(cacheFile)
	if err != nil {
		return nil, err
	}

	var entry CacheEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return nil, fmt.Errorf("failed to unmarshal cache entry: %v", err)
	}

	return &entry, nil
}

// GetCacheEntryPath returns the path for storing cached outputs
func (c *Cache) GetCacheEntryPath(inputHash string) string {
	return filepath.Join(c.cacheDir, inputHash)
}
