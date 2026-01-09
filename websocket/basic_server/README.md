# 基础 WebSocket Server

这是一个使用 `gorilla/websocket` 库实现的最基础的 WebSocket 服务器，支持：
- ✅ WebSocket 客户端连接
- ✅ 频道订阅/取消订阅
- ✅ 消息通信
- ✅ 频道广播

## 功能特性

1. **连接管理**
   - 自动分配唯一客户端ID
   - 连接确认消息
   - 自动清理断开连接

2. **订阅管理**
   - 订阅频道（subscribe）
   - 取消订阅（unsubscribe）
   - 频道订阅确认

3. **消息通信**
   - 支持 JSON 消息格式
   - 心跳机制（ping/pong）
   - 频道广播

## 快速开始

### 1. 安装依赖

```bash
cd basic_server
go mod tidy
```

### 2. 运行服务器

```bash
go run main.go
```

服务器将在 `http://localhost:8080` 启动，WebSocket 端点为 `ws://localhost:8080/ws`

### 3. 测试连接

#### 使用浏览器控制台测试

```javascript
// 1. 连接
const ws = new WebSocket('ws://localhost:8080/ws');

// 2. 监听消息
ws.onmessage = (event) => {
  const msg = JSON.parse(event.data);
  console.log('收到消息:', msg);
};

// 3. 等待连接确认
ws.onopen = () => {
  console.log('连接已建立');
  
  // 4. 订阅频道
  ws.send(JSON.stringify({
    action: 'subscribe',
    channel: 'lottery:created'
  }));
};

// 5. 发送心跳
ws.send(JSON.stringify({
  action: 'ping'
}));
```

#### 使用 curl 测试广播

```bash
# 向频道广播消息
curl -X POST http://localhost:8080/broadcast \
  -H "Content-Type: application/json" \
  -d '{
    "channel": "lottery:created",
    "data": {
      "lottery_id": "123",
      "name": "测试彩票",
      "status": "active"
    }
  }'
```

## 消息格式

### 客户端 → 服务器

**订阅频道**
```json
{
  "action": "subscribe",
  "channel": "lottery:created"
}
```

**取消订阅**
```json
{
  "action": "unsubscribe",
  "channel": "lottery:created"
}
```

**心跳**
```json
{
  "action": "ping"
}
```

### 服务器 → 客户端

**连接确认**
```json
{
  "clientId": "uuid",
  "action": "connect",
  "code": 200,
  "msg": "success"
}
```

**订阅确认**
```json
{
  "clientId": "uuid",
  "action": "subscribe",
  "channel": "lottery:created",
  "code": 200,
  "msg": "success"
}
```

**频道消息**
```json
{
  "action": "message",
  "channel": "lottery:created",
  "code": 200,
  "msg": "success",
  "data": {
    "lottery_id": "123",
    "name": "测试彩票"
  }
}
```

**心跳响应**
```json
{
  "clientId": "uuid",
  "action": "pong",
  "code": 200,
  "msg": "success"
}
```

## 代码结构

```
basic_server/
├── main.go          # 主程序
├── go.mod           # Go模块定义
└── README.md        # 说明文档
```

## 核心组件

1. **Server**: 服务器主结构，管理所有连接和订阅
2. **Client**: 客户端连接结构
3. **Message**: 消息格式定义
4. **Response**: 响应格式定义

## 测试流程

1. 启动服务器
2. 客户端连接 WebSocket
3. 收到连接确认
4. 客户端订阅频道
5. 收到订阅确认
6. 服务器广播消息到频道
7. 订阅该频道的客户端收到消息

## 下一步

这个基础版本验证了核心功能。之后可以：
1. 添加认证机制
2. 添加消息持久化
3. 优化并发性能
4. 添加监控和日志
5. 逐步拆解，自己实现底层协议

