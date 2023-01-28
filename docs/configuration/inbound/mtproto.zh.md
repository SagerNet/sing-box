### 结构

```json
{
  "type": "mtproto",
  "tag": "mtproto-in",

  ... // 监听字段

  "users": [
    {
      "name": "sekai",
      "secret": "ee134132e79f44020784bddce2e734b5e2676f6f676c652e636f6d"
    }
  ]
}
```

!!! warning ""

    默认安装不包含 MTProto，参阅 [安装](/zh/#_2)。

### 监听字段

参阅 [监听字段](/zh/configuration/shared/listen/)。

### 字段

#### users

==必填==

MTProto 用户，其中 secret 是 MTProto V3 密钥。

!!! note ""

    受限于其身份认证算法，MTProto 多用户入站可能性能不佳。
