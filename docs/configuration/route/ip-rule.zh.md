### 结构

```json
{
  "route": {
    "ip_rules": [
      {
        "inbound": [
          "mixed-in"
        ],
        "ip_version": 6,
        "network": [
          "tcp"
        ],
        "domain": [
          "test.com"
        ],
        "domain_suffix": [
          ".cn"
        ],
        "domain_keyword": [
          "test"
        ],
        "domain_regex": [
          "^stun\\..+"
        ],
        "geosite": [
          "cn"
        ],
        "source_geoip": [
          "private"
        ],
        "geoip": [
          "cn"
        ],
        "source_ip_cidr": [
          "10.0.0.0/24",
          "192.168.0.1"
        ],
        "ip_cidr": [
          "10.0.0.0/24",
          "192.168.0.1"
        ],
        "source_port": [
          12345
        ],
        "source_port_range": [
          "1000:2000",
          ":3000",
          "4000:"
        ],
        "port": [
          80,
          443
        ],
        "port_range": [
          "1000:2000",
          ":3000",
          "4000:"
        ],
        "invert": false,
        "action": "direct",
        "outbound": "wireguard"
      },
      {
        "type": "logical",
        "mode": "and",
        "rules": [],
        "invert": false,
        "action": "direct",
        "outbound": "wireguard"
      }
    ]
  }
}

```

!!! note ""

    当内容只有一项时，可以忽略 JSON 数组 [] 标签。

### Default Fields

!!! note ""

    默认规则使用以下匹配逻辑:  
    (`domain` || `domain_suffix` || `domain_keyword` || `domain_regex` || `geosite` || `geoip` || `ip_cidr`) &&  
    (`port` || `port_range`) &&  
    (`source_geoip` || `source_ip_cidr`) &&  
    (`source_port` || `source_port_range`) &&  
    `other fields`

#### inbound

[入站](/zh/configuration/inbound) 标签。

#### ip_version

4 或 6。

默认不限制。

#### network

匹配网络协议。

可用值：

* `tcp`
* `udp`
* `icmpv4`
* `icmpv6`

#### domain

匹配完整域名。

#### domain_suffix

匹配域名后缀。

#### domain_keyword

匹配域名关键字。

#### domain_regex

匹配域名正则表达式。

#### geosite

匹配 GeoSite。

#### source_geoip

匹配源 GeoIP。

#### geoip

匹配 GeoIP。

#### source_ip_cidr

匹配源 IP CIDR。

#### ip_cidr

匹配 IP CIDR。

#### source_port

匹配源端口。

#### source_port_range

匹配源端口范围。

#### port

匹配端口。

#### port_range

匹配端口范围。

#### invert

反选匹配结果。

#### action

==必填==

| Action | 描述                  |
|--------|---------------------|
| return | 停止 IP 路由并将该连接组装到传输层 |
| block  | 屏蔽该连接               |
| direct | 直接转发该连接             |


#### outbound

==action 为 direct 则必填==

目标出站的标签。

### 逻辑字段

#### type

`logical`

#### mode

==必填==

`and` 或 `or`

#### rules

==必填==

包括的默认规则。