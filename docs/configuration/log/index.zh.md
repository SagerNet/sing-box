# 日志

### 结构

```json
{
  "log": {
    "disabled": false,
    "level": "info",
    "output": "box.log",
    "timestamp": true,
    "access": {
      "enabled": true,
      "path": "access"
    }
  }
}

```

### 字段

#### disabled

禁用日志，启动后不输出日志。

#### level

日志等级，可选值：`trace` `debug` `info` `warn` `error` `fatal` `panic`。

#### output

输出文件路径，启动后将不输出到控制台。

#### timestamp

添加时间到每行。

#### access

独立的访问日志输出。

### Access 字段

#### enabled

启用访问日志输出。

#### path

访问日志目录。文件以 JSON Lines 格式输出，按小时切分为 `access-YYYY-MM-DD-HH.log`，并保留 7 天。
