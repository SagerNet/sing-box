!!! quote "Changes in sing-box 1.14.0"

    :material-plus: [cipher](#cipher)  
    :material-plus: [mac](#mac)  
    :material-plus: [kex_algorithm](#kex_algorithm)

### Structure

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

  ... // Dial Fields
}
```

### Fields

#### server

==Required==

Server address.

#### server_port

Server port. 22 will be used if empty.

#### user

SSH user, root will be used if empty.

#### password

Password.

#### private_key

Private key.

#### private_key_path

Private key path.

#### private_key_passphrase

Private key passphrase.

#### host_key

Host key. Accept any if empty.

#### host_key_algorithms

Host key algorithms.

#### client_version

Client version. Random version will be used if empty.

#### cipher

!!! question "Since sing-box 1.14.0"

Allowed ciphers. Default values are used if empty.

#### mac

!!! question "Since sing-box 1.14.0"

Allowed MAC algorithms. Default values are used if empty.

#### kex_algorithm

!!! question "Since sing-box 1.14.0"

Allowed key exchange algorithms. Default values are used if empty.

### Dial Fields

See [Dial Fields](/configuration/shared/dial/) for details.
