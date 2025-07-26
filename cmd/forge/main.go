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
	"github.com/fatih/color"
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

// Color definitions for modern CLI output
var (
	colorSuccess = color.New(color.FgGreen, color.Bold)
	colorError   = color.New(color.FgRed, color.Bold)
	colorWarning = color.New(color.FgYellow, color.Bold)
	colorInfo    = color.New(color.FgCyan, color.Bold)
	colorBuild   = color.New(color.FgMagenta, color.Bold)
	colorCache   = color.New(color.FgBlue, color.Bold)
	colorHeader  = color.New(color.FgWhite, color.Bold)
	colorDim     = color.New(color.FgHiBlack)
)

var rootCmd = &cobra.Command{
	Use:   "forge",
	Short: fmt.Sprintf("ForgeCache - Universal Dev Cache & Dependency Manager %s", version),
}

func main() {
	// Clean up old executables from previous updates
	update.CleanupOldExecutables()

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
		colorHeader.Printf("\n[FORGE VERSION]\n")
		colorInfo.Printf("ForgeCache %s\n", version)
		colorDim.Printf("Commit:      %s\n", commit)
		colorDim.Printf("Built:       %s\n", date)
		colorDim.Printf("Go version:  %s\n", goVersion)
		colorDim.Printf("OS/Arch:     %s/%s\n", buildOS, buildArch)
		colorDim.Printf("Runtime:     %s/%s\n", runtime.GOOS, runtime.GOARCH)
	},
}

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Check for and install updates",
	Long:  `Check for newer versions of ForgeCache and automatically download and install them.`,
	Run: func(cmd *cobra.Command, args []string) {
		colorHeader.Printf("\n[UPDATE CHECK]\n")
		colorDim.Printf("Checking for updates...\n\n")

		checker := update.NewUpdateChecker(version, commit)
		release, hasUpdate, err := checker.CheckForUpdates()
		if err != nil {
			colorError.Printf("Error checking for updates: %v\n", err)
			os.Exit(1)
		}

		if !hasUpdate {
			colorSuccess.Printf("You are running the latest version!\n")
			return
		}

		colorInfo.Printf("New version available: %s\n", release.TagName)
		colorDim.Printf("Released: %s\n", release.PublishedAt.Format("January 2, 2006"))
		if release.Body != "" {
			colorDim.Printf("Release notes:\n%s\n", release.Body)
		}

		// Ask for confirmation
		colorDim.Printf("\nDo you want to download and install this update? [y/N]: ")
		var response string
		fmt.Scanln(&response)

		if strings.ToLower(response) != "y" && strings.ToLower(response) != "yes" {
			colorDim.Printf("Update cancelled.\n")
			return
		}

		// Download and install
		colorInfo.Printf("\nDownloading update...\n")
		if err := checker.DownloadAndInstall(release); err != nil {
			colorError.Printf("Error installing update: %v\n", err)
			os.Exit(1)
		}
	},
}

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize ForgeCache in current directory",
	Run: func(cmd *cobra.Command, args []string) {
		colorHeader.Printf("\n[INITIALIZE]\n")
		colorDim.Printf("Initializing ForgeCache...\n\n")

		// Create cache directory structure
		c := cache.NewCache(".")
		if err := c.Initialize(); err != nil {
			colorError.Printf("Error initializing cache: %v\n", err)
			os.Exit(1)
		}

		// Create default .forgefile if it doesn't exist
		if _, err := os.Stat(".forgefile"); os.IsNotExist(err) {
			defaultConfig := config.DefaultConfig()
			if err := config.SaveConfig(defaultConfig); err != nil {
				colorError.Printf("Error creating .forgefile: %v\n", err)
				os.Exit(1)
			}
			colorSuccess.Printf("Created .forgefile with default configuration\n")
		} else {
			colorInfo.Printf(".forgefile already exists\n")
		}

		colorSuccess.Printf("ForgeCache initialized successfully!\n")
		colorDim.Printf("Edit .forgefile to configure your project\n")
	},
}

var buildCmd = &cobra.Command{
	Use:   "build",
	Short: "Build project with smart caching",
	Run: func(cmd *cobra.Command, args []string) {
		forceRebuild, _ := cmd.Flags().GetBool("force")

		colorHeader.Printf("\n[FORGE BUILD]\n")
		colorDim.Printf("Starting build process\n\n")

		// Load and validate config
		cfg, err := config.LoadConfig()
		if err != nil {
			colorError.Printf("Failed to load configuration: %v\n", err)
			colorDim.Printf("Run 'forge init' to create a .forgefile\n")
			os.Exit(1)
		}

		colorInfo.Printf("Configuration loaded successfully\n")

		// Initialize cache and executor
		c := cache.NewCache(".")
		if err := c.Initialize(); err != nil {
			colorError.Printf("Error initializing cache: %v\n", err)
			os.Exit(1)
		}
		executor := builder.NewExecutor(".")

		// Compute input hash
		colorDim.Printf("Checking cache...\n")
		inputHash, err := c.HashInputs(cfg.Cache.Inputs)
		if err != nil {
			colorError.Printf("Error computing input hash: %v\n", err)
			os.Exit(1)
		}

		colorDim.Printf("Input hash: %s...\n", inputHash[:16])

		// Check if build is cached (unless forcing rebuild)
		if !forceRebuild && c.IsCached(inputHash) {
			colorCache.Printf("Cache hit! Checking build result...\n")

			// Load cache entry to check if it was successful
			cacheEntry, err := c.LoadCacheEntry(inputHash)
			if err != nil {
				colorWarning.Printf("Warning: Could not load cache entry: %v\n", err)
			} else if !cacheEntry.Success {
				colorWarning.Printf("Previous build failed (exit code %d), rebuilding...\n", cacheEntry.ExitCode)
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
								colorSuccess.Printf("Cache hit! Build outputs are up-to-date (built %v ago)\n", time.Since(cacheEntry.Timestamp).Truncate(time.Second))
								colorDim.Printf("Note: Cannot overwrite running executable, but cache indicates no rebuild needed\n")
								return
							}
						}
					}
					colorWarning.Printf("Warning: Could not restore outputs: %v\n", err)
				} else {
					colorSuccess.Printf("Outputs restored from cache (built %v ago)\n", time.Since(cacheEntry.Timestamp).Truncate(time.Second))
				}
				return
			}
		}

		// Run build command
		colorBuild.Printf("Building project...\n")
		colorDim.Printf("Running command: %s\n", cfg.Build.Cmd)

		buildResult, err := executor.Execute(cfg.Build.Cmd)
		if err != nil {
			colorError.Printf("Error executing build command: %v\n", err)
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
			colorWarning.Printf("Warning: Could not store cache entry: %v\n", err)
		} else {
			colorCache.Printf("Build result cached for future use\n")
		}

		// Cache outputs if build was successful
		if buildResult.Success {
			cacheEntryPath := c.GetCacheEntryPath(inputHash)
			if err := executor.CopyOutputs(cfg.Cache.Outputs, cacheEntryPath); err != nil {
				colorWarning.Printf("Warning: Could not cache outputs: %v\n", err)
			} else {
				colorCache.Printf("Build outputs cached\n")
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
		colorHeader.Printf("\n[CLEAN CACHE]\n")
		colorDim.Printf("Cleaning build cache\n\n")

		c := cache.NewCache(".")
		cacheDir := c.GetCacheDir()

		// Check if cache directory exists
		if _, err := os.Stat(cacheDir); os.IsNotExist(err) {
			colorInfo.Printf("No cache found - nothing to clean\n")
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
			colorWarning.Printf("Error calculating cache size: %v\n", err)
		} else {
			colorInfo.Printf("Cache size: %.2f MB\n", float64(totalSize)/(1024*1024))
		}

		// Ask for confirmation
		colorDim.Printf("Are you sure you want to clean the cache? [y/N]: ")
		var response string
		fmt.Scanln(&response)

		if strings.ToLower(response) != "y" && strings.ToLower(response) != "yes" {
			colorDim.Printf("Clean cancelled.\n")
			return
		}

		// Remove cache directory
		if err := os.RemoveAll(cacheDir); err != nil {
			colorError.Printf("Error cleaning cache: %v\n", err)
			os.Exit(1)
		}

		colorSuccess.Printf("Cache cleaned successfully!\n")
		colorInfo.Printf("Freed %.2f MB of disk space\n", float64(totalSize)/(1024*1024))
	},
}

var fetchCmd = &cobra.Command{
	Use:   "fetch",
	Short: "Download and setup toolchain dependencies",
	Run: func(cmd *cobra.Command, args []string) {
		colorHeader.Printf("\n[FETCH DEPENDENCIES]\n")
		colorDim.Printf("Fetching toolchain dependencies...\n")
		colorWarning.Printf("TODO: Implement fetch logic\n")
	},
}

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run the built binary",
	Run: func(cmd *cobra.Command, args []string) {
		colorHeader.Printf("\n[RUN PROJECT]\n")
		colorDim.Printf("Running project...\n")
		colorWarning.Printf("TODO: Implement run logic\n")
	},
}
