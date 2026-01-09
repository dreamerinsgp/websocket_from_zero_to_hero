# WebSocket Server - Code Explanation

## Overview
This is a WebSocket server implementation in Go that supports:
- Client connections and disconnections
- Channel-based pub/sub messaging
- Heartbeat (ping/pong)
- Broadcasting messages to subscribed channels

---

## 1. Core Data Structures

### 1.1 Message Types

```go
type Message struct {
    Action  string      `json:"action"`   // Action type: "subscribe", "unsubscribe", "ping"
    Channel string      `json:"channel"`  // Channel name for pub/sub
    Data    interface{} `json:"data,omitempty"` // Optional payload
}
```
**Purpose**: Incoming message format from clients

```go
type Response struct {
    ClientID string      `json:"clientId"` // Unique client identifier
    Action   string      `json:"action"`   // Response action type
    Channel  string      `json:"channel"`  // Channel name (if applicable)
    Code     int         `json:"code"`     // Status code (200 = success)
    Msg      string      `json:"msg"`      // Status message
    Data     interface{} `json:"data,omitempty"` // Response payload
}
```
**Purpose**: Outgoing message format to clients

### 1.2 Client Structure

```go
type Client struct {
    ID       string              // Unique UUID identifier
    Conn     *websocket.Conn     // WebSocket connection handle
    Send     chan []byte         // Buffered channel for outgoing messages
    Channels map[string]bool     // Set of subscribed channels
}
```
**Purpose**: Represents a connected WebSocket client
- `ID`: Unique identifier for tracking
- `Conn`: The actual WebSocket connection
- `Send`: Buffered channel (256 capacity) for queuing messages
- `Channels`: Tracks which channels this client is subscribed to

### 1.3 Server Structure

```go
type Server struct {
    clients       map[*Client]bool            // All connected clients
    subscriptions map[string]map[*Client]bool // Channel -> Clients mapping
    register      chan *Client                // Channel for new client registration
    unregister    chan *Client                // Channel for client disconnection
    broadcast     chan BroadcastMsg           // Channel for broadcasting messages
    mu            sync.RWMutex                // Read-write lock for thread safety
}
```
**Purpose**: Central server that manages all clients and channels
- `clients`: Set of all active client connections
- `subscriptions`: Two-level map: channel name → set of subscribed clients
- `register/unregister/broadcast`: Channels for async event handling
- `mu`: Mutex for concurrent access protection

### 1.4 Broadcast Message

```go
type BroadcastMsg struct {
    Channel string      // Target channel name
    Data    interface{} // Message payload
}
```
**Purpose**: Structure for broadcasting messages to a channel

---

## 2. Connection Flow

### 2.1 Step 1: HTTP Upgrade to WebSocket

**Location**: `HandleWebSocket()` function (lines 139-171)

```go
func (s *Server) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
    // Upgrade HTTP connection to WebSocket
    conn, err := upgrader.Upgrade(w, r, nil)
```

**Process**:
1. Client sends HTTP request with WebSocket upgrade headers
2. `upgrader.Upgrade()` performs the handshake:
   - Validates `Upgrade: websocket` header
   - Validates `Connection: Upgrade` header
   - Checks `Sec-WebSocket-Key` and computes `Sec-WebSocket-Accept`
   - Switches protocol from HTTP to WebSocket
3. Returns a `*websocket.Conn` if successful

**Upgrader Configuration** (lines 14-20):
```go
var upgrader = websocket.Upgrader{
    ReadBufferSize:  1024,  // Buffer size for reading
    WriteBufferSize: 1024, // Buffer size for writing
    CheckOrigin: func(r *http.Request) bool {
        return true // Allows all origins (should restrict in production)
    },
}
```

### 2.2 Step 2: Client Creation

**Location**: `HandleWebSocket()` function (lines 147-153)

```go
client := &Client{
    ID:       uuid.New().String(),        // Generate unique ID
    Conn:     conn,                       // Store WebSocket connection
    Send:     make(chan []byte, 256),     // Create buffered send channel
    Channels: make(map[string]bool),      // Initialize empty channel subscriptions
}
```

**Purpose**: Creates a client object with:
- Unique UUID identifier
- WebSocket connection handle
- Buffered channel for outgoing messages (256 capacity prevents blocking)
- Empty channel subscription map

### 2.3 Step 3: Client Registration

**Location**: `HandleWebSocket()` function (line 156) → `Run()` function (lines 76-80)

```go
s.register <- client  // Send client to registration channel
```

**Registration Process** (in `Run()` goroutine):
```go
case client := <-s.register:
    s.mu.Lock()
    s.clients[client] = true  // Add to clients map
    s.mu.Unlock()
    log.Printf("客户端 %s 已连接，当前连接数: %d", client.ID, len(s.clients))
```

**Purpose**: 
- Adds client to the server's client map
- Thread-safe operation using mutex
- Logs connection event

### 2.4 Step 4: Connection Confirmation Response

**Location**: `HandleWebSocket()` function (lines 158-166)

```go
response := Response{
    ClientID: client.ID,
    Action:   "connect",
    Code:     200,
    Msg:      "success",
}
data, _ := json.Marshal(response)
client.Send <- data  // Queue response message
```

**Purpose**: Sends initial connection confirmation to client with their assigned ID

### 2.5 Step 5: Start Goroutines

**Location**: `HandleWebSocket()` function (lines 169-170)

```go
go s.writePump(client)  // Handle outgoing messages
go s.readPump(client)   // Handle incoming messages
```

**Purpose**: Starts two concurrent goroutines:
- `readPump`: Reads messages from client
- `writePump`: Writes messages to client

---

## 3. Message Reading (readPump)

**Location**: Lines 174-199

```go
func (s *Server) readPump(client *Client) {
    defer func() {
        s.unregister <- client  // Unregister on exit
        client.Conn.Close()     // Close connection
    }()

    for {
        _, message, err := client.Conn.ReadMessage()
        // Parse JSON message
        // Handle message based on action type
    }
}
```

**Process**:
1. **Read Loop**: Continuously reads WebSocket messages
2. **Error Handling**: On read error, triggers cleanup (unregister + close)
3. **Message Parsing**: Unmarshals JSON into `Message` struct
4. **Message Routing**: Calls `handleMessage()` to process based on action

**Key Points**:
- Runs in separate goroutine per client
- Blocks on `ReadMessage()` until data arrives
- Automatically cleans up on connection close

---

## 4. Message Writing (writePump)

**Location**: Lines 202-220

```go
func (s *Server) writePump(client *Client) {
    defer client.Conn.Close()

    for {
        select {
        case message, ok := <-client.Send:
            if !ok {
                // Channel closed, send close frame
                client.Conn.WriteMessage(websocket.CloseMessage, []byte{})
                return
            }
            // Write message to WebSocket connection
            client.Conn.WriteMessage(websocket.TextMessage, message)
        }
    }
}
```

**Process**:
1. **Select Loop**: Waits for messages on `client.Send` channel
2. **Channel Closed**: If channel closes, sends WebSocket close frame and exits
3. **Message Writing**: Writes JSON message as WebSocket text frame

**Key Points**:
- Runs in separate goroutine per client
- Uses buffered channel to prevent blocking senders
- Handles graceful shutdown when channel closes

---

## 5. Message Handling

**Location**: `handleMessage()` function (lines 223-234)

```go
func (s *Server) handleMessage(client *Client, msg *Message) {
    switch msg.Action {
    case "subscribe":
        s.handleSubscribe(client, msg.Channel)
    case "unsubscribe":
        s.handleUnsubscribe(client, msg.Channel)
    case "ping":
        s.handlePing(client)
    default:
        log.Printf("未知操作: %s", msg.Action)
    }
}
```

### 5.1 Subscribe Action

**Location**: `handleSubscribe()` function (lines 237-262)

**Process**:
1. **Lock**: Acquires write lock for thread safety
2. **Add to Client**: Adds channel to client's subscription map
3. **Add to Server**: Adds client to channel's subscription map
4. **Response**: Sends confirmation response

```go
// Add channel to client's subscriptions
client.Channels[channel] = true

// Add client to channel's subscribers
if s.subscriptions[channel] == nil {
    s.subscriptions[channel] = make(map[*Client]bool)
}
s.subscriptions[channel][client] = true

// Send confirmation
response := Response{
    ClientID: client.ID,
    Action:   "subscribe",
    Channel:  channel,
    Code:     200,
    Msg:      "success",
}
```

### 5.2 Unsubscribe Action

**Location**: `handleUnsubscribe()` function (lines 265-292)

**Process**:
1. **Remove from Client**: Deletes channel from client's map
2. **Remove from Server**: Deletes client from channel's subscribers
3. **Cleanup**: Removes empty channel entries
4. **Response**: Sends confirmation

```go
// Remove from client
delete(client.Channels, channel)

// Remove from channel
if subs, ok := s.subscriptions[channel]; ok {
    delete(subs, client)
    if len(subs) == 0 {
        delete(s.subscriptions, channel)  // Cleanup empty channels
    }
}
```

### 5.3 Ping Action (Heartbeat)

**Location**: `handlePing()` function (lines 295-304)

**Process**:
- Responds with "pong" action
- Used for connection health checks

```go
response := Response{
    ClientID: client.ID,
    Action:   "pong",
    Code:     200,
    Msg:      "success",
}
```

---

## 6. Broadcasting/Pushing Messages

### 6.1 Broadcast Entry Point

**Location**: `BroadcastToChannel()` function (lines 307-312)

```go
func (s *Server) BroadcastToChannel(channel string, data interface{}) {
    s.broadcast <- BroadcastMsg{
        Channel: channel,
        Data:    data,
    }
}
```

**Purpose**: Public API for broadcasting to a channel (non-blocking)

### 6.2 Broadcast Processing

**Location**: `Run()` function (lines 100-134)

**Process**:
1. **Receive**: Gets broadcast message from channel
2. **Lock**: Acquires read lock
3. **Get Subscribers**: Retrieves all clients subscribed to channel
4. **Copy List**: Creates copy to avoid holding lock during sends
5. **Unlock**: Releases lock before sending
6. **Send**: Iterates through clients and sends message

```go
case msg := <-s.broadcast:
    s.mu.RLock()
    subs, ok := s.subscriptions[msg.Channel]
    if !ok {
        // No subscribers, skip
        continue
    }
    
    // Copy subscriber list
    clients := make([]*Client, 0, len(subs))
    for client := range subs {
        clients = append(clients, client)
    }
    s.mu.RUnlock()

    // Create response message
    response := Response{
        Action:  "message",
        Channel: msg.Channel,
        Code:    200,
        Msg:     "success",
        Data:    msg.Data,
    }
    data, _ := json.Marshal(response)
    
    // Send to all subscribers
    for _, client := range clients {
        select {
        case client.Send <- data:
            // Success
        default:
            // Channel full, close connection
            close(client.Send)
            s.unregister <- client
        }
    }
```

**Key Points**:
- **Non-blocking Send**: Uses `select` with `default` to avoid blocking
- **Failure Handling**: If send channel is full, closes connection
- **Thread Safety**: Uses read lock, copies list, then unlocks before sending

---

## 7. Client Disconnection

**Location**: `Run()` function (lines 82-98)

**Process**:
1. **Trigger**: `readPump` detects connection close and sends to `unregister` channel
2. **Cleanup**:
   - Removes client from `clients` map
   - Closes `Send` channel (signals `writePump` to exit)
   - Removes client from all subscribed channels
   - Cleans up empty channels

```go
case client := <-s.unregister:
    s.mu.Lock()
    if _, ok := s.clients[client]; ok {
        delete(s.clients, client)
        close(client.Send)  // Signal writePump to exit
        
        // Remove from all channel subscriptions
        for channel := range client.Channels {
            if subs, ok := s.subscriptions[channel]; ok {
                delete(subs, client)
                if len(subs) == 0 {
                    delete(s.subscriptions, channel)
                }
            }
        }
    }
    s.mu.Unlock()
```

---

## 8. Server Lifecycle

### 8.1 Server Initialization

**Location**: `main()` function (lines 314-350)

```go
server := NewServer()  // Create server instance
go server.Run()        // Start event loop in goroutine
```

### 8.2 Event Loop (Run)

**Location**: `Run()` function (lines 73-136)

**Purpose**: Central event dispatcher running in separate goroutine

**Event Types**:
1. **register**: New client connected
2. **unregister**: Client disconnected
3. **broadcast**: Message to broadcast to channel

**Pattern**: Uses `select` statement to handle multiple channels concurrently

### 8.3 HTTP Routes

```go
http.HandleFunc("/ws", server.HandleWebSocket)  // WebSocket endpoint
http.HandleFunc("/broadcast", ...)              // HTTP API for broadcasting
```

**Broadcast Endpoint** (lines 322-340):
- Accepts POST requests with JSON body
- Allows external systems to broadcast messages
- Format: `{"channel": "channel_name", "data": {...}}`

---

## 9. Thread Safety

**Mutex Usage**:
- **Write Lock (`Lock()`)**: Used when modifying maps (register, unregister, subscribe, unsubscribe)
- **Read Lock (`RLock()`)**: Used when reading maps (broadcast)
- **No Lock**: When sending messages (after copying client list)

**Why This Design**:
- Prevents race conditions when multiple goroutines access shared data
- Read locks allow concurrent reads
- Write locks ensure exclusive access for modifications

---

## 10. Message Flow Diagram

```
Client                    Server
  |                         |
  |--HTTP Request---------->|
  |                         | Upgrade to WebSocket
  |<--WebSocket Upgrade-----|
  |                         |
  |--WebSocket Connected--->|
  |                         | Create Client
  |                         | Register Client
  |<--Connect Response------| (with ClientID)
  |                         |
  |--Subscribe Message----->|
  |                         | Add to Channel
  |<--Subscribe Response----|
  |                         |
  |--Ping Message---------->|
  |<--Pong Response---------|
  |                         |
  |                         | Broadcast Message
  |<--Message Broadcast-----| (to all subscribers)
  |                         |
  |--Unsubscribe Message-->|
  |                         | Remove from Channel
  |<--Unsubscribe Response--|
  |                         |
  |--Close Connection------>|
  |                         | Unregister Client
  |                         | Cleanup Subscriptions
```

---

## 11. Key Design Patterns

1. **Goroutine per Connection**: Each client has dedicated read/write goroutines
2. **Channel-based Communication**: Uses channels for async event handling
3. **Pub/Sub Pattern**: Channel-based subscription model
4. **Hub Pattern**: Central server manages all clients and routing
5. **Buffered Channels**: Prevents blocking on message sends

---

## 12. Error Handling

- **Connection Errors**: Detected in `readPump`, triggers cleanup
- **Send Failures**: If `client.Send` is full, connection is closed
- **JSON Parsing Errors**: Logged and skipped, connection continues
- **Unknown Actions**: Logged but connection remains active

---

This architecture provides a scalable, thread-safe WebSocket server with pub/sub capabilities suitable for real-time applications.

