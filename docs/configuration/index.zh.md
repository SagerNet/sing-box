# 引言

sing-box 使用 JSON 作为配置文件格式。

### 结构

```json
{
  "log": {},
  "dns": {},
  "inbounds": [],
  "outbounds": [],
  "route": {},
  "experimental": {}
}
```

### 字段

| Key            | Format                |
|----------------|-----------------------|
| `log`          | [日志](./log)           |
| `dns`          | [DNS](./dns)          |
| `inbounds`     | [入站](./inbound)       |
| `outbounds`    | [出站](./outbound)      |
| `route`        | [路由](./route)         |
| `experimental` | [实验性](./experimental) |

### 检查

```bash
sing-box check
```

### 格式化

```bash
sing-box format -w -c config.json -D config_directory
```

### 合并

```bash
sing-box merge output.json -c config.json -D config_directory
```