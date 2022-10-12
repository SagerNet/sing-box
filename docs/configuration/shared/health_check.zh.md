### 结构

```json
{
  "interval": "60s",
  "sampling": 10,
  "timeout": "5s",
  "destination": "http://www.gstatic.com/generate_204",
  "connectivity": "http://connectivitycheck.platform.hicloud.com/generate_204",
  "max_rtt": "1000ms",
  "tolerance": 0.2
}
```

### 字段

#### interval

每个节点的健康检查间隔。不小于`10s`，默认为 `60s`。

#### sampling

对最近的多少次检查结果进行采样。大于 `0`，默认为 `10`。

#### timeout

健康检查超时时间。大于 `0s`，默认为 `5s`。

#### destination

健康检查目标。默认为 `http://www.gstatic.com/generate_204`。

#### connectivity

网络连通性检查地址。健康检查失败，可能是由于网络不可用造成的（比如断开 WIFI 连接）。设置此项，可避免此类情况的下将节点判定为失效。若不设置，则不会有此行为。

#### max_rtt

可接受的健康检查最大往返时间，超过此设定值的节点将被不被选择。 默认为 `0`，即接受任何节点。

#### tolerance

健康检查失败的容忍度。大于 `0`，小于 `1`，默认为 `0`。

若`sampling=10, tolerance=0.2`，则表示在最近的 10 次检查中，最多允许 2 次失败。
