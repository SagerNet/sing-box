#### 依赖

* Linux & Systemd
* Git
* C 编译器环境

#### 安装

```shell
git clone https://github.com/SagerNet/sing-box
cd sing-box
./release/local/install_go.sh # 如果已安装 go1.19 则跳过
./release/local/install.sh
```

编辑配置文件 `/usr/local/etc/sing-box/config.json`

```shell
./release/local/enable.sh
```

#### 更新

```shell
./release/local/update.sh
```

#### 其他命令

| 操作   | 命令                                            |
|------|-----------------------------------------------|
| 启动   | `sudo systemctl start sing-box`               |
| 停止   | `sudo systemctl stop sing-box`                |
| 强制停止 | `sudo systemctl kill sing-box`                |
| 重启   | `sudo systemctl restart sing-box`             |
| 查看日志 | `sudo journalctl -u sing-box --output cat -e` |
| 实时日志 | `sudo journalctl -u sing-box --output cat -f` |
| 卸载   | `./release/local/uninstall.sh`                |