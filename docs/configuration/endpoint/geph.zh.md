`geph` 端点以数据包/VPN 模式启动 Geph5，并通过标准输入输出连接到 Sing-box。

Geph5 的 stdio 协议由原始 IPv4/IPv6 数据包组成，每个数据包前面带有一个 16 位大端长度。Sing-box 提供用户态 IP 协议栈，将普通 TCP 和 UDP 端点操作转换为这些数据包。

### 结构

```json
{
  "type": "geph",
  "tag": "geph5",
  "executable_path": "/usr/bin/geph5-client",
  "config_path": "/etc/geph5/client.yaml",
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

#### extra_args

额外的 Geph5 命令行参数。Sing-box 始终提供 `--config` 和 `--stdio-vpn`，不要在此重复这些托管参数。

#### startup_timeout

托管客户端进程的启动超时时间。进程必须成功启动后端点才可用。

默认值：`15s`。

