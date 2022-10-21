# 引言

sing-box 使用 JSON 作为配置文件格式。

### 结构

```json
{
  "log": {},
  "dns": {},
  "inbounds": [],
  "outbounds": [],
  "route": {},
  "experimental": {}
}
```

### 字段

| Key            | Format                |
|----------------|-----------------------|
| `log`          | [日志](./log)           |
| `dns`          | [DNS](./dns)          |
| `inbounds`     | [入站](./inbound)       |
| `outbounds`    | [出站](./outbound)      |
| `route`        | [路由](./route)         |
| `experimental` | [实验性](./experimental) |

### 检查

```bash
$ sing-box check
```

### 格式化

```bash
$ sing-box format -w
```

### 多个配置文件

> 如果只使用单个配置文件，您完全可以忽略这一节。

sing-box 支持多文件配置。按照配置文件的加载顺序，后者会覆盖并合并到前者。

```bash
# 根据参数顺序加载
sing-box run -c inbound.json -c outbound.json
# 根据文件名顺序加载
sing-box run -r -c config_dir
```

假设我们有两个 `JSON` 文件：

`a.json`:

```json
{
  "log": {"level": "debug"},
  "inbounds": [{"tag": "in-1"}],
  "outbounds": [{"_priority": 100, "tag": "out-1"}],
  "route": {"rules": [
    {"_tag":"rule1","inbound":["in-1"],"outbound":"out-1"}
  ]}
}
```

`b.json`:

```json
{
  "log": {"level": "error"},
  "outbounds": [{"_priority": -100, "tag": "out-2"}],
  "route": {"rules": [
    {"_tag":"rule1","inbound":["in-1.1"],"outbound":"out-1.1"}
  ]}
}
```

合并后:

```jsonc
{
  // level 字段被后来者覆盖
  "log": {"level": "error"},
  "inbounds": [{"tag": "in-1"}],
  "outbounds": [
    // out-2 虽然是后来者，但由于 _priority 小，反而排在前面
    {"tag": "out-2"}, 
    {"tag": "out-1"}
  ],
  "route": {"rules": [
    // 2条规则被合并，因为它们具有相同的 "_tag"，
    // outbound 字段在合并过程被覆盖为 "out-1.1"
    {"inbound":["in-1","in-1.1"],"outbound":"out-1.1"}
  ]}
}
```

只需记住这几个规则：

- 简单字段（字符串、数字、布尔值）会被后来者覆盖，其它字段（数组、对象）会被合并。
- 数组内拥有相同 `_tag` 的对象会被合并。
- 数组会按 `_priority` 字段值进行排序，越小的优先级越高。
