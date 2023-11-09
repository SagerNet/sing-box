---
icon: material/package
---

# Package Manager

## :material-download-box: Manual Installation

=== ":material-debian: Debian / DEB"

    ```bash
    bash <(curl -fsSL https://sing-box.app/deb-install.sh)
    ```

=== ":material-redhat: Redhat / RPM"

    ```bash
    bash <(curl -fsSL https://sing-box.app/rpm-install.sh)
    ```

=== ":simple-archlinux: Archlinux / PKG"

    ```bash
    bash <(curl -fsSL https://sing-box.app/arch-install.sh)
    ```

## :material-book-lock-open: Managed Installation

=== ":material-linux: Linux"

    | Type     | Platform           | Link                | Command                      | Actively maintained |
    |----------|--------------------|---------------------|------------------------------|---------------------|
    | AUR      | (Linux) Arch Linux | [sing-box][aur] ᴬᵁᴿ | `? -S sing-box`              | :material-check:    |
    | nixpkgs  | (Linux) NixOS      | [sing-box][nixpkgs] | `nix-env -iA nixos.sing-box` | :material-check:    |
    | Homebrew | macOS / Linux      | [sing-box][brew]    | `brew install sing-box`      | :material-check:    |
    | Alpine   | (Linux) Alpine     | [sing-box][alpine]  | `apk add sing-box`           | :material-alert:    |

=== ":material-apple: macOS"

    | Type     | Platform      | Link             | Command                 | Actively maintained |
    |----------|---------------|------------------|-------------------------|---------------------|
    | Homebrew | macOS / Linux | [sing-box][brew] | `brew install sing-box` | :material-check:    |

=== ":material-microsoft-windows: Windows"

    | Type       | Platform           | Link                | Command                      | Actively maintained |
    |------------|--------------------|---------------------|------------------------------|---------------------|
    | Scoop      | Windows            | [sing-box][scoop]   | `scoop install sing-box`     | :material-check:    |
    | Chocolatey | Windows            | [sing-box][choco]   | `choco install sing-box`     | :material-check:    |
    | winget     | Windows            | [sing-box][winget]  | `winget install sing-box`    | :material-alert:    |

=== ":material-android: Android"

    | Type       | Platform           | Link                | Command                      | Actively maintained |
    |------------|--------------------|---------------------|------------------------------|---------------------|
    | Termux     | Android            | [sing-box][termux]  | `pkg add sing-box`           | :material-check:    |

## :material-book-multiple: Service Management

For Linux systems with [systemd][systemd], usually the installation already includes a sing-box service,
you can manage the service using the following command:

| Operation | Command                                       |
|-----------|-----------------------------------------------|
| Enable    | `sudo systemctl enable sing-box`              |
| Disable   | `sudo systemctl disable sing-box`             |
| Start     | `sudo systemctl start sing-box`               |
| Stop      | `sudo systemctl stop sing-box`                |
| Kill      | `sudo systemctl kill sing-box`                |
| Restart   | `sudo systemctl restart sing-box`             |
| Logs      | `sudo journalctl -u sing-box --output cat -e` |
| New Logs  | `sudo journalctl -u sing-box --output cat -f` |

[alpine]: https://pkgs.alpinelinux.org/packages?name=sing-box

[aur]: https://aur.archlinux.org/packages/sing-box

[nixpkgs]: https://github.com/NixOS/nixpkgs/blob/nixos-unstable/pkgs/tools/networking/sing-box/default.nix

[termux]: https://github.com/termux/termux-packages/tree/master/packages/sing-box

[brew]: https://formulae.brew.sh/formula/sing-box

[choco]: https://chocolatey.org/packages/sing-box

[scoop]: https://github.com/ScoopInstaller/Main/blob/master/bucket/sing-box.json

[winget]: https://github.com/microsoft/winget-pkgs/tree/master/manifests/s/SagerNet/sing-box

[systemd]: https://systemd.io/