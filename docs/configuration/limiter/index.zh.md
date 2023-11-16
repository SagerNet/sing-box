# 限速

### 结构

```json
{
  "limiters": [
    {
      "tag": "limiter-a",
      "download": "10M",
      "upload": "1M",
      "auth_user": [
        "user-a",
        "user-b"
      ],
      "auth_user_independent": false,
      "inbound": [
        "in-a",
        "in-b"
      ],
      "inbound_independent": false
    }
  ]
}

```

### 字段

#### download upload

==必填==

格式: `[Integer][Unit]` 例如: `100M, 100m, 1G, 1g`.

支持的单位 (大小写不敏感): `B, K, M, G, T, P, E`.

#### tag

限速标签，在路由规则中使用。

#### auth_user

用户组限速，参阅入站设置。

#### auth_user_independent

使每个用户有单独的限速。关闭时将共享限速。

#### inbound

入站组限速。

#### inbound_independent

使每个入站有单独的限速。关闭时将共享限速。
