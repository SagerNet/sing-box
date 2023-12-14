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

### Dial Fields

See [Dial Fields](/configuration/shared/dial/) for details.
