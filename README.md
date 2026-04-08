# Kerrigan v2 - Distributed Resource Trading Network

## Project Overview

Kerrigan v2 is a decentralized resource trading platform built on P2P network and blockchain technology. It enables sharing of compute resources (CPU/GPU/Storage/Bandwidth) with transparent pricing via smart contracts and e-CNY payments.

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    Application Layer                         │
│    GPU Plugin  │  Storage Plugin  │  Proxy Plugin           │
├─────────────────────────────────────────────────────────────┤
│                    Plugin Runtime                            │
│    Lifecycle  │  Resource Abstraction  │  Metering         │
├─────────────────────────────────────────────────────────────┤
│                    P2P Network Layer                         │
│    Control Plane  │  Data Plane                             │
├─────────────────────────────────────────────────────────────┤
│                    Blockchain Layer                          │
│    Resource Registry  │  Trading  │  Plugin Registry       │
│                    e-CNY Payment                            │
└─────────────────────────────────────────────────────────────┘
```

## Quick Start

### Prerequisites

- Go 1.21+
- Docker (optional, for containerized deployment)

### Build

```bash
# Clone the repository
git clone https://github.com/kerrigan/kerrigan.git
cd kerrigan-v2

# Install dependencies
make deps

# Build binaries
make build

# Run tests
make test
```

### Run Node

```bash
# Start a provider node
./build/kerrigan-node run --role provider --gpu --storage

# Start a consumer node
./build/kerrigan-node run --role consumer

# Start as genesis node
./build/kerrigan-node run --genesis
```

### CLI Usage

```bash
# Check node status
./build/kerrigan-cli node status

# List available resources
./build/kerrigan-cli resource list

# Subscribe to resources
./build/kerrigan-cli resource subscribe --type gpu --amount 1
```

## Core Plugins

### GPU Plugin
Distributed GPU computing for AI inference, model fine-tuning, and batch processing.

### Storage Plugin
IPFS-based distributed storage with P2P file retrieval.

### Proxy Plugin
Network bandwidth sharing with geo-pricing.

## Directory Structure

```
kerrigan-v2/
├── cmd/               # Entry points
│   ├── node/          # Node binary
│   └── cli/           # CLI binary
├── internal/          # Internal packages
│   ├── core/          # Core modules
│   │   ├── network/    # P2P networking
│   │   ├── plugin/     # Plugin system
│   │   ├── resource/   # Resource abstraction
│   │   ├── task/       # Task scheduling
│   │   └── metering/   # Billing system
│   ├── chain/         # Blockchain integration
│   │   ├── contracts/  # Smart contracts
│   │   └── payment/    # e-CNY integration
│   ├── plugins/        # Built-in plugins
│   └── api/           # gRPC/HTTP APIs
├── pkg/               # Public packages
├── configs/           # Configuration
├── scripts/           # Build scripts
├── docs/              # Documentation
└── tests/             # Test fixtures
```

## Configuration

Configuration is done via YAML files in `configs/`:

```yaml
# node.yaml
node:
  node_id: ""  # Auto-generated if empty
  roles:
    - provider
  plugins:
    - gpu-share
    - storage

network:
  control_port: 38888
  data_port: 38889
  seed_nodes:
    - /ip4/1.2.3.4/tcp/38888/p2p/Qmxxx

blockchain:
  network: testnet  # mainnet, testnet, local
  payment_contract: "0x..."
```

## Documentation

- [Architecture](docs/ARCHITECTURE.md)
- [Plugin Development Guide](docs/PLUGIN_GUIDE.md)
- [API Reference](docs/API.md)
- [Deployment Guide](docs/DEPLOYMENT.md)

## License

MIT License - see LICENSE file

## Contributing

Contributions welcome! Please read CONTRIBUTING.md before submitting PRs.
