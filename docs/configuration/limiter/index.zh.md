# 限速

### 结构

```json
{
  "limiters": [
    {
      "tag": "limiter-a",
      "download": "1M",
      "upload": "10M",
      "auth_user": [
        "user-a",
        "user-b"
      ],
      "inbound": [
        "in-a",
        "in-b"
      ]
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

用户组全局限速，参阅入站设置。

#### inbound

入站组全局限速。

!!! info ""

    所有用户、入站和有限速标签的路由规则共享同一个限速。为了独立生效，请分别配置限速器。