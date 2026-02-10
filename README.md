# Talk Trace Bot

Telegram 群聊消息总结 Bot - 自动保存群聊消息并生成每日 AI 总结

## 功能特性

- 📝 **消息存储**：自动保存所有群聊消息到 SQLite 数据库
- 🤖 **AI 总结**：使用 LLM 每日自动总结每位群成员的聊天记录
- 🧹 **自动清理**：定时清理过期消息，保持数据库精简
- 📢 **智能通知**：支持私信、群发或两者，自动处理消息长度限制
- 🔌 **多 LLM 支持**：支持 OpenAI、Azure、DeepSeek、Qwen 等多种 LLM 模型
- ⚡ **Token 管理**：自动处理 token 超限，智能拆分长文本

## 系统要求

- Linux 系统（推荐使用 WSL2）
- Go 1.24+ 
- TDLib 库（Telegram 官方库）
- SQLite3

## 编译步骤

### WSL2 快速编译（推荐）

如果你使用 WSL2，可以使用自动化脚本：

```bash
# 1. 安装所有依赖（包括 Go 和 TDLib）
chmod +x install_deps.sh
./install_deps.sh

# 2. 编译项目
chmod +x build.sh
./build.sh
```

详细说明请参考 [BUILD_WSL2.md](BUILD_WSL2.md)

### 手动编译步骤

#### 1. 安装 TDLib

在 WSL2/Linux 中安装 TDLib：

```bash
# Ubuntu/Debian
sudo apt-get update
sudo apt-get install -y build-essential cmake gperf libssl-dev zlib1g-dev libreadline-dev

# 下载并编译 TDLib
git clone https://github.com/tdlib/td.git
cd td
mkdir build
cd build
cmake -DCMAKE_BUILD_TYPE=Release ..
cmake --build . -j$(nproc)
sudo cmake --install .
```

#### 2. 安装 Go 依赖

```bash
go mod download
```

#### 3. 编译项目

```bash
# 使用提供的编译脚本
chmod +x build.sh
./build.sh

# 或手动编译
go build -o talk-trace-bot .
```

## 配置

1. 复制配置文件模板：

```bash
cp etc/config.yaml.sample etc/config.yaml
```

2. 编辑 `etc/config.yaml`，配置以下内容：

- **TelegramApp**: 配置 Telegram API ID 和 Hash（从 https://my.telegram.org 获取）
- **SourceChatId**: 要监控的群聊 ID
- **LLM**: 配置 LLM API 端点和密钥
- **Summary**: 配置总结时间、保留天数和通知方式

## 运行

```bash
./talk-trace-bot -f etc/config.yaml
```

## 配置说明

### TelegramApp

- `ApiId`: Telegram API ID
- `ApiHash`: Telegram API Hash
- `SourceChatId`: 要监控的群聊 ID（负数表示群组）

### LLM

- `BaseURL`: LLM API 端点（支持 OpenAI 兼容的 API）
  - OpenAI: `https://api.openai.com/v1`
  - DeepSeek: `https://api.deepseek.com/v1`
  - Qwen: `https://dashscope.aliyuncs.com/compatible-mode/v1`
- `APIKey`: API 密钥
- `Model`: 模型名称（如 `gpt-4o`, `deepseek-chat`, `qwen-plus`）
- `MaxTokens`: 模型上下文窗口大小

### Summary

- `Cron`: Cron 表达式，定义总结执行时间（如 `"0 23 * * *"` 表示每天 23:00）
- `RetentionDays`: 消息保留天数
- `NotifyMode`: 通知模式
  - `private`: 仅私信通知
  - `group`: 仅群内通知
  - `both`: 两者都通知
- `NotifyUserIds`: 私信通知的目标用户 ID 列表

## 工作流程

1. Bot 启动后自动监听指定群聊的消息
2. 所有消息自动保存到 SQLite 数据库
3. 按配置的 cron 时间执行每日总结：
   - 生成每位成员的聊天摘要
   - 保存摘要到数据库
   - 发送通知（私信/群发）
   - 清理过期消息（保留 RetentionDays + 1 天）

## 注意事项

- 首次运行需要登录 Telegram，按照提示输入验证码
- 确保 LLM API 密钥有效且有足够额度
- 消息清理会在摘要生成后执行，确保不会误删当日数据
- Telegram 消息长度限制为 4096 字符，超出会自动拆分

## 测试

项目包含完整的单元测试和集成测试。

### 运行单元测试

```bash
# 运行所有测试
go test ./...

# 运行 LLM 模块测试
go test ./internal/llm -v

# 查看测试覆盖率
go test ./internal/llm -cover
```

### 运行集成测试

集成测试需要真实的 LLM API key（可选）：

```bash
export LLM_API_KEY="your-api-key"
export LLM_BASE_URL="https://api.openai.com/v1"  # 可选
export LLM_MODEL="gpt-3.5-turbo"  # 可选

go test -tags=integration ./internal/llm -v
```

详细测试说明请参考 [internal/llm/README_TEST.md](internal/llm/README_TEST.md)

## License

See LICENSE file for details.
