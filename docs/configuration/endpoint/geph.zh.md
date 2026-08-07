`geph` 端点以数据包/VPN 模式启动 Geph5，并通过标准输入输出连接到 Sing-box。

Geph5 的 stdio 协议由原始 IPv4/IPv6 数据包组成，每个数据包前面带有一个 16 位大端长度。Sing-box 提供用户态 IP 协议栈，将普通 TCP 和 UDP 端点操作转换为这些数据包。

### 结构

```json
{
  "type": "geph",
  "tag": "geph5",
  "executable_path": "/usr/bin/geph5-client",
  "config_path": "/etc/geph5/client.yaml",
  "control_address": "127.0.0.1:9913",
  "startup_timeout": "15s",
  "extra_args": []
}
```

### 字段

#### executable_path

Geph5 客户端可执行文件路径。

默认值：`geph5-client`。

#### config_path

必填。传递给 Geph5 `--config` 选项的 YAML 配置文件路径。

#### control_address

Geph5 控制 RPC 监听地址。

必填，且必须是回环地址（例如 `127.0.0.1:9913`）。

Geph5 YAML 里需将 `control_listen` 设置为同一地址。端点启动时该地址必须未被占用；Sing-box 会拒绝已被占用的监听地址，避免误用其他 Geph 进程的就绪状态。Geph5 的 TCP 控制 RPC 不提供身份验证，因此必须使用回环地址，并限制其他本地用户对主机的访问。

#### extra_args

额外的 Geph5 命令行参数。Sing-box 始终提供 `--config` 和 `--stdio-vpn`，不要在此重复这些托管参数。

#### startup_timeout

启动 Geph5 并等待控制 RPC 的 `conn_info` 状态变为 `Connected` 的超时时间。Geph 建立至少一个经过身份验证的隧道会话后，端点才可用。

默认值：`15s`。
