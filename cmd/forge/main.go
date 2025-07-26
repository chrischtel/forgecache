package main

import (
	"fmt"
	"os"
	"time"

	"github.com/chrischtel/forgecache/internal/cache"
	"github.com/chrischtel/forgecache/internal/config"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "forge",
	Short: "ForgeCache - Universal Dev Cache & Dependency Manager",
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
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(buildCmd)
	rootCmd.AddCommand(fetchCmd)
	rootCmd.AddCommand(runCmd)
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

		// Initialize cache
		c := cache.NewCache(".")

		// Compute input hash
		inputHash, err := c.HashInputs(cfg.Cache.Inputs)
		if err != nil {
			fmt.Printf("Error computing input hash: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Input hash: %s\n", inputHash[:16]+"...")

		// Check if build is cached
		if c.IsCached(inputHash) {
			fmt.Println("✅ Build result found in cache, skipping build")
			return
		}

		// Run build command
		fmt.Printf("🔨 Running build command: %s\n", cfg.Build.Cmd)

		// TODO: Actually execute the build command
		// For now, just simulate it
		fmt.Println("Build completed successfully!")

		// Store cache entry
		cacheEntry := &cache.CacheEntry{
			InputHash: inputHash,
			BuildCmd:  cfg.Build.Cmd,
			Timestamp: time.Now(),
		}

		if err := c.StoreCacheEntry(cacheEntry); err != nil {
			fmt.Printf("Warning: Could not store cache entry: %v\n", err)
		} else {
			fmt.Println("💾 Build result cached for future use")
		}
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
