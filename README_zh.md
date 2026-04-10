# 👑 Kerrigan

[English](./README.md) | 中文

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](./LICENSE)
[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?logo=go&logoColor=white)](./go.mod)

**Kerrigan** - 可议价的去中心化任务分发网络

基于 P2P 网络和区块链技术构建的去中心化资源交易平台。通过智能合约和数字人民币（e-CNY）支付，实现计算资源（GPU/存储/带宽）的共享与透明定价。

## 通信结构

![通信结构](media/kerrigan通信结构.png)

## 客户端可视化效果

![客户端演示](media/kerrigan前端效果.gif)

## 系统架构（合规双链设计）

```
┌───────────────────────────────────────────────────────────────────┐
│                          应用层                                    │
│              GPU 插件  │  存储插件  │  代理插件                      │
├───────────────────────────────────────────────────────────────────┤
│                          插件运行时                                │
│              生命周期管理  │  资源抽象层  │  计费计量                    │
├───────────────────────────────────────────────────────────────────┤
│                          P2P 网络层                                │
│                      控制平面  │  数据平面                           │
├───────────────────────────────────────────────────────────────────┤
│                                                                   │
│    ┌───────────────────────────┐  ┌───────────────────────────┐    │
│    │         业务链              │  │        结算层              │    │
│    │      (联盟链)               │  │    (e-CNY 2.0)            │    │
│    ├───────────────────────────┤  ├───────────────────────────┤    │
│    │ • 节点注册与身份认证       │  │ • 资金托管 (Escrow)        │    │
│    │ • 资源注册与能力声明       │  │ • e-CNY 支付与收款         │    │
│    │ • 任务分发与匹配           │  │ • 清结算与对账              │    │
│    │ • 服务交付存证             │  │ • 合规审计日志              │    │
│    │ • 争议仲裁记录             │  │ • 税务申报接口              │    │
│    └───────────────────────────┘  └───────────────────────────┘    │
│                                                                   │
└───────────────────────────────────────────────────────────────────┘

⚖️ 合规设计：不发币原则  •  仅支持 e-CNY  •  持牌机构接入
```

### 合规特性
- **不发币**: 纯资源交易，无虚拟货币
- **e-CNY 2.0 原生支持**: 银行存款型数字人民币（2026年启动）
- **持牌接入**: 通过银行/持牌机构接入支付
- **KYC/AML 合规**: 完整的身份验证

## 核心概念

### 核心价值
- **去中心化**: 无单一控制点，资源分布全球
- **可议价**: 资源价格由市场供需动态调节
- **可信**: 智能合约保障透明交易
- **可扩展**: 插件化架构支持任意资源类型

### 节点角色
| 角色 | 描述 |
|------|------|
| **Provider** | 贡献闲置计算资源 |
| **Consumer** | 使用网络资源 |
| **Relay** | 转发流量实现 NAT 穿透 |
| **Validator** | 验证链上交易 |

### P2P 网络
- **控制平面**: 节点发现、协议协商、心跳保活 (端口 38888)
- **数据平面**: 隧道传输、负载均衡、流量加密 (端口 38889)
- **NAT 穿透**: 直连、打洞、中继Fallback

## 插件

### 🎮 GPU 插件
用于 AI 工作负载的分布式 GPU 计算：
- LLM 推理（文本生成）
- 图像生成（Stable Diffusion）
- Embedding 向量
- 模型微调（LoRA）
- 批量推理

**定价**: 基于 GPU 型号（RTX 4090/A100/H100）、显存档位和时长

### 📦 存储插件
基于 IPFS 的分布式存储：
- 文件上传与 CID 检索
- Pinning 持久化
- P2P 文件分发
- 多副本冗余

**定价**: 基于 GB/月存储量和副本数

### 🌐 代理插件
网络带宽共享：
- HTTP/HTTPS 透明代理
- SOCKS5 通用代理
- TUN/TAP 全局代理
- 地理定价（CN/US/JP/SG/EU/HK）

**定价**: 基于地区、带宽档位和流量

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

## 项目结构

```
kerrigan/
├── cmd/               # 入口程序 (node, cli)
├── internal/          # 内部包
│   ├── core/         # 核心模块 (网络, 插件, 资源)
│   │   ├── network/  # P2P 网络 (control, data, discovery, tunnel)
│   │   └── plugin/   # 插件系统 (runtime, registry, loader)
│   ├── chain/        # 区块链集成
│   │   ├── contracts/# 智能合约
│   │   └── payment/  # e-CNY 集成
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

- [架构脑图](docs/ARCHITECTURE_MINDMAP.md)
- [通信架构](docs/COMMUNICATION.md)
- [插件开发指南](docs/REFACTOR_PLAN_v2.md)

## 许可证

本项目开源，基于 [MIT 许可证](./LICENSE)。
