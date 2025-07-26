# ForgeCache

**Universal Dev Cache & Dependency Manager**

ForgeCache is a powerful tool with the following features:

| Feature | Description |
|---------|-------------|
| **Smart Builds** | Only rebuild when inputs actually change |
| **Cross-Platform** | Works on Windows, Linux, macOS |
| **Language Agnostic** | Works with any build command |
| **Project Local** | Cache is local to each project |
| **Fast** | Cache hits restore in milliseconds vs seconds/minutes for rebuilds |
| **Simple Config** | Single TOML file configuration |nguage, cross-project development tool that intelligently caches build artifacts and manages language toolchains to accelerate your development workflow.

## What Is It?

A **smart build cache** that helps you:

* Cache and re-use build artifacts across runs and machines
* Manage language toolchains (Go, Rust, Zig, etc.)
* Track inputs/outputs to only rebuild when truly needed
* Keep all projects reproducible and fast
* Replace scattered ad-hoc dev tooling with a single smart engine

**Inspired by:**
* **Bazel** - for smart dependency graphs and caching
* **Volta/NVM** - for managing language/tool versions
* **Nix** - but more approachable and easier to adopt

## Core Use Case

If you write code in multiple languages (Go, Rust, Zig, Crystal) and want:
- Consistent toolchain versions
- Fast rebuilds through intelligent caching
- Cache reuse across similar projects
- Simple configuration (no complex Makefiles or bash scripts)

ForgeCache provides this through a single declarative config file.

## Quick Start

### 1. Initialize a project

```bash
forge init
```

This creates a `.forgefile` with sensible defaults:

```toml
[toolchain]
go = "1.22"

[build]
cmd = "go build -o myapp ./cmd"

[cache]
inputs = ["go.mod", "go.sum", "**/*.go"]
outputs = ["myapp"]
```

### 2. Build with caching

```bash
forge build
```

First run builds normally. Subsequent runs with unchanged inputs restore from cache instantly.

### 3. Additional commands

```bash
forge clean      # Clear build cache
forge version    # Show version info
forge update     # Auto-update to latest version
```

## How It Works

1. **Input Hashing**: ForgeCache computes SHA256 hashes of all input files and directories
2. **Cache Check**: Before building, it checks if outputs exist for the current input hash
3. **Smart Execution**: Only rebuilds when inputs change, otherwise restores cached outputs
4. **Metadata Storage**: Stores build metadata (duration, success/failure, timestamps) in `.forge/cache`
