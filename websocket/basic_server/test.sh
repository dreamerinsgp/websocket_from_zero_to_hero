#!/bin/bash

# WebSocket 基础服务器测试脚本

echo "=========================================="
echo "WebSocket 基础服务器测试"
echo "=========================================="
echo ""

# 颜色定义
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

# 检查服务器是否运行
check_server() {
    if curl -s http://localhost:8080/ws > /dev/null 2>&1; then
        return 0
    else
        return 1
    fi
}

# 测试连接和订阅
test_websocket() {
    echo -e "${YELLOW}测试 WebSocket 连接和订阅...${NC}"
    
    # 使用 Node.js 测试（如果可用）
    if command -v node &> /dev/null; then
        node << 'EOF'
const WebSocket = require('ws');

const ws = new WebSocket('ws://localhost:8080/ws');
let connected = false;
let subscribed = false;

ws.on('open', () => {
    console.log('✅ 连接成功');
    connected = true;
    
    // 订阅频道
    setTimeout(() => {
        ws.send(JSON.stringify({
            action: 'subscribe',
            channel: 'lottery:created'
        }));
        console.log('📤 发送订阅请求');
    }, 500);
});

ws.on('message', (data) => {
    const msg = JSON.parse(data.toString());
    console.log('📨 收到消息:', JSON.stringify(msg, null, 2));
    
    if (msg.action === 'connect' && msg.code === 200) {
        console.log('✅ 连接确认收到');
    }
    
    if (msg.action === 'subscribe' && msg.code === 200) {
        console.log('✅ 订阅确认收到');
        subscribed = true;
        
        // 测试完成，关闭连接
        setTimeout(() => {
            ws.close();
            process.exit(0);
        }, 1000);
    }
});

ws.on('error', (error) => {
    console.error('❌ 错误:', error.message);
    process.exit(1);
});

setTimeout(() => {
    if (!connected || !subscribed) {
        console.error('❌ 测试超时');
        process.exit(1);
    }
}, 5000);
EOF
    else
        echo -e "${YELLOW}Node.js 未安装，跳过 WebSocket 测试${NC}"
        echo "请使用浏览器打开 test_client.html 进行测试"
    fi
}

# 测试广播
test_broadcast() {
    echo ""
    echo -e "${YELLOW}测试广播功能...${NC}"
    
    response=$(curl -s -X POST http://localhost:8080/broadcast \
        -H "Content-Type: application/json" \
        -d '{
            "channel": "lottery:created",
            "data": {
                "lottery_id": "test-123",
                "name": "测试彩票",
                "status": "active"
            }
        }')
    
    if [ "$response" == "Broadcast sent" ]; then
        echo -e "${GREEN}✅ 广播测试成功${NC}"
    else
        echo -e "${RED}❌ 广播测试失败: $response${NC}"
    fi
}

# 主流程
echo "1. 检查服务器状态..."
if check_server; then
    echo -e "${GREEN}✅ 服务器正在运行${NC}"
else
    echo -e "${RED}❌ 服务器未运行${NC}"
    echo "请先运行: go run main.go"
    exit 1
fi

echo ""
echo "2. 测试 WebSocket 连接和订阅"
test_websocket

echo ""
echo "3. 测试广播功能"
test_broadcast

echo ""
echo -e "${GREEN}=========================================="
echo "测试完成！"
echo "==========================================${NC}"
echo ""
echo "提示："
echo "- 使用浏览器打开 test_client.html 进行交互式测试"
echo "- 使用 curl 测试广播: curl -X POST http://localhost:8080/broadcast -H 'Content-Type: application/json' -d '{\"channel\":\"lottery:created\",\"data\":{}}'"

