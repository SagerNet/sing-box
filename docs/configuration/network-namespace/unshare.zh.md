---
icon: material/new-box
---

!!! question "自 sing-box 1.14.0 起"

# Unshare

创建一个新的网络命名空间，无需 root 权限。

!!! info ""

    无 root 运行需要内核允许非特权用户创建 user namespace。

### 结构

```json
{
  "network_namespaces": [
    {
      "type": "unshare",
      "tag": "",
      "pid_file": ""
    }
  ]
}
```

### 字段

#### pid_file

如果设置，持有该命名空间的进程 PID 将写入此路径。

当 sing-box 以 root 运行时，可通过 `nsenter -t <pid> -n` 进入该命名空间，
否则使用 `nsenter -t <pid> -U --preserve-credentials -n`。
