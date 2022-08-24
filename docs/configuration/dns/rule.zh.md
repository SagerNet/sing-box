### 结构

```json
{
  "dns": {
    "rules": [
      {
        "inbound": [
          "mixed-in"
        ],
        "ip_version": 6,
        "network": "tcp",
        "auth_user": [
          "usera",
          "userb"
        ],
        "protocol": [
          "tls",
          "http",
          "quic"
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
        "source_ip_cidr": [
          "10.0.0.0/24"
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
        "process_name": [
          "curl"
        ],
        "package_name": [
          "com.termux"
        ],
        "user": [
          "sekai"
        ],
        "user_id": [
          1000
        ],
        "invert": false,
        "outbound": [
          "direct"
        ],
        "server": "local",
        "disable_cache": false
      },
      {
        "type": "logical",
        "mode": "and",
        "rules": [],
        "server": "local",
        "disable_cache": false
      }
    ]
  }
}

```

!!! note ""

    当内容只有一项时，可以忽略 JSON Array [] 标签。

### 默认字段

!!! note ""

    默认规则使用以下逻辑关系词匹配：
    (`domain` || `domain_suffix` || `domain_keyword` || `domain_regex` || `geosite`) &&  
    (`source_geoip` || `source_ip_cidr`) &&  
    `other fields`  

#### inbound

[入站](../inbound)标签。

#### ip_version

4 (A dns记录查询) or 6 (AAAA dns记录查询)。

如果为空则不受限制。

#### network

`tcp` 或 `udp`。

#### user

用户名，请参阅每个入站了解详情。

#### protocol

协议探测, 详见 [Sniff](/configuration/route/sniff/)。

#### domain

匹配完整域名。

#### domain_suffix

匹配域后缀。

#### domain_keyword

使用关键字匹配域。

#### domain_regex

使用正则表达式匹配域。

#### geosite

依据 geosite 匹配。

#### source_geoip

匹配源 geoip。

#### source_ip_cidr

匹配源 ip cidr。

#### source_port

匹配源端口。

#### source_port_range

匹配源端口范围。

#### port

匹配端口。

#### port_range

匹配端口范围。

#### process_name

!!! error ""

    仅支持 Linux, Windows, and macOS。

匹配进程名称。

#### package_name

匹配 android 包名。

#### user

!!! error ""

    仅支持 Linux。

匹配用户名。

#### user_id

!!! error ""

    仅支持 Linux。

匹配用户 ID。

#### invert

反转匹配结果。

#### outbound

匹配出站。

#### server

==必填==

目标 DNS 服务器的标签。

#### disable_cache

在此查询中禁用缓存。

### 逻辑字段

#### type

`logical`

#### mode

`and` 或 `or`

#### rules

包括默认规则。

#### invert

反转匹配结果。

#### server

==必填==

目标 DNS 服务器的标签。

#### disable_cache

在此查询中禁用缓存。