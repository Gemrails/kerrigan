# Kerrigan 通信架构文档

## 目录

1. [插件与节点通信模式](#1-插件与节点通信模式)
2. [Agent 插件任务分发](#2-agent-插件任务分发)
3. [代理插件网络通信](#3-代理插件网络通信)
4. [P2P 网络设计](#4-p2p-网络设计)
5. [数据流示例](#5-数据流示例)

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

## 2. Agent 插件任务分发

Agent 插件是 Kerrigan 的核心特性，支持将 AI 任务分发到运行 OpenClaw 或 DeerFlow 框架的远程节点。

### 2.1 Agent 插件架构

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                           Agent Plugin (Agent 插件)                              │
│                                                                             │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐   │
│  │   Registry   │  │  Dispatcher  │  │  OpenClaw    │  │   DeerFlow   │   │
│  │              │  │              │  │  Adapter     │  │   Adapter    │   │
│  │ • 节点注册   │  │ • 任务队列   │  │              │  │              │   │
│  │ • 能力匹配   │  │ • Worker池   │  │ • WebSocket  │  │ • HTTP API   │   │
│  │ • 心跳检测   │  │ • 结果回调   │  │ • Task Queue │  │ • LangGraph  │   │
│  │ • 负载均衡   │  │ • 超时控制   │  │ • Port 18789 │  │ • Port 8000  │   │
│  └──────────────┘  └──────────────┘  └──────────────┘  └──────────────┘   │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 2.2 任务分发流程

```
┌─────────┐    ┌─────────┐    ┌─────────┐    ┌─────────┐    ┌─────────┐
│Consumer │    │  Node   │    │ Agent   │    │Registry │    │ Remote  │
│  客户端 │    │ (本地)  │    │ Plugin  │    │  注册表  │    │ Agent   │
└────┬────┘    └────┬────┘    └────┬────┘    └────┬────┘    └────┬────┘
     │               │               │               │               │
     │ 1. 提交任务  │               │               │               │
     │──────────────►│               │               │               │
     │               │ 2. 能力匹配  │               │               │
     │               │──────────────►│──────────────►│               │
     │               │               │               │               │
     │               │ 3. 返回最优节点│◄──────────────│               │
     │               │◄──────────────│               │               │
     │               │               │               │               │
     │               │ 4. 分发任务  │               │               │
     │               │──────────────►│──────────────►│               │
     │               │               │               │               │
     │               │               │               │ 5. 执行任务  │
     │               │               │               │───────────────►│
     │               │               │               │               │
     │               │               │ 6. 异步结果  │◄──────────────│
     │               │◄──────────────│◄──────────────│               │
     │               │               │               │               │
     │ 7. 返回结果  │               │               │               │
     │◄──────────────│               │               │               │
```

### 2.3 核心组件

#### AgentNode 节点定义

```go
// 文件: internal/plugins/agent/registry.go

type AgentNode struct {
    NodeID        string                 // 节点 ID
    AgentType     AgentType            // "openclaw" 或 "deerflow"
    Endpoint      string                 // WebSocket 或 HTTP URL
    Capabilities  []AgentCapability     // 节点能力
    Status        AgentStatus           // online/busy/offline
    LastHeartbeat time.Time             // 最后心跳时间
    Load          float64               // 负载 0-1
}

// Agent 框架类型
type AgentType string
const (
    AgentTypeOpenClaw AgentType = "openclaw"
    AgentTypeDeerFlow AgentType = "deerflow"
)

// 节点能力
type AgentCapability string
const (
    CapWebSearch    AgentCapability = "web_search"
    CapCodeExec     AgentCapability = "code_execution"
    CapDeepResearch AgentCapability = "deep_research"
    CapImageGen     AgentCapability = "image_generation"
    CapSubAgent     AgentCapability = "sub_agent_orchestration"
)
```

#### Registry 注册表

```go
// 文件: internal/plugins/agent/registry.go

type Registry struct {
    mu    sync.RWMutex
    nodes map[string]*AgentNode
}

// 按能力查找节点
func (r *Registry) GetNodesByCapability(cap AgentCapability) []*AgentNode {
    // 返回具有特定能力的节点列表
}

// 按类型查找节点
func (r *Registry) GetNodesByType(agentType AgentType) []*AgentNode {
    // 返回特定框架类型的节点
}
```

#### Dispatcher 任务分发器

```go
// 文件: internal/plugins/agent/dispatcher.go

type Dispatcher struct {
    config   Config
    registry *Registry
    taskQueue chan *TaskRequest
    activeTasks map[string]*activeTask
}

// 任务请求
type TaskRequest struct {
    ID           string
    Type         string                 // "research", "code", "general"
    Prompt       string
    Capabilities []AgentCapability      // 所需能力
    Timeout      time.Duration
}

// 任务分发
func (d *Dispatcher) Dispatch(ctx context.Context, req *TaskRequest) (*TaskResult, error) {
    // 1. 选择最优节点
    // 2. 根据节点类型选择适配器
    // 3. 执行任务并返回结果
}
```

### 2.4 OpenClaw 适配器

OpenClaw 使用 WebSocket Gateway 协议 (端口 18789)：

```go
// 文件: internal/plugins/agent/openclaw.go

type OpenClawAdapter struct {
    endpoint string  // ws://host:18789
    conn    *websocket.Conn
}

// 任务请求
type OpenClawTaskRequest struct {
    TaskID   string `json:"task_id"`
    Prompt   string `json:"prompt"`
    Thinking string `json:"thinking,omitempty"` // "low", "medium", "high"
    Model    string `json:"model,omitempty"`
}

// WebSocket 消息格式
type OpenClawMessage struct {
    Type    string          `json:"type"` // "task", "status", "ping"
    Content json.RawMessage `json:"content,omitempty"`
}
```

### 2.5 DeerFlow 适配器

DeerFlow 使用 HTTP REST API (端口 8000)：

```go
// 文件: internal/plugins/agent/deerflow.go

type DeerFlowAdapter struct {
    baseURL string  // http://host:8000
    client  *http.Client
}

// 任务请求
type DeerFlowRequest struct {
    Query    string                 `json:"query"`
    TaskType string               `json:"task_type,omitempty"` // "research", "code"
    Options  map[string]interface{} `json:"options,omitempty"`
}

// API 端点
// POST /api/v1/tasks     - 创建任务
// GET  /api/v1/tasks/:id - 查询状态
// DELETE /api/v1/tasks/:id - 取消任务
```

### 2.6 支持的任务类型

| 任务类型 | 描述 | OpenClaw | DeerFlow |
|---------|------|----------|----------|
| `research` | 深度研究，自动网页搜索和源综合 | ✅ | ✅ |
| `code` | 代码生成、调试和执行 | ✅ | ✅ |
| `general` | 多轮对话和任务自动化 | ✅ | ✅ |
| `image-generation` | AI 图像生成 | ✅ | ✅ |
| `multi-step` | 多步骤复合任务 | ✅ (SubAgent) | ✅ (LangGraph) |

---

## 3. 代理插件网络通信

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

## 4. P2P 网络设计

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

## 5. 数据流示例

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

### 5.3 Agent 任务分发流程

```
┌─────────┐    ┌─────────┐    ┌─────────┐    ┌─────────┐    ┌─────────┐
│Consumer │    │  Node   │    │ Agent   │    │Registry │    │ Remote  │
│         │    │         │    │ Plugin  │    │         │    │ Agent   │
│ Client  │    │         │    │         │    │         │    │ Node    │
└────┬────┘    └────┬────┘    └────┬────┘    └────┬────┘    └────┬────┘
     │               │               │               │               │
     │ 1. Submit     │               │               │               │
     │    Task       │               │               │               │
     │──────────────►│               │               │               │
     │               │               │               │               │
     │               │ 2. Capability │               │               │
     │               │    Matching   │               │               │
     │               │──────────────►│──────────────►│               │
     │               │               │               │               │
     │               │ 3. Best Node │◄──────────────│               │
     │               │◄──────────────│               │               │
     │               │               │               │               │
     │               │ 4. Dispatch   │               │               │
     │               │──────────────►│──────────────►│               │
     │               │               │               │               │
     │               │               │               │ 5. Execute   │
     │               │               │               │───────────────►│
     │               │               │               │               │
     │               │               │ 6. Async      │◄──────────────│
     │               │◄──────────────│◄──────────────│               │
     │               │               │               │               │
     │ 7. Result     │               │               │               │
     │◄──────────────│               │               │               │
```

**OpenClaw vs DeerFlow 对比:**

| 特性 | OpenClaw | DeerFlow |
|------|----------|----------|
| **协议** | WebSocket | HTTP REST |
| **端口** | 18789 | 8000 |
| **任务队列** | ✅ 内置 | ❌ 轮询 |
| **LangGraph** | ❌ | ✅ |
| **子 Agent** | ✅ | ✅ |
| **适用场景** | 自动化、聊天 | 深度研究、代码生成 |

---

## 附录

### A. 关键文件路径

| 文件 | 说明 |
|------|------|
| `internal/plugins/plugins.go` | 插件接口定义 |
| `internal/plugins/gpu-share/plugin.go` | GPU 插件实现 |
| `internal/plugins/agent/plugin.go` | Agent 插件主实现 |
| `internal/plugins/agent/registry.go` | Agent 节点注册表 |
| `internal/plugins/agent/dispatcher.go` | Agent 任务分发器 |
| `internal/plugins/agent/openclaw.go` | OpenClaw 适配器 |
| `internal/plugins/agent/deerflow.go` | DeerFlow 适配器 |
| `internal/plugins/proxy/plugin.go` | 代理插件实现 |
| `internal/plugins/proxy/tunnel/tunnel.go` | 隧道服务实现 |
| `internal/core/plugin/interface.go` | 核心插件接口 |

### B. 默认端口配置

| 端口 | 用途 |
|------|------|
| 1080 | HTTP 代理 |
| 1081 | SOCKS5 代理 |
| 1082 | 隧道服务 |
| 18789 | OpenClaw WebSocket Gateway |
| 8000 | DeerFlow HTTP API |
| 3000 | DeerFlow Web UI |
| 38888 | P2P 控制平面 |
| 38889 | P2P 数据平面 |

---

*文档更新: 2026-04-10*
