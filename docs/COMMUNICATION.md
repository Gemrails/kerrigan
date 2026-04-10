# Kerrigan 通信架构文档

## 目录

1. [插件与节点通信模式](#1-插件与节点通信模式)
2. [代理插件网络通信](#2-代理插件网络通信)
3. [P2P 网络设计](#3-p2p-网络设计)
4. [数据流示例](#4-数据流示例)

---

## 1. 插件与节点通信模式

### 1.1 接口驱动架构

插件与节点之间通过 Go interface 进行进程内通信，无需网络开销。

```
┌─────────────────────────────────────────────────────────────┐
│                        Node 节点进程                          │
│                                                             │
│  ┌─────────────────────────────────────────────────────┐   │
│  │              Plugin Runtime (插件运行时)               │   │
│  │                                                     │   │
│  │   ┌─────────┐  ┌─────────┐  ┌─────────┐           │   │
│  │   │ GPU插件  │  │存储插件  │  │代理插件  │           │   │
│  │   │         │  │         │  │         │           │   │
│  │   │ Plugin  │  │ Plugin  │  │ Plugin  │           │   │
│  │   └────┬────┘  └────┬────┘  └────┬────┘           │   │
│  │        │            │            │                 │   │
│  │        ▼            ▼            ▼                 │   │
│  │   ┌─────────────────────────────────────┐          │   │
│  │   │       Plugin Interface (Go interface) │          │   │
│  │   │  • Init() / Start() / Stop()       │          │   │
│  │   │  • GetResourceProvider()           │          │   │
│  │   │  • GetTaskExecutor()               │          │   │
│  │   └─────────────────────────────────────┘          │   │
│  └─────────────────────────────────────────────────────┘   │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

### 1.2 核心接口定义

#### Plugin 接口 (插件基础接口)

```go
// 文件: internal/plugins/plugins.go

type Plugin interface {
    Name() string                           // 插件名称
    Version() string                        // 插件版本
    Initialize(configBytes []byte) error    // 初始化
    Shutdown() error                        // 关闭
}
```

#### ResourceProvider 接口 (资源提供者)

```go
type ResourceProvider interface {
    ListResources() ([]Resource, error)    // 列出可用资源
    GetResource(id string) (*Resource, error) // 获取单个资源
    AllocateResource(id string, memoryMB int) error  // 分配资源
    ReleaseResource(id string, memoryMB int) error   // 释放资源
}

type Resource struct {
    ID        string
    Type      string      // "GPU", "Storage", "Bandwidth"
    Name      string
    Capacity  float64     // 总容量
    Available float64     // 可用容量
    Metadata  map[string]interface{}
}
```

#### TaskExecutor 接口 (任务执行器)

```go
type TaskExecutor interface {
    ExecuteTask(ctx context.Context, task *Task) (*TaskResult, error)
}

type Task struct {
    ID               string
    Type             string    // "inference", "image-generation"
    RequiredMemoryMB int
    Metadata         map[string]interface{}
}

type TaskResult struct {
    Status string                 // "pending", "running", "completed", "failed"
    Output map[string]interface{} // 任务输出
    Error  string                 // 错误信息
}
```

### 1.3 插件实现示例

#### GPU 插件

```go
// 文件: internal/plugins/gpu-share/plugin.go

type GPUPlugin struct {
    config        Config
    gpus          map[string]*GPUInfo
    inferenceEng  *inference.Engine      // 推理引擎
    taskScheduler *scheduler.Scheduler    // 任务调度器
    pricingEngine *pricing.Engine         // 定价引擎
}

func (p *GPUPlugin) GetResourceProvider() plugins.ResourceProvider {
    return p
}

func (p *GPUPlugin) GetTaskExecutor() plugins.TaskExecutor {
    return p
}

// 任务执行流程
func (p *GPUPlugin) ExecuteTask(ctx context.Context, task *plugins.Task) (*plugins.TaskResult, error) {
    switch task.Type {
    case "inference":
        return p.executeInference(ctx, task)
    case "image-generation":
        return p.executeImageGeneration(ctx, task)
    // ...
    }
}
```

---

## 2. 代理插件网络通信

Proxy 插件包含独立的网络服务组件，支持多种协议和加密方式。

### 2.1 代理插件架构

```
┌─────────────────────────────────────────────────────────────┐
│                     Proxy Plugin (代理插件)                    │
│                                                             │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐      │
│  │ HTTP Proxy   │  │ SOCKS5 Proxy │  │ Tunnel Server │      │
│  │ Port: 1080   │  │ Port: 1081   │  │ Port: 1082    │      │
│  └──────┬───────┘  └──────┬───────┘  └──────┬───────┘      │
│         │                 │                 │               │
│         └─────────────────┼─────────────────┘               │
│                           │                                 │
│                           ▼                                 │
│               ┌─────────────────────┐                        │
│               │   Traffic Shaper    │  ← 流量限速             │
│               │   (流量整形器)       │                        │
│               └─────────────────────┘                        │
│                           │                                 │
│                           ▼                                 │
│               ┌─────────────────────┐                        │
│               │   Stats Collector   │  ← 计量计费             │
│               │   (统计收集器)       │                        │
│               └─────────────────────┘                        │
└─────────────────────────────────────────────────────────────┘
```

### 2.2 隧道协议支持

#### 配置文件

```go
type Config struct {
    // 代理端口
    HTTPProxyPort  int  `json:"http_proxy_port"`  // 默认: 1080
    SOCKS5Port     int  `json:"socks5_port"`        // 默认: 1081
    TunnelPort     int  `json:"tunnel_port"`        // 默认: 1082

    // 隧道配置
    EnableKCP       bool   `json:"enable_kcp"`        // KCP 协议
    EnableTLSObfusc bool   `json:"enable_tls_obfusc"` // TLS 混淆
    KCPSecret       string `json:"kcp_secret"`         // KCP 密钥

    // 流量限制
    DownloadSpeedKBPS int  `json:"download_speed_kbps"`
    UploadSpeedKBPS   int  `json:"upload_speed_kbps"`
}
```

#### 支持的协议

| 协议 | 说明 | 端口 | 用途 |
|------|------|------|------|
| **HTTP** | HTTP 透明代理 | 1080 | 网页浏览 |
| **SOCKS5** | 通用代理协议 | 1081 | 应用层代理 |
| **TCP** | 明文 TCP 隧道 | 1082 | 直接传输 |
| **KCP** | 可靠 UDP 隧道 | 1082 | 低延迟传输 |
| **TLS** | TLS 1.3 混淆 | 1082 | 流量伪装 |

### 2.3 加密机制

```go
// 文件: internal/plugins/proxy/tunnel/tunnel.go

// ChaCha20 加密
func Encrypt(data []byte, key []byte) ([]byte, error) {
    // ChaCha20-Poly1305 AEAD
    nonce := make([]byte, 24)
    rand.Read(nonce)
    block, _ := chacha20.NewXChaCha20(key, nonce)
    // ...
}

// KCP 加密 (AES)
func deriveKey(secret string) []byte {
    return pbkdf2.Key([]byte(secret), []byte("kerrigan-kcp-salt"), 4096, 32, sha256.New)
}
```

---

## 3. P2P 网络设计

### 3.1 双层网络架构

```
┌─────────────────────────────────────────────────────────────┐
│                    P2P Network Layer                        │
│                                                             │
│  ┌──────────────────────┐     ┌──────────────────────┐       │
│  │    Control Plane     │     │     Data Plane       │       │
│  │    (控制平面)         │     │    (数据平面)         │       │
│  ├──────────────────────┤     ├──────────────────────┤       │
│  │  Port: 38888         │     │  Port: 38889         │       │
│  ├──────────────────────┤     ├──────────────────────┤       │
│  │ • 节点发现           │     │ • 隧道传输           │       │
│  │   (Kademlia DHT)    │     │ • 负载均衡           │       │
│  │ • 协议协商          │     │ • 流量加密           │       │
│  │ • 心跳保活          │     │ • 代理转发           │       │
│  │ • 服务注册          │     │ • 数据中继           │       │
│  └──────────────────────┘     └──────────────────────┘       │
│                                                             │
│  ┌─────────────────────────────────────────────────────┐    │
│  │              NAT 穿透策略                            │    │
│  │   直连 (公网IP) → 打洞 (Hole Punching) → 中继Fallback │    │
│  └─────────────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────────┘
```

### 3.2 控制平面职责

| 功能 | 描述 |
|------|------|
| **节点发现** | Kademlia DHT 分布式哈希表 |
| **协议协商** | 版本兼容、能力交换 |
| **心跳保活** | 定期 ping/pong 维持连接 |
| **服务注册** | 通告可用资源和服务 |

### 3.3 数据平面职责

| 功能 | 描述 |
|------|------|
| **隧道传输** | 端到端加密隧道 |
| **负载均衡** | 多路径流量分配 |
| **流量加密** | ChaCha20/TLS 加密 |
| **代理转发** | 跨境/跨区域流量 |

### 3.4 NAT 穿透策略

```
优先级顺序:

1. 直连 (Direct Connection)
   └── 适用于: 公网 IP 节点
       条件: 双方都有公网 IP

2. 打洞 (Hole Punching)
   ├── UDP Hole Punching
   │   └── 适用于: 对称型 NAT
   └── TCP Hole Punching
       └── 适用于: 端口限制型 NAT

3. 中继 Fallback
   └── 适用于: 所有穿透失败情况
       机制: Relay 节点转发流量
```

---

## 4. 数据流示例

### 4.1 GPU 推理任务流程

```
┌─────────┐     ┌─────────┐     ┌─────────┐     ┌─────────┐
│ Consumer│     │  Node   │     │  Plugin │     │   GPU   │
│  客户端 │     │  节点   │     │  Runtime│     │  硬件   │
└────┬────┘     └────┬────┘     └────┬────┘     └────┬────┘
     │               │               │               │
     │ 1. 发起推理请求 │               │               │
     │──────────────►│               │               │
     │               │ 2. 查询可用GPU │               │
     │               │──────────────►│               │
     │               │               │ 3. 返回GPU列表  │
     │               │◄──────────────│               │
     │               │               │               │
     │               │ 4. 分配GPU资源│               │
     │               │──────────────►│               │
     │               │               │ 5. 锁定GPU内存 │
     │               │               │───────────────►│
     │               │               │               │
     │               │ 6. 下发推理任务│               │
     │               │──────────────►│               │
     │               │               │ 7. 执行推理    │
     │               │               │───────────────►│
     │               │               │               │
     │               │               │ 8. 返回结果    │
     │               │◄──────────────│               │
     │               │               │ 9. 释放GPU资源 │
     │               │               │───────────────►│
     │               │               │               │
     │ 10. 返回推理结果│               │               │
     │◄──────────────│               │               │
```

### 4.2 代理流量转发流程

```
┌─────────┐     ┌─────────┐     ┌─────────┐     ┌─────────┐
│ Consumer│     │ Provider│     │  Relay  │     │  Target │
│  客户端 │     │  节点   │     │  节点   │     │  目标   │
└────┬────┘     └────┬────┘     └────┬────┘     └────┬────┘
     │               │               │               │
     │ 1. 连接代理   │               │               │
     │──────────────►│               │               │
     │               │               │               │
     │ 2. KCP/TCP加密│隧道传输       │               │
     │═══════════════════════════════►               │
     │               │               │               │
     │               │ 3. 流量整形   │               │
     │               │──────────────►│               │
     │               │               │               │
     │               │ 4. 统计流量   │               │
     │               │──────────────►│               │
     │               │               │               │
     │               │ 5. 转发到目标  │               │
     │               │═══════════════════════════════►│
     │               │               │               │
     │               │               │ 6. 目标响应   │
     │               │◄═══════════════════════════════│
     │               │               │               │
     │ 7. 加密返回   │               │               │
     │◄═══════════════════════════════│               │
     │               │               │               │
```

---

## 附录

### A. 关键文件路径

| 文件 | 说明 |
|------|------|
| `internal/plugins/plugins.go` | 插件接口定义 |
| `internal/plugins/gpu-share/plugin.go` | GPU 插件实现 |
| `internal/plugins/proxy/plugin.go` | 代理插件实现 |
| `internal/plugins/proxy/tunnel/tunnel.go` | 隧道服务实现 |
| `internal/core/plugin/interface.go` | 核心插件接口 |

### B. 默认端口配置

| 端口 | 用途 |
|------|------|
| 1080 | HTTP 代理 |
| 1081 | SOCKS5 代理 |
| 1082 | 隧道服务 |
| 38888 | P2P 控制平面 |
| 38889 | P2P 数据平面 |

---

*文档更新: 2026-04-10*
