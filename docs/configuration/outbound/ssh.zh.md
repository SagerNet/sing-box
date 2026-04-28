!!! quote "sing-box 1.14.0 中的更改"

    :material-plus: [cipher](#cipher)  
    :material-plus: [mac](#mac)  
    :material-plus: [kex_algorithm](#kex_algorithm)

### 结构

```json
{
  "type": "ssh",
  "tag": "ssh-out",

  "server": "127.0.0.1",
  "server_port": 22,
  "user": "root",
  "password": "admin",
  "private_key": "",
  "private_key_path": "$HOME/.ssh/id_rsa",
  "private_key_passphrase": "",
  "host_key": [
    "ecdsa-sha2-nistp256 AAAAE2VjZHNhLXNoYTItbmlzdH..."
  ],
  "host_key_algorithms": [],
  "client_version": "SSH-2.0-OpenSSH_7.4p1",
  "cipher": [],
  "mac": [],
  "kex_algorithm": [],

  ... // 拨号字段
}
```

### 字段

#### server

==必填==

服务器地址。

#### server_port

服务器端口，默认使用 22。

#### user

SSH 用户, 默认使用 root。

#### password

密码。

#### private_key

密钥。

#### private_key_path

密钥路径。

#### private_key_passphrase

密钥密码。

#### host_key

主机密钥，留空接受所有。

#### host_key_algorithms

主机密钥算法。

#### client_version

客户端版本，默认使用随机值。

#### cipher

!!! question "自 sing-box 1.14.0 起"

允许的加密算法。留空使用默认值。

#### mac

!!! question "自 sing-box 1.14.0 起"

允许的 MAC 算法。留空使用默认值。

#### kex_algorithm

!!! question "自 sing-box 1.14.0 起"

允许的密钥交换算法。留空使用默认值。

### 拨号字段

参阅 [拨号字段](/zh/configuration/shared/dial/)。
