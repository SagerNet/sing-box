---
icon: material/alpha-t-box
---

# TUIC

A recently popular Chinese-made simple protocol based on QUIC, the selling point is the BBR congestion control algorithm.

!!! warning

    Even though GFW rarely blocks UDP-based proxies, such protocols actually have far more characteristics than TCP based proxies.

| Specification                                             | Binary Characteristics | Active Detect Hiddenness |
|-----------------------------------------------------------|------------------------|--------------------------|
| [GitHub](https://github.com/EAimTY/tuic/blob/dev/SPEC.md) | :material-alert:       | :material-check:         | 

## Password Generator

| Generated UUID         | Generated  Password        | Action                                                          |
|------------------------|----------------------------|-----------------------------------------------------------------|
| <code id="uuid"><code> | <code id="password"><code> | <button class="md-button" onclick="generate()">Refresh</button> |

<script>
    function generateUUID() {
        const uuid = 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, function(c) {
            let r = Math.random() * 16 | 0,
            v = c === 'x' ? r : (r & 0x3 | 0x8);
            return v.toString(16);
        });
        document.getElementById("uuid").textContent = uuid;
    }
    function generatePassword() {
        const array = new Uint8Array(16);
        window.crypto.getRandomValues(array);
        document.getElementById("password").textContent = btoa(String.fromCharCode.apply(null, array));
    }
    function generate() {
        generateUUID();
        generatePassword();
    }
    generate();
</script>

## :material-server: Server Example

=== ":material-harddisk: With local certificate"

    ```json
     {
      "inbounds": [
        {
          "type": "tuic",
          "listen": "::",
          "listen_port": 8080,
          "users": [
            {
              "name": "sekai",
              "uuid": "<uuid>",
              "password": "<password>"
            }
          ],
          "congestion_control": "bbr",
          "tls": {
            "enabled": true,
            "server_name": "example.org",
            "key_path": "/path/to/key.pem",
            "certificate_path": "/path/to/certificate.pem"
          }
        }
      ]
    }
    ```

=== ":material-auto-fix: With ACME"

    ```json
    {
      "inbounds": [
        {
          "type": "tuic",
          "listen": "::",
          "listen_port": 8080,
          "users": [
            {
              "name": "sekai",
              "uuid": "<uuid>",
              "password": "<password>"
            }
          ],
          "congestion_control": "bbr",
          "tls": {
            "enabled": true,
            "server_name": "example.org",
            "acme": {
              "domain": "example.org",
              "email": "admin@example.org"
            }
          }
        }
      ]
    }
    ```

=== ":material-cloud: With ACME and Cloudflare API"

    ```json
    {
      "inbounds": [
        {
          "type": "tuic",
          "listen": "::",
          "listen_port": 8080,
          "users": [
            {
              "name": "sekai",
              "uuid": "<uuid>",
              "password": "<password>"
            }
          ],
          "congestion_control": "bbr",
          "tls": {
            "enabled": true,
            "server_name": "example.org",
            "acme": {
              "domain": "example.org",
              "email": "admin@example.org",
              "dns01_challenge": {
                "provider": "cloudflare",
                "api_token": "my_token"
              }
            }
          }
        }
      ]
    }
    ```

## :material-cellphone-link: Client Example

=== ":material-web-check: With valid certificate"

    ```json
    {
      "outbounds": [
        {
          "type": "tuic",
          "server": "127.0.0.1",
          "server_port": 8080,
          "uuid": "<uuid>",
          "password": "<password>",
          "congestion_control": "bbr",
          "tls": {
            "enabled": true,
            "server_name": "example.org"
          }
        }
      ]
    }
    ```

=== ":material-check: With self-sign certificate"

    !!! info "Tip"
        
        Use `sing-box merge` command to merge configuration and certificate into one file.

    ```json
    {
      "outbounds": [
        {
          "type": "tuic",
          "server": "127.0.0.1",
          "server_port": 8080,
          "uuid": "<uuid>",
          "password": "<password>",
          "congestion_control": "bbr",
          "tls": {
            "enabled": true,
            "server_name": "example.org",
            "certificate_path": "/path/to/certificate.pem"
          }
        }
      ]
    }
    ```

=== ":material-alert: Ignore certificate verification"

    ```json
    {
      "outbounds": [
        {
          "type": "tuic",
          "server": "127.0.0.1",
          "server_port": 8080,
          "uuid": "<uuid>",
          "password": "<password>",
          "congestion_control": "bbr",
          "tls": {
            "enabled": true,
            "server_name": "example.org",
            "insecure": true
          }
        }
      ]
    }
    ```

