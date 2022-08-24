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

关闭日志功能，开启后将无日志输出。

#### level

日志级别。可选参数有：`trace` `debug` `info` `warn` `error` `fatal` `panic`。

#### output

输出日志文件地址。开启后将不会在终端反馈日志信息。

#### timestamp

在输出的每一行加入时间戳。