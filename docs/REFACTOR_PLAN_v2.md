# Kerrigan 重构计划 v2.0

> 可议价的去中心化资源分发网络 - 架构设计文档

---

## 一、项目愿景与定位

### 1.1 重新定义

Kerrigan v2 是一个**去中心化的资源交易基础设施**，允许任意节点贡献闲置的计算资源（CPU/GPU/存储/带宽），并通过智能合约实现资源的透明定价与结算。

### 1.2 核心价值主张

| 价值维度 | 描述 |
|---------|------|
| **去中心化** | 无单一控制点，资源分布在全球节点上 |
| **可议价** | 资源价格由市场供需动态调节 |
| **可信** | 数字人民币支付，链上合约保障 |
| **可扩展** | 插件化架构，支持任意资源类型 |

### 1.3 目标场景

- AI 推理服务（LLM、图像生成）
- 分布式存储与 CDN
- 网络加速与代理服务
- 边缘计算任务分发

---

## 二、整体架构设计

### 2.1 架构总览

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              Kerrigan v2 架构                                │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  ┌─────────────────────────────────────────────────────────────────────┐  │
│  │                        应用层 (Application Layer)                      │  │
│  │  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐           │  │
│  │  │ GPU插件  │  │存储插件  │  │代理插件  │  │ 其它插件  │           │  │
│  │  └──────────┘  └──────────┘  └──────────┘  └──────────┘           │  │
│  └─────────────────────────────────────────────────────────────────────┘  │
│                                      │                                       │
│  ┌─────────────────────────────────────────────────────────────────────┐  │
│  │                      插件运行时 (Plugin Runtime)                      │  │
│  │         插件管理  │  资源抽象  │  任务调度  │  计量计费                 │  │
│  └─────────────────────────────────────────────────────────────────────┘  │
│                                      │                                       │
│  ┌─────────────────────────────────────────────────────────────────────┐  │
│  │                      P2P 网络层 (P2P Network Layer)                   │  │
│  │  ┌────────────────────────┐    ┌────────────────────────┐            │  │
│  │  │     控制网络           │    │      数据网络           │            │  │
│  │  │  (节点发现/协议协商)    │    │  (隧道传输/负载均衡)    │            │  │
│  │  └────────────────────────┘    └────────────────────────┘            │  │
│  └─────────────────────────────────────────────────────────────────────┘  │
│                                      │                                       │
│  ┌─────────────────────────────────────────────────────────────────────┐  │
│  │                       区块链层 (Blockchain Layer)                     │  │
│  │  ┌──────────────────┐  ┌──────────────────┐  ┌──────────────────┐    │  │
│  │  │   资源注册合约   │  │   交易结算合约   │  │  插件仓库合约   │    │  │
│  │  └──────────────────┘  └──────────────────┘  └──────────────────┘    │  │
│  │                     │                                                    │  │
│  │  ┌──────────────────┐  ┌──────────────────┐                            │  │
│  │  │  数字人民币支付   │  │   Token 经济     │                            │  │
│  │  │   (e-CNY 集成)   │  │   (可选)         │                            │  │
│  │  └──────────────────┘  └──────────────────┘                            │  │
│  └─────────────────────────────────────────────────────────────────────┘  │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 2.2 分层职责

| 层级 | 组件 | 职责 |
|------|------|------|
| **应用层** | GPU/存储/代理插件 | 特定资源类型的实现 |
| **运行时层** | Plugin Runtime | 插件生命周期管理、资源抽象、计量 |
| **网络层** | Control Plane | 节点发现、协议协商、心跳 |
| **网络层** | Data Plane | 隧道传输、流量路由、负载均衡 |
| **区块链层** | Smart Contracts | 资源注册、交易结算、插件仓库 |
| **区块链层** | Payment | 数字人民币支付通道 |

### 2.3 目录结构

```
kerrigan-v2/
├── cmd/
│   ├── node/                    # 节点主程序
│   │   └── main.go
│   └── cli/                     # CLI 客户端
│       └── main.go
├── internal/
│   ├── core/                    # 核心模块
│   │   ├── network/             # P2P 网络
│   │   │   ├── control/         # 控制平面
│   │   │   ├── data/            # 数据平面
│   │   │   ├── discovery/       # 节点发现
│   │   │   └── tunnel/          # 隧道管理
│   │   ├── plugin/              # 插件系统
│   │   │   ├── runtime/         # 运行时
│   │   │   ├── registry/        # 本地注册表
│   │   │   └── loader/          # 插件加载器
│   │   ├── resource/            # 资源抽象
│   │   │   ├── gpu/             # GPU 资源
│   │   │   ├── storage/         # 存储资源
│   │   │   └── bandwidth/       # 带宽资源
│   │   ├── task/                # 任务调度
│   │   │   ├── scheduler/       # 调度器
│   │   │   └── queue/           # 任务队列
│   │   └── metering/            # 计量计费
│   │       ├── collector/       # 数据采集
│   │       └── billing/         # 计费引擎
│   ├── chain/                   # 区块链交互
│   │   ├── contracts/           # 智能合约 ABI
│   │   │   ├── resource.sol     # 资源注册合约
│   │   │   ├── trading.sol      # 交易结算合约
│   │   │   └── registry.sol     # 插件仓库合约
│   │   ├── client/              # 链交互客户端
│   │   └── payment/             # 支付集成
│   │       └── ecny/            # 数字人民币
│   ├── plugins/                 # 内置插件
│   │   ├── gpu-share/           # GPU 算力插件
│   │   ├── storage/             # 分布式存储插件
│   │   └── proxy/               # 网络代理插件
│   └── api/                     # API 服务
│       ├── grpc/                # gRPC 接口
│       └── http/                # HTTP 接口
├── pkg/
│   ├── crypto/                  # 密码学工具
│   ├── utils/                   # 通用工具
│   └── log/                     # 日志
├── configs/                     # 配置文件
├── scripts/                     # 脚本
├── docs/                        # 文档
└── tests/                       # 测试
```

---

## 三、核心插件详细设计

### 3.1 GPU 算力插件 (gpu-share)

#### 3.1.1 功能概述

GPU 插件允许节点贡献闲置的 GPU 算力，为 AI 推理、模型微调等场景提供分布式计算能力。

#### 3.1.2 技术架构

```
┌────────────────────────────────────────────────────────────────────────┐
│                         GPU 插件架构                                    │
├────────────────────────────────────────────────────────────────────────┤
│                                                                        │
│  ┌──────────────┐     ┌──────────────┐     ┌──────────────┐         │
│  │   Client SDK │     │  Load Balancer│     │   Task Queue │         │
│  │  (Python/JS) │────▶│              │────▶│              │         │
│  └──────────────┘     └──────────────┘     └──────────────┘         │
│                              │                    │                    │
│                              ▼                    ▼                    │
│                      ┌──────────────┐     ┌──────────────┐           │
│                      │  GPU Node    │     │  GPU Node    │           │
│                      │  Selector    │────▶│  Pool        │           │
│                      └──────────────┘     └──────────────┘           │
│                                                 │                     │
│                      ┌──────────────────────────┼───────────────┐     │
│                      │                          │               │     │
│                      ▼                          ▼               ▼     │
│               ┌──────────┐              ┌──────────┐    ┌──────────┐ │
│               │  Node A  │              │  Node B  │    │  Node C  │ │
│               │ RTX 4090 │              │ A100 40G │    │ H100 80G │ │
│               └──────────┘              └──────────┘    └──────────┘ │
│                                                                        │
└────────────────────────────────────────────────────────────────────────┘
```

#### 3.1.3 支持的场景

| 场景 | 输入 | 输出 | 适用模型 |
|------|------|------|---------|
| **LLM 推理** | 文本 Prompt | 文本响应 | Llama, ChatGLM, Qwen |
| **图像生成** | 文本描述 | 图片 | Stable Diffusion |
| **embedding** | 文本 | 向量 | BGE, M3E |
| **模型微调** | 数据集 | 微调后模型 | LoRA 训练 |
| **批量推理** | 批量数据 | 结果文件 | 任意模型 |

#### 3.1.4 定价模型

```go
type GPUPricing struct {
    BasePrice     float64            // 基础价格 (元/秒)
    TierMultipiler map[string]float64 // GPU 型号系数
    MemoryMultipiler map[string]float64 // 显存系数
    DurationDiscount map[int]float64   // 时长折扣 (小时数 -> 折扣)
}

var DefaultPricing = GPUPricing{
    BasePrice: 0.001, // 0.1分钱/秒
    TierMultipiler: map[string]float64{
        "RTX_4090":   1.0,
        "RTX_3090":   0.9,
        "A100_40G":   3.0,
        "A100_80G":   4.5,
        "H100_80G":   8.0,
    },
    MemoryMultipiler: map[string]float64{
        "low":    1.0,   // < 8GB
        "medium": 1.5,   // 8-24GB
        "high":   2.0,   // 24-80GB
    },
}
```

#### 3.1.5 API 接口

```protobuf
service GPUService {
    // 提交推理任务
    rpc Infer(InferRequest) returns (InferResponse);
    
    // 批量推理
    rpc BatchInfer(BatchInferRequest) returns (BatchInferResponse);
    
    // 获取可用 GPU 列表
    rpc ListAvailableGPU(ListGPURequest) returns (ListGPUResponse);
    
    // 预约 GPU 资源
    rpc ReserveGPU(ReserveRequest) returns (ReserveResponse);
}
```

#### 3.1.6 与大模型集成的关键设计

**1. 模型市场**
```go
type ModelMarketplace struct {
    Models []ModelInfo
    
    // 预置模型
    PreloadedModels []string
}

type ModelInfo struct {
    ModelID   string
    Name      string
    SizeGB    int           // 模型大小
    MemReq    int           // 显存需求
    Type      ModelType     // text/image/embedding
    Endpoint  string        // 推理端点
}
```

**2. 模型加载策略**
- 热门模型预加载到高速存储
- 冷门模型按需下载
- 支持 LoRA adapter 热切换

**3. 推理会话管理**
```go
type InferenceSession struct {
    SessionID   string
    ModelID     string
    NodeID      string
    StartTime   time.Time
    MaxDuration time.Duration
    State       SessionState
}
```

---

### 3.2 分布式存储插件 (storage)

#### 3.2.1 功能概述

基于 IPFS 的分布式存储插件，提供 P2P 文件存储、检索和分发服务。

#### 3.2.2 技术架构

```
┌────────────────────────────────────────────────────────────────────────┐
│                       分布式存储插件架构                                  │
├────────────────────────────────────────────────────────────────────────┤
│                                                                        │
│  ┌──────────────────────────────────────────────────────────────────┐ │
│  │                      存储抽象层 (Storage Abstraction)              │ │
│  │   mount/storage   │   ipfs/pinning   │   cache/lru              │ │
│  └──────────────────────────────────────────────────────────────────┘ │
│                                    │                                    │
│                                    ▼                                    │
│  ┌──────────────────────────────────────────────────────────────────┐ │
│  │                      IPFS 交互层                                   │ │
│  │   add(file)    │   cat(cid)    │   pin(cid)    │   dag          │ │
│  └──────────────────────────────────────────────────────────────────┘ │
│                                    │                                    │
│                                    ▼                                    │
│  ┌──────────────────────────────────────────────────────────────────┐ │
│  │                      DHT 路由层                                   │ │
│  │   providers(CID)   │   findPeer   │   AnnounceProvider            │ │
│  └──────────────────────────────────────────────────────────────────┘ │
│                                    │                                    │
│  ┌─────────┐  ┌─────────┐  ┌─────────┐  ┌─────────┐                 │
│  │  Node A │  │  Node B │  │  Node C │  │  Node D │                 │
│  │ [Block] │  │ [Block] │  │ [Block] │  │ [Block] │                 │
│  └─────────┘  └─────────┘  └─────────┘  └─────────┘                 │
│                                                                        │
└────────────────────────────────────────────────────────────────────────┘
```

#### 3.2.3 核心功能

| 功能 | 描述 | API |
|------|------|-----|
| **上传** | 分块存储，返回 CID | `PUT /storage` |
| **下载** | 通过 CID 检索下载 | `GET /storage/{cid}` |
| **Pin** | 持久化存储 | `POST /pin/{cid}` |
| **列表** | 查看已 Pin 的文件 | `GET /pins` |
| **分享** | 生成下载链接 | `POST /share/{cid}` |

#### 3.2.4 存储市场机制

```go
type StorageOffer struct {
    ProviderID    string
    PricePerGB    float64   // 元/GB/月
    Replication   int       // 副本数
    Duration      int        // 最小存储时长(天)
    Bandwidth     int        // 带宽限制(Mbps)
}

type StorageContract struct {
    CID          string
    Providers    []string
    TotalSize    int64
    Duration     time.Duration
    TotalCost    float64
    PaymentMethod PaymentMethod
}
```

#### 3.2.5 激励机制

- **存储证明**：定期生成存储证明，验证数据完整性
- **奖励机制**：长期存储获得额外奖励
- **惩罚机制**：数据丢失扣除押金

```go
type StorageProof struct {
    CID          string
    ProviderID   string
    Proof        []byte     // 零知识证明
    Timestamp    int64
}
```

---

### 3.3 网络代理插件 (proxy)

#### 3.3.1 功能概述

允许节点共享自己的网络出口，为其他节点提供代理服务。

#### 3.3.2 技术架构

```
┌────────────────────────────────────────────────────────────────────────┐
│                        网络代理插件架构                                  │
├────────────────────────────────────────────────────────────────────────┤
│                                                                        │
│                         ┌──────────────┐                               │
│                         │   Gateway    │                               │
│                         │  (入口节点)   │                               │
│                         └──────┬───────┘                               │
│                                │                                        │
│            ┌───────────────────┼───────────────────┐                   │
│            │                   │                   │                   │
│            ▼                   ▼                   ▼                   │
│     ┌──────────┐        ┌──────────┐        ┌──────────┐             │
│     │  Proxy A │        │  Proxy B │        │  Proxy C │             │
│     │ (美国节点)│        │ (日本节点)│        │ (欧洲节点)│             │
│     └──────────┘        └──────────┘        └──────────┘             │
│                                                                        │
│     ┌──────────────────────────────────────────────────────────┐       │
│     │                     隧道网络 (KCP/libp2p)                 │       │
│     │   控制隧道: 38888   │   数据隧道: 38889                   │       │
│     └──────────────────────────────────────────────────────────┘       │
│                                                                        │
│     流量加密: ChaCha20-Poly1305   │   协议伪装: TLS 1.3               │
│                                                                        │
└────────────────────────────────────────────────────────────────────────┘
```

#### 3.3.3 代理类型

| 类型 | 描述 | 适用场景 |
|------|------|---------|
| **HTTP/HTTPS** | 透明代理 | 网页访问、API 调用 |
| **SOCKS5** | 通用代理 | 游戏、邮件、IM |
| **TUN/TAP** | 全局代理 | 全流量代理 |

#### 3.3.4 定价模型

```go
type ProxyPricing struct {
    BasePrice      float64   // 基础价格 (元/MB)
    RegionMultiplier map[string]float64  // 地区系数
    BandwidthTier   map[int]float64     // 带宽档位
}

var DefaultProxyPricing = ProxyPricing{
    BasePrice: 0.0001, // 1分钱/100MB
    RegionMultiplier: map[string]float64{
        "CN":  1.0,    // 中国
        "US":  1.5,    // 美国
        "JP":  1.3,    // 日本
        "SG":  1.2,    // 新加坡
        "EU":  1.4,    // 欧洲
        "HK":  0.8,    // 香港
    },
    BandwidthTier: map[int]float64{
        10:   1.0,    // 10Mbps
        50:   0.9,    // 50Mbps
        100:  0.8,    // 100Mbps
        1000: 0.6,    // 1Gbps
    },
}
```

#### 3.3.5 流量统计

```go
type TrafficRecord struct {
    SessionID   string
    ProviderID  string
    ConsumerID  string
    BytesIn     int64
    BytesOut    int64
    StartTime   time.Time
    EndTime     time.Time
    Region      string
}

type BillingCycle struct {
    ProviderID   string
    StartTime    time.Time
    EndTime      time.Time
    TotalMB      float64
    TotalCost    float64
    PaymentStatus PaymentStatus
}
```

---

## 四、插件系统设计

### 4.1 插件架构

```
┌────────────────────────────────────────────────────────────────────────┐
│                          插件系统架构                                   │
├────────────────────────────────────────────────────────────────────────┤
│                                                                        │
│  ┌──────────────────────────────────────────────────────────────────┐ │
│  │                      插件运行时 (Plugin Runtime)                   │ │
│  │                                                                   │ │
│  │   ┌────────────┐   ┌────────────┐   ┌────────────┐              │ │
│  │   │  加载器    │   │  生命周期   │   │  资源管理   │              │ │
│  │   │  Loader   │   │  Manager   │   │  Manager   │              │ │
│  │   └────────────┘   └────────────┘   └────────────┘              │ │
│  │                                                                   │ │
│  │   ┌────────────┐   ┌────────────┐   ┌────────────┐              │ │
│  │   │  计量计费   │   │  安全沙箱   │   │  升级更新   │              │ │
│  │   │  Metering  │   │  Sandbox   │   │  Updater   │              │ │
│  │   └────────────┘   └────────────┘   └────────────┘              │ │
│  └──────────────────────────────────────────────────────────────────┘ │
│                                    │                                   │
│                                    ▼                                   │
│  ┌──────────────────────────────────────────────────────────────────┐ │
│  │                      插件接口层 (Plugin Interface)                 │ │
│  │                                                                   │ │
│  │   type Plugin interface {                                         │ │
│  │       Init(ctx context.Context, cfg PluginConfig) error            │ │
│  │       Start() error                                               │ │
│  │       Stop() error                                                │ │
│  │       GetInfo() PluginInfo                                        │ │
│  │       GetCapabilities() []Capability                              │ │
│  │   }                                                               │ │
│  │                                                                   │ │
│  │   type ResourceProvider interface {                                │ │
│  │       QueryResources() (ResourceList, error)                       │ │
│  │       AllocateResources(req ResourceRequest) (Allocation, error)    │ │
│  │       ReleaseResources(id AllocationID) error                      │ │
│  │   }                                                               │ │
│  │                                                                   │ │
│  │   type TaskExecutor interface {                                    │ │
│  │       Execute(ctx context.Context, task Task) (*Result, error)     │ │
│  │       QueryStatus(taskID string) (TaskStatus, error)               │ │
│  │   }                                                               │ │
│  └──────────────────────────────────────────────────────────────────┘ │
│                                    │                                   │
│                                    ▼                                   │
│  ┌──────────────────────────────────────────────────────────────────┐ │
│  │                      插件实现 (Plugin Implementations)             │ │
│  │                                                                   │ │
│  │   ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐        │ │
│  │   │ GPU插件  │  │存储插件  │  │代理插件  │  │ 第三方   │        │ │
│  │   │ gpu-share│  │storage   │  │proxy     │  │ 插件     │        │ │
│  │   └──────────┘  └──────────┘  └──────────┘  └──────────┘        │ │
│  └──────────────────────────────────────────────────────────────────┘ │
│                                                                        │
└────────────────────────────────────────────────────────────────────────┘
```

### 4.2 插件生命周期

```
┌────────────────────────────────────────────────────────────────────────┐
│                        插件生命周期                                     │
├────────────────────────────────────────────────────────────────────────┤
│                                                                        │
│   ┌─────────┐    ┌─────────┐    ┌─────────┐    ┌─────────┐           │
│   │ DISCOVER│───▶│ INSTALL │───▶│  INIT   │───▶│  START  │           │
│   │ 发现     │    │ 安装     │    │ 初始化   │    │ 运行     │           │
│   └─────────┘    └─────────┘    └─────────┘    └────┬────┘           │
│                                                       │                │
│                                                       ▼                │
│   ┌─────────┐    ┌─────────┐    ┌─────────┐    ┌─────────┐           │
│   │UNINSTALL│◀───│  STOP   │◀───│ PAUSE   │◀───│ RUNNING │           │
│   │ 卸载     │    │ 停止     │    │ 暂停     │    │         │           │
│   └─────────┘    └─────────┘    └─────────┘    └─────────┘           │
│                                                                        │
└────────────────────────────────────────────────────────────────────────┘
```

### 4.3 链上插件仓库

```solidity
// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

contract PluginRegistry {
    
    struct Plugin {
        string pluginId;
        string name;
        string version;
        string description;
        string manifestCID;      // 插件清单 IPFS CID
        string packageCID;        // 插件包 IPFS CID
        string author;
        uint256 price;            // 价格 (e-CNY, 最小单位)
        bytes32 checksum;        // 包校验和
        uint256 downloads;
        uint256 rating;
        bool isActive;
    }
    
    mapping(string => Plugin) public plugins;
    mapping(string => string[]) public versionHistory;
    
    event PluginPublished(
        string indexed pluginId,
        string version,
        string manifestCID,
        uint256 price
    );
    
    event PluginDownloaded(
        string indexed pluginId,
        address indexed user,
        uint256 amount
    );
    
    function publish(
        string memory pluginId,
        string memory name,
        string memory version,
        string memory description,
        string memory manifestCID,
        string memory packageCID,
        bytes32 checksum,
        uint256 price
    ) external;
    
    function download(
        string memory pluginId
    ) external payable returns (string memory);
    
    function updateRating(
        string memory pluginId,
        uint256 rating
    ) external;
}
```

### 4.4 插件安全

| 安全措施 | 描述 |
|---------|------|
| **代码签名** | 插件包必须经过私钥签名 |
| **权限控制** | 插件只能访问声明的权限 |
| **资源限制** | CPU/内存/磁盘使用上限 |
| **沙箱隔离** | 插件运行在隔离环境中 |
| **审计机制** | 插件行为可审计追溯 |

---

## 五、数字人民币支付集成

### 5.1 设计目标

- 支持 e-CNY (数字人民币) 作为主要支付方式
- 支付即时到账，无中间商
- 链上合约保障交易安全

### 5.2 支付架构

```
┌────────────────────────────────────────────────────────────────────────┐
│                      数字人民币支付架构                                   │
├────────────────────────────────────────────────────────────────────────┤
│                                                                        │
│   ┌──────────┐     ┌──────────┐     ┌──────────┐     ┌──────────┐    │
│   │ Consumer │────▶│  合约   │────▶│  支付   │────▶│ Provider │    │
│   │  (用户)  │     │(托管)   │     │  网关   │     │ (节点)   │    │
│   └──────────┘     └──────────┘     └──────────┘     └──────────┘    │
│        │                │                │                │            │
│        │  预授权        │  锁定资金      │  解锁转账      │            │
│        │  ─────────────▶│  ────────────▶│  ────────────▶│            │
│        │                │                │                │            │
│   ┌────────────────────────────────────────────────────────────┐       │
│   │                    e-CNY 钱包/账户                         │       │
│   └────────────────────────────────────────────────────────────┘       │
│                                                                        │
└────────────────────────────────────────────────────────────────────────┘
```

### 5.3 支付流程

```go
type PaymentFlow struct {
    // Step 1: 用户发起请求，预授权
    Step1_Authorize = "用户发起资源请求，合约预授权冻结对应金额"
    
    // Step 2: 服务交付
    Step2_Deliver = "节点提供服务，计量系统记录实际用量"
    
    // Step 3: 结算确认
    Step3_Settle = "用户确认服务完成，合约根据实际用量结算"
    
    // Step 4: 资金释放
    Step4_Release = "资金从合约转给节点，扣除平台服务费"
}
```

### 5.4 智能合约设计

```solidity
// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

contract PaymentEscrow {
    
    enum Status { 
        Created,        // 创建
        Funded,         // 已充值
        InProgress,     // 服务中
        Completed,      // 已完成
        Disputed,       // 争议中
        Refunded        // 已退款
    }
    
    struct Payment {
        bytes32 paymentId;
        address payer;
        address payee;
        uint256 amount;
        uint256 platformFee;      // 平台手续费
        uint256 releasedAmount;   // 已释放金额
        Status status;
        uint256 createdAt;
        uint256 completedAt;
        string resourceType;       // gpu/storage/proxy
        bytes resourceParams;     // 资源参数
    }
    
    mapping(bytes32 => Payment) public payments;
    
    // 创建支付
    function createPayment(
        address payee,
        uint256 amount,
        string memory resourceType,
        bytes memory resourceParams
    ) external returns (bytes32);
    
    // 确认服务开始
    function confirmService(bytes32 paymentId) external;
    
    // 更新计量数据
    function updateUsage(
        bytes32 paymentId,
        uint256 usedAmount
    ) external;
    
    // 完成支付
    function completePayment(
        bytes32 paymentId,
        uint256 finalAmount
    ) external;
    
    // 发起争议
    function raiseDispute(bytes32 paymentId) external;
    
    // 仲裁裁决
    function resolveDispute(
        bytes32 paymentId,
        uint256 payerAmount,
        uint256 payeeAmount
    ) external onlyArbitrator;
}
```

### 5.5 与数字人民币集成

```go
type ECNYIntegration struct {
    // 钱包地址
    WalletAddress string
    
    // 支付网关配置
    GatewayConfig GatewayConfig
    
    // 合约地址
    PaymentContract string
    RegistryContract string
}

type GatewayConfig struct {
    APIEndpoint string  // 数字人民币支付网关
    AppID       string  // 应用ID
    MerchantID  string  // 商户ID
}
```

---

## 六、P2P 网络设计

### 6.1 双层网络架构

```
┌────────────────────────────────────────────────────────────────────────┐
│                         P2P 双层网络架构                                 │
├────────────────────────────────────────────────────────────────────────┤
│                                                                        │
│   ┌────────────────────────────────────────────────────────────────┐   │
│   │                    控制平面 (Control Plane)                      │   │
│   │                                                                 │   │
│   │   功能: 节点发现、协议协商、心跳保活、服务注册                     │   │
│   │   协议: libp2p Kademlia DHT                                     │   │
│   │   端口: 38888                                                   │   │
│   │                                                                 │   │
│   │   消息类型:                                                     │   │
│   │   - PING/PONG: 心跳                                             │   │
│   │   - REGISTER: 节点注册                                           │   │
│   │   - DISCOVER: 节点发现                                          │   │
│   │   - NEGOTIATE: 协议协商                                         │   │
│   │                                                                 │   │
│   └────────────────────────────────────────────────────────────────┘   │
│                                    │                                   │
│                                    ▼                                   │
│   ┌────────────────────────────────────────────────────────────────┐   │
│   │                    数据平面 (Data Plane)                        │   │
│   │                                                                 │   │
│   │   功能: 实际数据传输、负载均衡、流量加密                          │   │
│   │   协议: KCP / WireGuard / libp2p                                │   │
│   │   端口: 38889                                                   │   │
│   │                                                                 │   │
│   │   隧道类型:                                                     │   │
│   │   - Direct: 直连                                               │   │
│   │   - NAT: NAT穿透打洞                                            │   │
│   │   - Relay: 中继转发                                             │   │
│   │                                                                 │   │
│   └────────────────────────────────────────────────────────────────┘   │
│                                                                        │
└────────────────────────────────────────────────────────────────────────┘
```

### 6.2 节点角色

```go
type NodeRole int

const (
    RoleConsumer NodeRole = 1 << iota   // 消费者：使用资源
    RoleProvider                        // 提供者：贡献资源
    RoleRelay                           // 中继：转发流量
    RoleValidator                       // 验证者：验证交易
)

type NodeInfo struct {
    NodeID       string
    Roles        NodeRole
    Endpoints    []Endpoint
    
    // 能力声明
    Capabilities []Capability
    
    // 信任分数
    TrustScore   float64
    
    // 在线状态
    LastSeen     time.Time
}

type Capability struct {
    Type    ResourceType  // gpu, storage, bandwidth
    Spec    ResourceSpec // 具体规格
    Price   PricingInfo  // 价格信息
}
```

### 6.3 NAT 穿透

```go
type NATTraversal struct {
    // NAT 类型检测
    DetectNATType() NATType
    
    // 打洞流程
    HolePunch(target NodeID) error
    
    // 中继Fallback
    SetupRelay() (RelayAddr, error)
}

type NATType int
const (
    NATNone NATType = iota  // 公网IP
    NATFullCone             // 全锥型
    NATRestricted           // 限制锥型
    NATPortRestricted       // 端口限制锥型
    NATSymmetric           // 对称型
)
```

---

*文档版本: v1.0*
*创建时间: 2026-04-01*
*作者: Kerrigan Team*
