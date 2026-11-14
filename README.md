# Homey

Homey 是基于 Gorilla WebSocket 的 Go 框架，提供标准库 HTTP 接入、消息路由、连接管理和可选的 Redis 转发。需要 Go 1.26 或更新版本。

## 运行示例

从仓库根目录执行以下命令，再用浏览器打开 `http://localhost:8080`。

```powershell
cd example/server
go run .
```

示例从 `conf/homey.yaml` 加载配置，通过 `homey.NewWithConfig(cfg)` 创建实例，并在 Echo v5 中调用 `h.ServeHTTP`。标准库可用 `mux.Handle("/ws", h)`，Echo v4 应用可继续用 `e.GET("/ws", h.Echo())`。使用 `homey.New()` 时，框架从默认配置及工作目录下可选的 `conf/homey.yaml` 取得配置。

应用关停时调用 `h.Shutdown(ctx)`。它停止接入、尝试发送 WebSocket Going Away 控制消息，并等待连接处理、关闭回调、worker 和 Redis 订阅退出。关停会取消尚未执行的任务，运行中的 handler 应响应连接 context。超时会返回 context 错误，后台关停会继续。需要仅触发关闭和资源清理时可以用 `h.Stop()`。应用自己的 HTTP 服务由应用负责关停。

## 消息与路由

每条 WebSocket 消息包含一个 Homey 消息。客户端的 WebSocket 消息类型必须与服务端的 `message.format` 一致。

| 字段 | 大小 | 出现条件 |
| --- | --- | --- |
| 类型 | 4 字节 | `tlv.type: true`，用于选择路由。省略时使用路由 0 |
| 长度 | 4 字节 | `tlv.length: true`，值等于 payload 的字节数 |
| Payload | 剩余字节 | 文本模式使用 UTF-8，二进制模式使用应用定义的数据 |

字段顺序与表格一致。整数的字节序由 `message.endian` 决定。默认使用文本模式、路由 0 和原始 payload。启用二进制包头时需将 `message.format` 设为 `binary`。应用可用 `h.Pack` 按实例配置编码，再用 `Connection.SendMsg` 发送。

`SendMsg` 将数据复制到连接的发送队列。返回 `nil` 表示已入队。队列满或连接关闭时，分别返回 `network.ErrSendQueueFull` 或 `network.ErrConnectionClosed`。

## Worker 与 Redis

`worker_pool_size` 控制共享 worker 数量。每个连接有一个容量为 64 的请求队列，队列满时关闭连接。设为 0 时，连接自己的 goroutine 按顺序执行 handler。启用 worker 后，同一连接的请求交给同一个 worker 顺序处理。`max_worker_task_len` 控制每个 worker 的队列容量，正容量队列满时关闭产生该请求的连接，容量为 0 时由连接的分发 goroutine 等候 worker 接收。WebSocket 读取循环独立处理 Ping、Pong 和 Close。

启用 `distribute.status` 后，调用 `h.Distribute()` 并处理返回的错误。订阅收到 Redis 确认后才返回成功。使用 `h.PublishForwardMsg(ctx, data)` 或 `h.PublishWorldMsg(ctx, data)` 按当前实例的 Redis 配置发布。两个频道均按目标连接 ID 转发。消息在上述格式之前携带一个 8 字节目标连接 ID，经 Base64 编码后发布。服务端转发给 WebSocket 客户端时移除目标连接 ID。Redis Pub/Sub 为至多一次投递，断线期间的消息不会重放。为每个同时运行的 Homey 实例配置唯一的 `distribute.machine_id`，取值为 1 到 65535。

## gRPC 跨实例单播

`rpcforward` 提供可选的 gRPC 后端。每个节点使用独立的 `nodeID`，并通过 `forwardv1.RegisterForwarderServer(grpcServer, forwarder)` 在内部 gRPC 监听端口注册服务。应用负责为 gRPC 服务和客户端配置传输凭据、认证以及关停。

调用 `rpcforward.New(h, nodeID, resolvePeer, timeout)` 创建转发器。固定部署可以用 `map[string]*grpc.ClientConn` 保存节点到连接的映射，并在 `resolvePeer(ctx, nodeID)` 中按节点查表。使用 `rpcforward.DialPeer` 创建并复用客户端连接，由应用在退出时关闭。动态部署可以把静态表换成服务发现。每个地址应指向具体实例。

```go
result, err := forwarder.Send(ctx, rpcforward.Target{
    NodeID:       "node-b",
    ConnectionID: connID,
}, payload)
```

本机目标直接查本地连接表，远端目标调用 gRPC。Payload 是可以直接交给 `Connection.SendMsg` 的 WebSocket 消息。业务结果包括 `QUEUED`、`CONNECTION_NOT_FOUND` 和 `QUEUE_FULL`。`QUEUED` 仅确认目标节点已经入队。超时或传输中断时，入队结果可能未知。默认客户端关闭可配置的 RPC 重试。

如果按用户发送，由业务维护用户到 `nodeID + connID` 的当前映射，并在连接重建时更新。单机模式的连接 ID 计数会在进程重启后重置，部署时可以让节点标识包含进程实例 ID，避免旧路由命中新连接。Redis 转发继续通过原有 API 使用。

接口定义见 `rpcforward/v1/forward.proto`。从仓库根目录使用 `protoc --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative rpcforward/v1/forward.proto` 生成 Go 代码。

## 测试

```sh
go test -race -timeout 60s ./...
```

集成测试通过本机随机端口建立 WebSocket 连接，运行环境需要允许本机回环连接。
设置 `HOMEY_TEST_REDIS_ADDR` 可运行双实例 Redis 集成测试。测试会主动断开 Redis 的 Pub/Sub 客户端，请使用专用测试实例。GitHub Actions 在 Linux 上验证两个 Go 模块，并执行竞态检测和依赖扫描。
