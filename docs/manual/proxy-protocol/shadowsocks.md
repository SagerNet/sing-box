---
icon: material/send
---

# Shadowsocks

Shadowsocks is the most well-known Chinese-made proxy protocol.
It exists in multiple versions, but only AEAD 2022 ciphers 
over TCP with multiplexing is recommended.

| Ciphers        | Specification                                              | Cryptographically sound | Resists passive detection | Resists active probes |
|----------------|------------------------------------------------------------|-------------------------|---------------------------|-----------------------|
| Stream Ciphers | [shadowsocks.org](https://shadowsocks.org/doc/stream.html) | :material-alert:        | :material-alert:          | :material-alert:      |
| AEAD           | [shadowsocks.org](https://shadowsocks.org/doc/aead.html)   | :material-check:        | :material-alert:          | :material-alert:      |
| AEAD 2022      | [shadowsocks.org](https://shadowsocks.org/doc/sip022.html) | :material-check:        | :material-check:          | :material-help:       |

(We strongly recommend using multiplexing to send UDP traffic over TCP, because
doing otherwise is vulnerable to passive detection.)

## :material-text-box-check: Password Generator

| For `2022-blake3-aes-128-gcm` cipher | For other ciphers             | Action                                                          |
|--------------------------------------|-------------------------------|-----------------------------------------------------------------|
| <code id="password_16"><code>        | <code id="password_32"><code> | <button class="md-button" onclick="generate()">Refresh</button> |

<script>
    function generatePassword(element, length) {
        const array = new Uint8Array(length);
        window.crypto.getRandomValues(array);
        document.getElementById(element).textContent = btoa(String.fromCharCode.apply(null, array));
    }
    function generate() {
      generatePassword("password_16", 16);
      generatePassword("password_32", 32);
    }
    generate();
</script>

## :material-server: Server Example

=== ":material-account: Single-user"

    ```json
     {
      "inbounds": [
        {
          "type": "shadowsocks",
          "listen": "::",
          "listen_port": 8080,
          "network": "tcp",
          "method": "2022-blake3-aes-128-gcm",
          "password": "<password>",
          "multiplex": {
            "enabled": true
          }
        }
      ]
    }
    ```

=== ":material-account-multiple: Multi-user"

    ```json
     {
      "inbounds": [
        {
          "type": "shadowsocks",
          "listen": "::",
          "listen_port": 8080,
          "network": "tcp",
          "method": "2022-blake3-aes-128-gcm",
          "password": "<server_password>",
          "users": [
            {
              "name": "sekai",
              "password": "<user_password>"
            }
          ],
          "multiplex": {
            "enabled": true
          }
        }
      ]
    }
    ```

## :material-cellphone-link: Client Example

=== ":material-account: Single-user"

    ```json
    {
      "outbounds": [
        {
          "type": "shadowsocks",
          "server": "127.0.0.1",
          "server_port": 8080,
          "method": "2022-blake3-aes-128-gcm",
          "password": "<pasword>",
          "multiplex": {
            "enabled": true
          }
        }
      ]
    }
    ```

=== ":material-account-multiple: Multi-user"

    ```json
    {
      "outbounds": [
        {
          "type": "shadowsocks",
          "server": "127.0.0.1",
          "server_port": 8080,
          "method": "2022-blake3-aes-128-gcm",
          "password": "<server_pasword>:<user_password>",
          "multiplex": {
            "enabled": true
          }
        }
      ]
    }
    ```
