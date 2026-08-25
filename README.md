# GoSentinel

面向 Go 微服务的嵌入式限流/熔断工具包与阻断雷达大屏。

## 1. 如何启动

```bash
docker compose up --build -d
```

浏览器打开 `http://localhost:31481`。

## 2. 使用说明

大屏展示 Pass/Block/熔断比例与雷达扫描。右侧可编辑资源 QPS，保存后经 WebSocket 下发到示例 Gin/gRPC 节点。对 `http://localhost:31483/work` 加压可观察阻断。

## 3. 服务列表及API说明

| 入口 | 说明 |
| --- | --- |
| http://localhost:31481 | 阻断雷达（同源反代 `/api` `/ws` `/healthz`） |
| http://localhost:31483/work | Gin 示例 |
| localhost:31484 | gRPC `demo.Demo/Work` |

详见 `docs/API.md`。

## 4. 测试账号

MVP 为可信内网演示，无登录。

## 5. 题目内容

动态熔断、自适应限流、滑动窗口统计与阻断雷达大屏。

## 6. 项目结构

`cmd/` 进程入口，`pkg/sentinel` 公共 SDK，`internal/` 引擎与控制面，`frontend/` Vue 大屏。

## 7. API 模拟与切换指南

本项目无外部计费 API。引擎、规则存储、WebSocket 均为真实实现。示例服务的 `fail=1` 与负载请求是本地流量发生器，用于制造 Pass/Block/Error，不是替换保护逻辑的 mock。不存在 real/mock Provider 开关。
