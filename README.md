# llm_manager

使用 Go 编写的大模型管理软件，支持本地模型与在线模型统一接入，并通过多个 Gateway Port 对外提供稳定接口。

## 功能

- ✅ Go 实现。
- ✅ 支持本地 LLM（如 `llama.cpp` server）。
- ✅ 支持在线模型（OpenAI-compatible API）。
- ✅ 支持在 Gateway 端口上动态切换模型提供者。
- ✅ 支持同时运行多个 Gateway 端口。

## 架构

- **Admin API**（默认 `:8080`）：
  - 创建 Gateway
  - 切换 Gateway 的 provider
  - 列出当前 Gateway 与可用 provider
- **Gateway API**（可多个端口并行）：
  - 统一推理入口 `POST /v1/generate`
  - 健康检查 `GET /health`

## 环境变量

- `ADMIN_PORT`：Admin API 端口（默认 `8080`）
- `LLAMA_CPP_URL`：本地 `llama.cpp` 服务地址，例如 `http://127.0.0.1:8081`
- `ONLINE_BASE_URL`：在线模型 Base URL（OpenAI 兼容）
- `ONLINE_API_KEY`：在线模型 API Key
- `ONLINE_MODEL`：在线模型名称（默认 `gpt-4o-mini`）
- `DEFAULT_GATEWAYS`：启动时默认网关，格式：`9001:local,9002:online`

## 启动

```bash
go run ./cmd/llm-manager
```

## 管理示例

### 1) 创建一个 gateway port

```bash
curl -X POST http://127.0.0.1:8080/gateways \
  -H 'Content-Type: application/json' \
  -d '{"port":9001,"provider":"local"}'
```

### 2) 切换 provider

```bash
curl -X PUT http://127.0.0.1:8080/gateways/9001/provider \
  -H 'Content-Type: application/json' \
  -d '{"provider":"online"}'
```

### 3) 调用 gateway 推理

```bash
curl -X POST http://127.0.0.1:9001/v1/generate \
  -H 'Content-Type: application/json' \
  -d '{"prompt":"你好，介绍一下你自己"}'
```

### 4) 并行多个 gateway

```bash
curl -X POST http://127.0.0.1:8080/gateways -H 'Content-Type: application/json' -d '{"port":9001,"provider":"local"}'
curl -X POST http://127.0.0.1:8080/gateways -H 'Content-Type: application/json' -d '{"port":9002,"provider":"online"}'
```
