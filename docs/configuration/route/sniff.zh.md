如果在入站中启用，则可以嗅探连接的协议和域名（如果存在）。

#### 支持的协议

|   网络    |  协议  |     域名      |
|:-------:|:----:|:-----------:|
|   TCP   | HTTP |    Host     |
|   TCP   | TLS  | Server Name |
|   UDP   | QUIC | Server Name |
|   UDP   | STUN |      /      |
| TCP/UDP | DNS  |      /      |