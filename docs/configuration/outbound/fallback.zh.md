### 结构

```json
{
  "type": "fallback",
  "tag": "fb",

  "outbounds": [
    "primary",
    "backup"
  ],
  "url": "",
  "interval": "",
  "idle_timeout": "",
  "interrupt_exist_connections": false
}
```

### 字段

#### outbounds

==必填==

按优先级排列的出站标签列表。

#### url

用于检查连接的链接。默认使用 `https://www.gstatic.com/generate_204`。

#### interval

检查间隔。默认使用 `3m`。

#### idle_timeout

空闲超时。默认使用 `30m`。

#### interrupt_exist_connections

当选定的出站发生改变时，中断现有连接。

仅入站连接受此设置影响，内部连接将始终被中断。
