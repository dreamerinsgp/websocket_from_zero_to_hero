# 使用说明

## 快速开始

### 1. 启动服务器

```bash
cd basic_server
go run main.go
```

服务器将在 `http://localhost:8080` 启动

### 2. 测试方式

#### 方式一：使用浏览器测试页面（推荐）

1. 在浏览器中打开 `test_client.html`
2. 点击"连接"按钮
3. 订阅频道（默认：`lottery:created`）
4. 在另一个终端使用 curl 发送广播消息

#### 方式二：使用浏览器控制台

```javascript
// 连接
const ws = new WebSocket('ws://localhost:8080/ws');

// 监听消息
ws.onmessage = (e) => {
  console.log('收到:', JSON.parse(e.data));
};

// 连接成功后订阅
ws.onopen = () => {
  ws.send(JSON.stringify({
    action: 'subscribe',
    channel: 'lottery:created'
  }));
};
```

#### 方式三：使用测试脚本

```bash
./test.sh
```

### 3. 测试广播

在服务器运行的情况下，打开另一个终端：

```bash
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

订阅了 `lottery:created` 频道的客户端将收到这条消息。

## 功能验证清单

- [ ] 服务器启动成功
- [ ] WebSocket 连接成功
- [ ] 收到连接确认消息
- [ ] 订阅频道成功
- [ ] 收到订阅确认消息
- [ ] 发送广播消息
- [ ] 订阅客户端收到广播消息
- [ ] 心跳 (ping/pong) 工作正常
- [ ] 取消订阅功能正常
- [ ] 断开连接后自动清理

## 下一步

验证基础功能后，可以：
1. 添加认证机制
2. 添加消息持久化
3. 优化并发性能
4. 添加监控和日志
5. 逐步拆解，自己实现底层协议

