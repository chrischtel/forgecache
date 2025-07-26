package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/chrischtel/forgecache/internal/cache"
	"github.com/chrischtel/forgecache/internal/config"
	"github.com/chrischtel/forgecache/internal/update"
	"github.com/chrischtel/forgecache/pkg/builder"
	"github.com/spf13/cobra"
)

// Build-time variables injected via ldflags
var (
	version   = "dev"     // Version string (e.g., "v1.0.0" or "latest-dev")
	commit    = "unknown" // Git commit hash
	date      = "unknown" // Build date (RFC3339 format)
	goVersion = "unknown" // Go version used for build
	buildOS   = "unknown" // OS the binary was built on
	buildArch = "unknown" // Architecture the binary was built for
)

var rootCmd = &cobra.Command{
	Use:   "forge",
	Short: fmt.Sprintf("ForgeCache - Universal Dev Cache & Dependency Manager %s", version),
	Long: `ForgeCache is a cross-language, cross-project dev tool that helps you:
- Cache and re-use build artifacts across runs and machines
- Manage language toolchains (like Go 1.21, Rust nightly, etc.)
- Track inputs/outputs to only rebuild when truly needed
- Keep all projects reproducible and fast`,
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	// Add subcommands
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(updateCmd)
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(buildCmd)
	rootCmd.AddCommand(cleanCmd)
	rootCmd.AddCommand(fetchCmd)
	rootCmd.AddCommand(runCmd)
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("ForgeCache %s\n", version)
		fmt.Printf("Commit:      %s\n", commit)
		fmt.Printf("Built:       %s\n", date)
		fmt.Printf("Go version:  %s\n", goVersion)
		fmt.Printf("OS/Arch:     %s/%s\n", buildOS, buildArch)
		fmt.Printf("Runtime:     %s/%s\n", runtime.GOOS, runtime.GOARCH)
	},
}

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Check for and install updates",
	Long:  `Check for newer versions of ForgeCache and automatically download and install them.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Checking for updates...")

		checker := update.NewUpdateChecker(version, commit)
		release, hasUpdate, err := checker.CheckForUpdates()
		if err != nil {
			fmt.Printf("Error checking for updates: %v\n", err)
			os.Exit(1)
		}

		if !hasUpdate {
			fmt.Println("✅ You are running the latest version!")
			return
		}

		fmt.Printf("🎉 New version available: %s\n", release.TagName)
		fmt.Printf("📅 Released: %s\n", release.PublishedAt.Format("January 2, 2006"))
		if release.Body != "" {
			fmt.Printf("📝 Release notes:\n%s\n", release.Body)
		}

		// Ask for confirmation
		fmt.Print("\nDo you want to download and install this update? [y/N]: ")
		var response string
		fmt.Scanln(&response)

		if strings.ToLower(response) != "y" && strings.ToLower(response) != "yes" {
			fmt.Println("Update cancelled.")
			return
		}

		// Download and install
		fmt.Println("\nDownloading update...")
		if err := checker.DownloadAndInstall(release); err != nil {
			fmt.Printf("Error installing update: %v\n", err)
			os.Exit(1)
		}
	},
}

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize ForgeCache in current directory",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Initializing ForgeCache...")

		// Create cache directory structure
		c := cache.NewCache(".")
		if err := c.Initialize(); err != nil {
			fmt.Printf("Error initializing cache: %v\n", err)
			os.Exit(1)
		}

		// Create default .forgefile if it doesn't exist
		if _, err := os.Stat(".forgefile"); os.IsNotExist(err) {
			defaultConfig := config.DefaultConfig()
			if err := config.SaveConfig(defaultConfig); err != nil {
				fmt.Printf("Error creating .forgefile: %v\n", err)
				os.Exit(1)
			}
			fmt.Println("Created .forgefile with default configuration")
		} else {
			fmt.Println(".forgefile already exists")
		}

		fmt.Println("ForgeCache initialized successfully!")
		fmt.Println("Edit .forgefile to configure your project")
	},
}

var buildCmd = &cobra.Command{
	Use:   "build",
	Short: "Build project with smart caching",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Building with ForgeCache...")

		// Load configuration
		cfg, err := config.LoadConfig()
		if err != nil {
			fmt.Printf("Error loading .forgefile: %v\n", err)
			fmt.Println("Run 'forge init' to create a .forgefile")
			os.Exit(1)
		}

		// Initialize cache and executor
		c := cache.NewCache(".")
		if err := c.Initialize(); err != nil {
			fmt.Printf("Error initializing cache: %v\n", err)
			os.Exit(1)
		}
		executor := builder.NewExecutor(".")

		// Compute input hash
		inputHash, err := c.HashInputs(cfg.Cache.Inputs)
		if err != nil {
			fmt.Printf("Error computing input hash: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Input hash: %s\n", inputHash[:16]+"...")

		// Check if build is cached
		if c.IsCached(inputHash) {
			fmt.Println("✅ Build result found in cache, restoring outputs...")

			// Load cache entry to check if it was successful
			cacheEntry, err := c.LoadCacheEntry(inputHash)
			if err != nil {
				fmt.Printf("Warning: Could not load cache entry: %v\n", err)
			} else if !cacheEntry.Success {
				fmt.Printf("⚠️  Previous build failed (exit code %d), rebuilding...\n", cacheEntry.ExitCode)
			} else {
				// Restore outputs from cache
				cacheEntryPath := c.GetCacheEntryPath(inputHash)
				if err := executor.RestoreOutputs(cfg.Cache.Outputs, cacheEntryPath); err != nil {
					// Check if the error is due to trying to overwrite the running executable
					currentExe, exeErr := os.Executable()
					if exeErr == nil {
						for _, output := range cfg.Cache.Outputs {
							absOutput, _ := filepath.Abs(output)
							absExe, _ := filepath.Abs(currentExe)
							if absOutput == absExe {
								fmt.Printf("📦 Cache hit! Build outputs are up-to-date (built %v ago)\n", time.Since(cacheEntry.Timestamp).Truncate(time.Second))
								fmt.Println("💡 Note: Cannot overwrite running executable, but cache indicates no rebuild needed")
								return
							}
						}
					}
					fmt.Printf("Warning: Could not restore outputs: %v\n", err)
				} else {
					fmt.Printf("📦 Outputs restored from cache (built %v ago)\n", time.Since(cacheEntry.Timestamp).Truncate(time.Second))
				}
				return
			}
		}

		// Run build command
		fmt.Printf("🔨 Running build command: %s\n", cfg.Build.Cmd)

		buildResult, err := executor.Execute(cfg.Build.Cmd)
		if err != nil {
			fmt.Printf("Error executing build command: %v\n", err)
			os.Exit(1)
		}

		// Store cache entry
		cacheEntry := &cache.CacheEntry{
			InputHash: inputHash,
			BuildCmd:  cfg.Build.Cmd,
			Timestamp: time.Now(),
			Success:   buildResult.Success,
			ExitCode:  buildResult.ExitCode,
			Duration:  buildResult.Duration.String(),
		}

		if err := c.StoreCacheEntry(cacheEntry); err != nil {
			fmt.Printf("Warning: Could not store cache entry: %v\n", err)
		} else {
			fmt.Println("💾 Build result cached for future use")
		}

		// Cache outputs if build was successful
		if buildResult.Success {
			cacheEntryPath := c.GetCacheEntryPath(inputHash)
			if err := executor.CopyOutputs(cfg.Cache.Outputs, cacheEntryPath); err != nil {
				fmt.Printf("Warning: Could not cache outputs: %v\n", err)
			} else {
				fmt.Println("📦 Build outputs cached")
			}
		}

		// Exit with same code as build command
		if !buildResult.Success {
			os.Exit(buildResult.ExitCode)
		}
	},
}

var cleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Clean build cache and outputs",
	Long:  `Remove cached build results and optionally clean output files.`,
	Run: func(cmd *cobra.Command, args []string) {
		c := cache.NewCache(".")
		cacheDir := c.GetCacheDir()

		// Check if cache directory exists
		if _, err := os.Stat(cacheDir); os.IsNotExist(err) {
			fmt.Println("No cache found - nothing to clean")
			return
		}

		// Get cache directory size
		var totalSize int64
		err := filepath.Walk(cacheDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.IsDir() {
				totalSize += info.Size()
			}
			return nil
		})

		if err != nil {
			fmt.Printf("Error calculating cache size: %v\n", err)
		} else {
			fmt.Printf("Cache size: %.2f MB\n", float64(totalSize)/(1024*1024))
		}

		// Ask for confirmation
		fmt.Print("Are you sure you want to clean the cache? [y/N]: ")
		var response string
		fmt.Scanln(&response)

		if strings.ToLower(response) != "y" && strings.ToLower(response) != "yes" {
			fmt.Println("Clean cancelled.")
			return
		}

		// Remove cache directory
		if err := os.RemoveAll(cacheDir); err != nil {
			fmt.Printf("Error cleaning cache: %v\n", err)
			os.Exit(1)
		}

		fmt.Println("✅ Cache cleaned successfully!")
		fmt.Printf("Freed %.2f MB of disk space\n", float64(totalSize)/(1024*1024))
	},
}

var fetchCmd = &cobra.Command{
	Use:   "fetch",
	Short: "Download and setup toolchain dependencies",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Fetching toolchain dependencies...")
		// TODO: Implement fetch logic
	},
}

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run the built binary",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Running project...")
		// TODO: Implement run logic
	},
}
