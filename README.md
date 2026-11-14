# Homey

Homey 是基于 Gorilla WebSocket 的 Go 框架，提供 Echo 接入、消息路由、连接管理和可选的 Redis 转发。

## 运行示例

从仓库根目录执行以下命令，再用浏览器打开 `http://localhost:8080`。

```powershell
cd example/server
go run .
```

示例从 `conf/homey.yaml` 加载配置，通过 `homey.NewWithConfig(cfg)` 创建实例，使用 `e.GET("/ws", h.Echo())` 注册端点。应用关停时调用 `h.Stop()`，它关闭已注册的 WebSocket、停止 worker 接收新任务，并取消 Redis 订阅。应用自己的 HTTP 服务仍由应用负责关停。使用 `homey.New()` 时，框架从默认配置及工作目录下可选的 `conf/homey.yaml` 取得配置。

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

`worker_pool_size` 控制 worker 数量。设为 0 时，每个连接按接收顺序在读取循环中执行 handler。启用 worker 后，同一连接的消息由同一个 worker 顺序处理。`max_worker_task_len` 控制每个 worker 的队列容量，正容量队列满时关闭产生该请求的连接，容量为 0 时等候 worker 接收或连接取消。业务 handler 应及时返回，并通过连接的 context 响应取消。

启用 `distribute.status` 后，调用 `h.Distribute()` 并处理返回的错误。Redis 转发消息在上述格式之前携带一个 8 字节目标连接 ID，经 Base64 编码后发布。服务端转发给 WebSocket 客户端时移除目标连接 ID。分布式模式使用 Sonyflake。为每个同时运行的 Homey 实例配置唯一的 `distribute.machine_id`，取值为 1 到 65535。

## 测试

```sh
go test -race -timeout 60s ./...
```

集成测试通过本机随机端口建立 WebSocket 连接，运行环境需要允许本机回环连接。
