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
  "ciphers": [
    "aes128-gcm@openssh.com",
    "chacha20-poly1305@openssh.com"
  ],
  "macs": [
    "hmac-sha2-256-etm@openssh.com"
  ],
  "key_exchanges": [
    "curve25519-sha256"
  ],

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

#### ciphers

加密算法列表。留空使用 SSH 默认值。

可用值: `aes128-gcm@openssh.com` | `aes256-gcm@openssh.com` | `chacha20-poly1305@openssh.com` | `aes128-ctr` | `aes192-ctr` | `aes256-ctr`。

#### macs

MAC（消息认证码）算法列表。留空使用 SSH 默认值。

可用值: `hmac-sha2-256-etm@openssh.com` | `hmac-sha2-512-etm@openssh.com` | `hmac-sha2-256` | `hmac-sha2-512` | `hmac-sha1`。

#### key_exchanges

密钥交换算法列表。留空使用 SSH 默认值。

可用值: `mlkem768x25519-sha256` | `curve25519-sha256` | `ecdh-sha2-nistp256` | `ecdh-sha2-nistp384` | `ecdh-sha2-nistp521` | `diffie-hellman-group14-sha256` | `diffie-hellman-group16-sha512` | `diffie-hellman-group-exchange-sha256`。

### 拨号字段

参阅 [拨号字段](/zh/configuration/shared/dial/)。
