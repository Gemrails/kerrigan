# 🦌 Kerrigan

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](./LICENSE)
[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?logo=go&logoColor=white)](./go.mod)

**Kerrigan** - 可议价的去中心化任务分发网络

基于 P2P 网络和区块链技术构建的去中心化资源交易平台。通过智能合约和数字人民币（e-CNY）支付，实现计算资源（GPU/存储/带宽）的共享与透明定价。

## 通信结构

![通信结构](media/kerrigan通信结构.png)

## 客户端可视化效果

![客户端演示](media/kerrigan前端效果.gif)

## 系统架构

```
┌─────────────────────────────────────────────────────────────┐
│                      应用层                                  │
│        GPU 插件  │  存储插件  │  代理插件                    │
├─────────────────────────────────────────────────────────────┤
│                      插件运行时                                │
│     生命周期管理  │  资源抽象层  │  计费计量                    │
├─────────────────────────────────────────────────────────────┤
│                      P2P 网络层                               │
│             控制平面  │  数据平面                             │
├─────────────────────────────────────────────────────────────┤
│                      区块链层                                 │
│       资源注册  │  交易  │  插件注册                          │
│                     e-CNY 支付                               │
└─────────────────────────────────────────────────────────────┘
```

## 核心特性

- **P2P 网络**: 基于 libp2p 的分布式点对点架构
- **插件系统**: 可扩展的插件架构，支持 GPU、存储和代理资源
- **区块链集成**: 用于资源注册和交易的智能合约
- **e-CNY 支付**: 带托管保护的集成支付系统

## 快速开始

### 前置要求

- Go 1.21+
- Docker（可选）

### 构建

```bash
# 克隆仓库
git clone git@github.com:Gemrails/kerrigan.git
cd kerrigan

# 安装依赖
make deps

# 构建二进制
make build
```

### 运行

```bash
# 启动 Provider 节点
./build/kerrigan-node run --role provider --gpu --storage

# 启动创世节点
./build/kerrigan-node run --genesis
```

### CLI 使用

```bash
# 查看节点状态
./build/kerrigan-cli node status

# 列出可用资源
./build/kerrigan-cli resource list
```

## 插件

### GPU 插件
用于 AI 推理、模型微调和批处理的分布式 GPU 计算。

### 存储插件
基于 IPFS 的分布式存储，支持 P2P 文件检索。

### 代理插件
支持地理定价的网络带宽共享。

## 项目结构

```
kerrigan/
├── cmd/               # 入口程序 (node, cli)
├── internal/          # 内部包
│   ├── core/         # 核心模块 (网络, 插件, 资源)
│   ├── chain/        # 区块链集成
│   └── plugins/      # 内置插件 (gpu-share, storage, proxy)
├── pkg/              # 公共工具 (crypto, log, utils)
├── configs/          # 配置文件
└── media/           # 图片和媒体文件
```

## 配置

通过 `configs/` 中的 YAML 文件进行配置：

```yaml
# node.yaml
node:
  node_id: ""  # 空则自动生成
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

## 文档

- [架构设计](docs/ARCHITECTURE_MINDMAP.md)
- [插件开发指南](docs/REFACTOR_PLAN_v2.md)

## 许可证

本项目开源，基于 [MIT 许可证](./LICENSE)。
