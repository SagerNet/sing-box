# XHTTP XMUX 基准

`BenchmarkXHTTPXMux` 在同一台主机、同一份本地 Xray XHTTP 服务端配置下，比较
sing-box 和 Xray 的 VMess XHTTP client。每个实现均测量无预算限制与一组固定
XMUX 预算：`max_concurrency=4`、`c_max_reuse_times=16`、
`h_max_request_times=64`、`h_max_reusable_secs=60`。

每次操作新建一个 SOCKS TCP 连接，经 XHTTP `packet-up`/H2 往本机 echo 服务
写入并读回 64 KiB。这同时覆盖连接分配、HTTP 请求预算轮换和有效负载传输。

```bash
XHTTP_XRAY_BINARY=/path/to/xray \
  go test ./transport/v2rayxhttp -run '^$' -bench '^BenchmarkXHTTPXMux$' \
  -benchmem -benchtime=10s -count=5
```

输出含义：

- `MB/s`：有效回显负载吞吐；
- `p99_ms`：单次 SOCKS 建连、64 KiB 往返的 P99；
- `connections/s`：完成的逻辑代理连接建立速率；
- `B/op`、`allocs/op`：Go benchmark 的分配指标。

`connections/s` 是逻辑连接指标；XMUX 实际 HTTP client 的数量与淘汰边界由
`TestXMuxMaxConnections`、`TestXMuxMaxConcurrency`、预算测试覆盖。测试不把
特定机器的跑分提交到仓库，因为 CPU 型号、内核调度和 Go 版本会显著影响结果。

## CPU 与内存 profile

应针对单一子基准收集 profile，避免将不同实现混在一张火焰图中：

```bash
XHTTP_XRAY_BINARY=/path/to/xray \
  go test ./transport/v2rayxhttp -run '^$' \
  -bench '^BenchmarkXHTTPXMux/sing-box/xmux$' -benchmem -benchtime=15s \
  -cpuprofile=/tmp/sing-box-xmux.cpu.pprof \
  -memprofile=/tmp/sing-box-xmux.mem.pprof

go tool pprof -top /tmp/sing-box-xmux.cpu.pprof
go tool pprof -top -sample_index=alloc_space /tmp/sing-box-xmux.mem.pprof
```

将子基准替换为 `sing-box/no-xmux`、`xray/no-xmux` 或 `xray/xmux` 即可形成
四组同机对照。Xray 子基准的 Go profile 记录基准进程；若需要 Xray 子进程自身的
CPU 火焰图，应同时对其进程使用系统级 profiler（例如 `perf record -p <pid>`）。
