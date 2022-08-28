# 日志

### 结构

```json
{
  "log": {
    "disabled": false,
    "level": "info",
    "output": "box.log",
    "timestamp": true
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