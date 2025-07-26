package builder

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/fatih/color"
)

// BuildResult represents the result of a build execution
type BuildResult struct {
	Success   bool
	ExitCode  int
	Output    string
	Error     string
	Duration  time.Duration
	Timestamp time.Time
}

// Executor handles running build commands
type Executor struct {
	workingDir string
}

// NewExecutor creates a new command executor
func NewExecutor(workingDir string) *Executor {
	return &Executor{
		workingDir: workingDir,
	}
}

// Execute runs a build command and returns the result
func (e *Executor) Execute(command string) (*BuildResult, error) {
	startTime := time.Now()

	colorBuild := color.New(color.FgMagenta, color.Bold)
	colorSuccess := color.New(color.FgGreen, color.Bold)
	colorError := color.New(color.FgRed, color.Bold)
	colorDim := color.New(color.FgHiBlack)

	colorBuild.Printf("Executing: %s\n", command)

	// Parse command and arguments
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return nil, fmt.Errorf("empty command")
	}

	cmd := exec.Command(parts[0], parts[1:]...)
	cmd.Dir = e.workingDir

	// Capture both stdout and stderr
	output, err := cmd.CombinedOutput()
	duration := time.Since(startTime)

	result := &BuildResult{
		Success:   err == nil,
		Output:    string(output),
		Duration:  duration,
		Timestamp: time.Now(),
	}

	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitError.ExitCode()
		}
		result.Error = err.Error()
	}

	// Print output
	if len(result.Output) > 0 {
		fmt.Print(result.Output)
	}

	if result.Success {
		colorSuccess.Printf("Build completed successfully in %v\n", duration)
	} else {
		colorError.Printf("Build failed with exit code %d in %v\n", result.ExitCode, duration)
		if result.Error != "" {
			colorDim.Printf("Error: %s\n", result.Error)
		}
	}

	return result, nil
}

// CopyOutputs copies build outputs to cache
func (e *Executor) CopyOutputs(outputs []string, cacheDir string) error {
	for _, output := range outputs {
		// Skip if output doesn't exist
		if _, err := os.Stat(output); os.IsNotExist(err) {
			continue
		}

		// Create destination path in cache
		destPath := filepath.Join(cacheDir, "outputs", output)
		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			return fmt.Errorf("failed to create cache output directory: %v", err)
		}

		// Copy file or directory
		if err := e.copyPath(output, destPath); err != nil {
			return fmt.Errorf("failed to copy output %s: %v", output, err)
		}
	}

	return nil
}

// RestoreOutputs restores build outputs from cache
func (e *Executor) RestoreOutputs(outputs []string, cacheDir string) error {
	for _, output := range outputs {
		srcPath := filepath.Join(cacheDir, "outputs", output)

		// Skip if cached output doesn't exist
		if _, err := os.Stat(srcPath); os.IsNotExist(err) {
			continue
		}

		// Create destination directory if needed
		if err := os.MkdirAll(filepath.Dir(output), 0755); err != nil {
			return fmt.Errorf("failed to create output directory: %v", err)
		}

		// Copy from cache to working directory
		if err := e.copyPath(srcPath, output); err != nil {
			return fmt.Errorf("failed to restore output %s: %v", output, err)
		}
	}

	return nil
}

// copyPath copies a file or directory recursively
func (e *Executor) copyPath(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	if srcInfo.IsDir() {
		return e.copyDir(src, dst)
	}

	return e.copyFile(src, dst)
}

// copyFile copies a single file
func (e *Executor) copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = dstFile.ReadFrom(srcFile)
	return err
}

// copyDir copies a directory recursively
func (e *Executor) copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Calculate relative path
		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}

		dstPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			return os.MkdirAll(dstPath, info.Mode())
		}

		return e.copyFile(path, dstPath)
	})
}
