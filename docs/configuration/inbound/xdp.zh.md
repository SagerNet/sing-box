`xdp` 入站通过 Linux AF_XDP 在网卡驱动层截获流量，完全绕过内核网络协议栈，适合高吞吐转发代理场景。

需要 Linux 5.9+ 内核及 `with_gvisor` 编译标签。

### 结构

```json
{
  "type": "xdp",
  "tag": "xdp-in",
  "interface": "eth0",
  "address": [
    "10.0.0.1/24"
  ],
  "route_address": [
    "0.0.0.0/0",
    "::/0"
  ],
  "route_exclude_address": [
    "192.168.0.0/16"
  ],
  "mtu": 1500,
  "frame_size": 4096,
  "frame_count": 4096,
  "udp_timeout": "5m"
}
```

### 字段

#### interface

==必填==

要挂载 XDP 程序的网络接口名称。

#### address

内部网络栈的本机 IP 地址（含前缀长度）。

留空时自动从网络接口检测，如果是网桥下的 Slave 网卡，需要手动指定 IP 地址

#### route_address

需要捕获并转发至 sing-box 的目标 CIDR 前缀列表（白名单）。

留空时捕获全部流量，等同于填写 `0.0.0.0/0` 和 `::/0`。

#### route_exclude_address

从捕获范围中排除的目标 CIDR 前缀列表（黑名单）。匹配的数据包将直接交由内核协议栈处理。

#### mtu

虚拟网络接口的 MTU。

默认：`1500`

#### frame_size

AF_XDP UMEM 帧大小（字节）。

默认：`4096`

#### frame_count

所有 RX 队列共享的 UMEM 帧总数。

默认：`4096`

#### udp_timeout

UDP 会话超时时间。

默认：`5m`
