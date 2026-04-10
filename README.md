# 👑 Kerrigan

English | [中文](./README_zh.md)

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](./LICENSE)
[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?logo=go&logoColor=white)](./go.mod)

**Kerrigan** - A Negotiable Decentralized AI Task Distribution Network
*Turn your lobster into data assets* 🦞

A decentralized resource trading platform built on P2P network and blockchain technology. It enables sharing of compute resources (GPU/Storage/Bandwidth) with transparent pricing via smart contracts and e-CNY payments.

**Core Innovation**: Native **OpenClaw & DeerFlow** integration - AI agent tasks are automatically dispatched to optimal remote nodes running [OpenClaw](https://github.com/openclaw/openclaw) or [DeerFlow](https://github.com/bytedance/deer-flow) frameworks, enabling distributed AI inference without direct hardware access.

## Communication Structure

![Communication Structure](media/kerrigan通信结构.png)

## Client Visualization

![Client Demo](media/kerrigan前端效果.gif)

## System Architecture (Compliant Dual-Chain Design)

```
┌───────────────────────────────────────────────────────────────────┐
│                       Application Layer                            │
│  Agent Plugin  │  GPU Plugin  │  Storage Plugin  │  Proxy Plugin │
│  (Featured)                                            │            │
├───────────────────────────────────────────────────────────────────┤
│                       Plugin Runtime                               │
│    Lifecycle  │  Resource Abstraction  │  Metering                │
├───────────────────────────────────────────────────────────────────┤
│                       P2P Network Layer                            │
│    Control Plane  │  Data Plane                                  │
├───────────────────────────────────────────────────────────────────┤
│                                                                   │
│   ┌───────────────────────────┐  ┌───────────────────────────┐    │
│   │   Business Chain         │  │   Settlement Layer        │    │
│   │   (Federated Chain)      │  │   (e-CNY 2.0)             │    │
│   ├───────────────────────────┤  ├───────────────────────────┤    │
│   │ • Resource Registry      │  │ • Escrow &托管           │    │
│   │ • Task Distribution      │  │ • e-CNY Payments          │    │
│   │ • Service Attestation    │  │ • Settlement &对账        │    │
│   │ • Dispute Arbitration    │  │ • Compliance Audit        │    │
│   └───────────────────────────┘  └───────────────────────────┘    │
│                                                                   │
└───────────────────────────────────────────────────────────────────┘

⚖️ Compliant Design: No Token/ICO • e-CNY Only • Licensed Bank Integration
```

### Compliance Features
- **No Token Issuance**: Pure resource trading, no virtual currency
- **e-CNY 2.0 Native**: Bank deposit-based digital RMB (launched 2026)
- **Licensed Integration**: Via licensed banks/institutions
- **KYC/AML Compliant**: Full identity verification

## Key Concepts

### Core Values
- **Decentralized**: No single control point, resources distributed globally
- **Negotiable**: Resource prices dynamically adjusted by market supply/demand
- **Trustworthy**: Smart contracts ensure transparent transactions
- **Extensible**: Plugin-based architecture supporting arbitrary resource types

### Node Roles
| Role | Description |
|------|-------------|
| **Provider** | Contributes idle compute resources |
| **Consumer** | Uses resources from the network |
| **Relay** | Forwards traffic for NAT traversal |
| **Validator** | Verifies transactions on-chain |

### P2P Network
- **Control Plane**: Node discovery, protocol negotiation, heartbeat (port 38888)
- **Data Plane**: Tunnel transmission, load balancing, traffic encryption (port 38889)
- **NAT Traversal**: Direct connection, hole punching, relay fallback

## ⭐ Featured Plugin: Agent Task Dispatch

**The most powerful feature**: Dispatch AI agent tasks to remote nodes running OpenClaw or DeerFlow frameworks. Unlike traditional GPU plugins that require direct hardware access, the Agent plugin enables task distribution across a P2P network of AI agents.

### 🤖 Agent Plugin

Distributed AI task execution via OpenClaw/DeerFlow integration:

```
┌─────────┐    Task Request    ┌─────────┐    Dispatch    ┌─────────┐
│Consumer │ ──────────────────► │  Node   │ ────────────► │OpenClaw │
│         │                    │ (Local) │               │ Node    │
└─────────┘                    └─────────┘               └─────────┘
                                                        or
                                                      ┌─────────┐
                                                      │DeerFlow │
                                                      │ Node    │
                                                      └─────────┘
```

**Supported Frameworks:**

| Framework | Protocol | Capabilities | Use Case |
|-----------|----------|--------------|----------|
| **OpenClaw** | WebSocket (port 18789) | Task queue, Multi-channel AI assistant, Sub-agent orchestration | Automation, Chat, Long-running tasks |
| **DeerFlow** | HTTP API (port 8000) | Deep research, LangGraph orchestration, Code execution, Multi-step workflows | Research, Complex coding, Multi-agent collaboration |

**Task Types:**
- `research` - Deep research with web search and source synthesis
- `code` - Code generation, debugging, and execution
- `general` - Multi-turn conversation and task automation

**Architecture:**
- **Agent Registry**: Tracks available agent nodes by capabilities (web_search, code_execution, deep_research)
- **Task Dispatcher**: Worker pool with concurrent task execution and timeout control
- **Protocol Adapters**: Pluggable adapters for different agent frameworks

**Pricing**: Based on task complexity, execution time, and required capabilities

---

## Plugins

### 🎮 GPU Plugin
Distributed GPU computing for AI workloads:
- LLM inference (text generation)
- Image generation (Stable Diffusion)
- Embedding vectors
- Model fine-tuning (LoRA)
- Batch inference

**Pricing**: Based on GPU model (RTX 4090/A100/H100), VRAM tier, and duration

### 📦 Storage Plugin
IPFS-based distributed storage:
- File upload with CID retrieval
- Pinning for persistence
- P2P file distribution
- Multi-replica redundancy

**Pricing**: Based on GB/month storage and replication factor

### 🌐 Proxy Plugin
Network bandwidth sharing:
- HTTP/HTTPS transparent proxy
- SOCKS5 universal proxy
- TUN/TAP global proxy
- Geo-pricing (CN/US/JP/SG/EU/HK)

**Pricing**: Based on region, bandwidth tier, and traffic volume

## Features

- **Agent Task Dispatch**: Native support for OpenClaw/DeerFlow frameworks - dispatch AI tasks across the P2P network
- **P2P Network**: Distributed peer-to-peer architecture with libp2p
- **Plugin System**: Extensible plugin architecture for Agent, GPU, Storage, and Proxy resources
- **Blockchain Integration**: Smart contracts for resource registration and trading
- **e-CNY Payments**: Integrated payment system with escrow protection

## Quick Start

### Prerequisites

- Go 1.21+
- Docker (optional)

### Build

```bash
# Clone the repository
git clone git@github.com:Gemrails/kerrigan.git
cd kerrigan

# Install dependencies
make deps

# Build binaries
make build
```

### Run

```bash
# Start provider node
./build/kerrigan-node run --role provider --gpu --storage

# Start as genesis node
./build/kerrigan-node run --genesis
```

### CLI Usage

```bash
# Check node status
./build/kerrigan-cli node status

# List available resources
./build/kerrigan-cli resource list
```

## Project Structure

```
kerrigan/
├── cmd/               # Entry points (node, cli)
├── internal/          # Internal packages
│   ├── core/         # Core modules (network, plugin, resource)
│   │   ├── network/  # P2P networking (control, data, discovery, tunnel)
│   │   └── plugin/   # Plugin system (runtime, registry, loader)
│   ├── chain/        # Blockchain integration
│   │   ├── contracts/# Smart contracts
│   │   └── payment/  # e-CNY integration
│   └── plugins/      # Built-in plugins (gpu-share, storage, proxy)
├── pkg/              # Public utilities (crypto, log, utils)
├── configs/          # Configuration files
└── media/           # Images and media files
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

- [Architecture Mind Map](docs/ARCHITECTURE_MINDMAP.md)
- [Communication Architecture](docs/COMMUNICATION.md)
- [Plugin Development Guide](docs/REFACTOR_PLAN_v2.md)

## License

This project is open source and available under the [MIT License](./LICENSE).
