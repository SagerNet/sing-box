### 服务器要求

* Linux
* `brutal` 拥塞控制算法内核模块已安装

参阅 [tcp-brutal](https://github.com/apernet/tcp-brutal)。

### 结构

```json
{
  "enabled": true,
  "up_mbps": 100,
  "down_mbps": 100
}
```

### 字段

#### enabled

启用 TCP Brutal 拥塞控制算法。

#### up_mbps, down_mbps

==必填==

上传和下载带宽，以 Mbps 为单位。
