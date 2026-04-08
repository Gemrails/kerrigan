# Kerrigan

**Kerrigan** - A Negotiable Decentralized Task Distribution Network

A decentralized resource trading platform built on P2P network and blockchain technology. It enables sharing of compute resources (GPU/Storage/Bandwidth) with transparent pricing via smart contracts and e-CNY payments.

## Architecture

![Communication Structure](media/kerrigan通信结构.png)

## Client Visualization

![Client Demo](media/kerrigan前端效果.gif)

## Features

- **P2P Network**: Distributed peer-to-peer architecture with libp2p
- **Plugin System**: Extensible plugin architecture for GPU, Storage, and Proxy resources
- **Blockchain Integration**: Smart contracts for resource registration and trading
- **e-CNY Payments**: Integrated payment system with escrow protection

## Project Structure

```
kerrigan/
├── cmd/               # Entry points (node, cli)
├── internal/          # Internal packages
│   ├── core/         # Core modules (network, plugin, resource)
│   ├── chain/        # Blockchain integration
│   └── plugins/      # Built-in plugins (gpu-share, storage, proxy)
├── pkg/              # Public utilities (crypto, log, utils)
├── configs/          # Configuration files
└── media/            # Images and media files
```

## Quick Start

### Prerequisites

- Go 1.21+
- Docker (optional)

### Build

```bash
# Install dependencies
make deps

# Build binaries
make build
```

### Run

```bash
# Start provider node
./build/kerrigan-node run --role provider --gpu --storage

# Start consumer node
./build/kerrigan-cli node status
```

## Plugins

### GPU Plugin
Distributed GPU computing for AI inference, model fine-tuning, and batch processing.

### Storage Plugin
IPFS-based distributed storage with P2P file retrieval.

### Proxy Plugin
Network bandwidth sharing with geo-pricing.

## License

MIT
