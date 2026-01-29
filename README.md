# 🤖 AI Multi-Agent Debate

<div align="center">

![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?style=for-the-badge&logo=go&logoColor=white)
![License](https://img.shields.io/badge/License-MIT-green?style=for-the-badge)
![Platform](https://img.shields.io/badge/Platform-Linux%20%7C%20macOS%20%7C%20Windows-lightgrey?style=for-the-badge)

**Ứng dụng Multi-Agent AI cho phép nhiều AI từ các providers khác nhau thảo luận, phản biện và góp ý với nhau về một chủ đề.**

[Quick Start](#-quick-start) •
[Tính năng](#-tính-năng) •
[Cấu hình](#-cấu-hình) •
[Sử dụng](#-sử-dụng) •
[API](#-api-endpoints)

</div>

---

## 🚀 Quick Start

### ⬇️ Tải file chạy ngay (Không cần cài đặt!)

Đã build sẵn cho các hệ điều hành. Tải về, giải nén và chạy ngay:

| Hệ điều hành | Kiến trúc | Download |
|--------------|-----------|----------|
| **🍎 macOS** | Apple Silicon (M1/M2/M3) | [📥 talk-darwin-arm64](https://github.com/yourusername/talk/releases/latest/download/talk-darwin-arm64) |
| **🍎 macOS** | Intel | [📥 talk-darwin-amd64](https://github.com/yourusername/talk/releases/latest/download/talk-darwin-amd64) |
| **🐧 Linux** | x64 | [📥 talk-linux-amd64](https://github.com/yourusername/talk/releases/latest/download/talk-linux-amd64) |
| **🐧 Linux** | ARM64 | [📥 talk-linux-arm64](https://github.com/yourusername/talk/releases/latest/download/talk-linux-arm64) |
| **🪟 Windows** | x64 | [📥 talk-windows-amd64.exe](https://github.com/yourusername/talk/releases/latest/download/talk-windows-amd64.exe) |

### Chạy nhanh trong 3 bước:

```bash
# 1. Tải file phù hợp với hệ điều hành của bạn (ví dụ macOS Apple Silicon)
curl -LO https://github.com/yourusername/talk/releases/latest/download/talk-darwin-arm64

# 2. Cấp quyền chạy (macOS/Linux)
chmod +x talk-darwin-arm64

# 3. Set API key và chạy
export OPENAI_API_KEY="sk-..."   # hoặc các provider khác
./talk-darwin-arm64
```

> 💡 **Tip**: Mở browser tại **http://localhost:8080** sau khi chạy

---

## ✨ Tính năng

### 🌐 Multi-Provider Support
Hỗ trợ 7 AI providers khác nhau:

| Provider | Models | Tính năng đặc biệt |
|----------|--------|-------------------|
| **OpenAI** | GPT-4, GPT-4 Turbo, GPT-3.5 | Ổn định, đa năng |
| **Anthropic** | Claude 3.5 Sonnet, Claude 3 Opus | Thinking models, dài hơi |
| **Google Gemini** | Gemini Pro, Gemini 1.5 Pro | Xử lý multimodal |
| **Ollama** | Llama 2, Mistral, Codellama | Local, miễn phí |
| **Grok** | Grok Beta | Realtime info từ X |
| **DeepSeek** | DeepSeek Chat, DeepSeek Coder | Code-focused |
| **Perplexity** | Sonar models | Web search tích hợp |

### 🎭 Multiple AI Agents
4 agents mặc định với vai trò phân biệt:

- 🔍 **Analyst** - Phân tích logic, data-driven
- 💡 **Creative** - Sáng tạo, đưa ý tưởng mới
- ⚔️ **Critic** - Phản biện, tìm lỗ hổng
- 🔗 **Synthesizer** - Tổng hợp, tìm điểm chung

### 🔄 Debate Modes
- **Round-Robin**: Các agent lần lượt phát biểu theo vòng
- **Free-Form**: Agent tự do phản hồi dựa trên context

### 🌟 Thêm nhiều tính năng khác
- ⚡ **Real-time Streaming** - Xem phản hồi AI theo thời gian thực qua WebSocket
- 🎨 **Modern Web UI** - Giao diện đẹp, responsive, dark mode
- 📤 **Export/Import** - Xuất cuộc thảo luận ra Markdown
- 🔧 **Fully Configurable** - Tùy chỉnh mọi thứ qua YAML config
- 🚀 **Auto Mode** - Để các agents tự động thảo luận liên tục

---

## 📦 Cài đặt

### Yêu cầu
- Go 1.21 hoặc mới hơn
- Ít nhất 1 API key từ các provider (hoặc Ollama cho local)

### Build từ source

```bash
# Clone repository
git clone https://github.com/yourusername/talk.git
cd talk

# Download dependencies
go mod tidy

# Build
go build -o talk .
```

### Hoặc sử dụng Go Install

```bash
go install github.com/yourusername/talk@latest
```

---

## ⚙️ Cấu hình

### Cách 1: Environment Variables (Nhanh)

```bash
# Set ít nhất 1 trong các keys
export OPENAI_API_KEY="sk-..."
export ANTHROPIC_API_KEY="sk-ant-..."
export GEMINI_API_KEY="..."
export GROK_API_KEY="xai-..."
export DEEPSEEK_API_KEY="..."
export PERPLEXITY_API_KEY="pplx-..."

# Chạy
./talk
```

### Cách 2: Config File (Chi tiết hơn)

```bash
# Copy template
cp config.example.yaml config.yaml

# Chỉnh sửa theo nhu cầu
vim config.yaml
```

**Ví dụ config.yaml:**

```yaml
server:
  port: "8080"

agents:
  - id: analyst
    name: Analyst
    role: "Phân tích logic, data-driven"
    system_prompt: |
      Bạn là Analyst - một chuyên gia phân tích logic và dữ liệu.
      Phân tích vấn đề một cách logic, có hệ thống.
    provider: openai
    model: gpt-4
    color: "#4A90D9"
    api_key: "sk-..."  # hoặc dùng env variable

  - id: creative
    name: Creative
    role: "Sáng tạo, đưa ý tưởng mới"
    provider: anthropic
    model: claude-3-5-sonnet-20241022
    color: "#9B59B6"

  - id: critic
    name: Critic
    role: "Phản biện, tìm lỗ hổng"
    provider: gemini
    model: gemini-pro
    color: "#E74C3C"

  - id: synthesizer
    name: Synthesizer
    role: "Tổng hợp, tìm điểm chung"
    provider: ollama
    model: llama2
    base_url: "http://localhost:11434"
    color: "#27AE60"
```

### Sử dụng Ollama (Local LLM - Miễn phí)

```bash
# 1. Cài đặt Ollama
curl -fsSL https://ollama.com/install.sh | sh

# 2. Pull model
ollama pull llama2
# hoặc ollama pull mistral

# 3. Chạy server
ollama serve

# 4. Cấu hình trong config.yaml
# provider: ollama
# model: llama2
# base_url: "http://localhost:11434"
```

---

## 🚀 Sử dụng

### Khởi động server

```bash
# Default port 8080
./talk

# Custom port
./talk -port 3000

# Custom config file
./talk -config myconfig.yaml
```

Mở browser: **http://localhost:8080**

### Giao diện Web

<div align="center">
<table>
<tr>
<td align="center"><b>1. Nhập chủ đề</b><br>Gõ chủ đề bạn muốn thảo luận</td>
<td align="center"><b>2. Bắt đầu</b><br>Click "Bắt đầu" hoặc "Start + Auto"</td>
<td align="center"><b>3. Theo dõi</b><br>Xem các AI tranh luận realtime</td>
</tr>
</table>
</div>

### Các nút điều khiển

| Nút | Chức năng |
|-----|-----------|
| **Bắt đầu** | Bắt đầu cuộc thảo luận với chủ đề đã nhập |
| **Start + Auto** | Bắt đầu và tự động để các agents thảo luận |
| **Agent tiếp theo** | Trigger agent kế tiếp phát biểu |
| **Tự động** | Bật/tắt mode tự động |
| **Dừng** | Dừng cuộc thảo luận |
| **Reset** | Xóa toàn bộ và bắt đầu lại |
| **Export MD** | Xuất cuộc thảo luận ra file Markdown |

---

## 📡 API Endpoints

### REST API

| Method | Endpoint | Mô tả | Body |
|--------|----------|-------|------|
| `GET` | `/api/agents` | Danh sách agents | - |
| `GET` | `/api/debate/status` | Trạng thái debate | - |
| `POST` | `/api/debate/start` | Bắt đầu debate | `{"topic": "..."}` |
| `POST` | `/api/debate/continue` | Tiếp tục với topic mới | `{"topic": "..."}` |
| `POST` | `/api/debate/stop` | Dừng debate | - |
| `POST` | `/api/debate/next` | Trigger agent tiếp theo | - |
| `POST` | `/api/debate/agent/{id}` | Trigger agent cụ thể | - |
| `GET` | `/api/debate/messages` | Lịch sử tin nhắn | - |
| `POST` | `/api/debate/mode` | Đổi mode | `{"mode": "round_robin"}` |
| `POST` | `/api/debate/reset` | Reset debate | - |

### WebSocket

Kết nối: `ws://localhost:8080/ws`

**Server → Client Events:**

```javascript
// Agent bắt đầu nói
{"type": "start", "agent_id": "analyst", "agent_name": "Analyst", "message_id": "msg_1", "color": "#4A90D9"}

// Streaming content
{"type": "chunk", "agent_id": "analyst", "content": "Theo phân tích...", "message_id": "msg_1"}

// Agent nói xong
{"type": "end", "agent_id": "analyst", "message_id": "msg_1"}

// Events khác
{"type": "debate_started", "topic": "..."}
{"type": "debate_stopped"}
{"type": "debate_reset"}
{"type": "mode_changed", "mode": "free_form"}
{"type": "error", "error": "..."}
```

---

## 📁 Cấu trúc dự án

```
talk/
├── main.go                      # Entry point, config parsing
├── go.mod                       # Go module definition
├── config.example.yaml          # Config template
│
├── internal/
│   ├── agent/
│   │   └── agent.go             # Agent logic & chat handling
│   │
│   ├── debate/
│   │   └── manager.go           # Debate orchestration, context building
│   │
│   ├── provider/
│   │   ├── provider.go          # Provider interface & factory
│   │   ├── openai.go            # OpenAI implementation
│   │   ├── anthropic.go         # Anthropic (Claude) implementation
│   │   ├── gemini.go            # Google Gemini implementation
│   │   ├── ollama.go            # Ollama (local) implementation
│   │   ├── grok.go              # Grok (xAI) implementation
│   │   ├── deepseek.go          # DeepSeek implementation
│   │   └── perplexity.go        # Perplexity implementation
│   │
│   ├── server/
│   │   ├── server.go            # HTTP server & routes
│   │   └── websocket.go         # WebSocket handlers
│   │
│   ├── proxy/
│   │   └── proxy.go             # API gateway/proxy functionality
│   │
│   └── storage/
│       └── storage.go           # Config & state storage
│
└── web/static/
    ├── index.html               # Web UI
    ├── style.css                # Styles (dark mode, responsive)
    └── app.js                   # Frontend JavaScript
```

---

## 🛠️ Tùy chỉnh Agents

### Thêm agent mới

```yaml
agents:
  # Thêm vào cuối danh sách
  - id: philosopher
    name: Philosopher
    role: "Triết gia, suy ngẫm sâu sắc"
    system_prompt: |
      Bạn là một triết gia. Đưa ra các góc nhìn triết học sâu sắc.
      Đặt câu hỏi về bản chất và ý nghĩa của vấn đề.
    provider: anthropic
    model: claude-3-5-sonnet-20241022
    color: "#8E44AD"
```

### Cấu hình provider với proxy

```yaml
- id: custom_agent
  name: Custom Agent
  provider: openai
  model: gpt-4
  base_url: "http://your-proxy.com/v1"
  api_key: "your-key"
```

### Sử dụng thinking models (Claude)

```yaml
- id: deep_thinker
  name: Deep Thinker
  provider: anthropic
  model: claude-opus-4-5-thinking  # Thinking model
  # Lưu ý: Cần max_tokens cao hơn cho thinking models
```

---

## 🔧 Troubleshooting

### ❌ Không có agents nào được load
- Kiểm tra API keys đã được set đúng chưa (env hoặc config.yaml)
- Xem log output khi khởi động server

### ❌ WebSocket không kết nối
- Đảm bảo không có firewall chặn WebSocket
- Kiểm tra browser console (F12) để xem lỗi chi tiết
- Thử refresh trang

### ❌ Ollama không hoạt động
```bash
# Kiểm tra Ollama đang chạy
curl http://localhost:11434/api/tags

# Xem danh sách models
ollama list

# Pull model nếu chưa có
ollama pull llama2
```

### ❌ Lỗi "Invalid signature in thinking block"
- Đây là lỗi với Claude thinking models
- Đã được fix trong version mới nhất
- Nếu vẫn gặp, hãy update code từ repository

### ❌ Lỗi 429 (Rate limit)
- Đợi một lúc rồi thử lại
- Kiểm tra API quota của bạn
- Có thể dùng proxy để rotate keys

---

## 🤝 Đóng góp

Contributions are welcome! Hãy:

1. Fork repository
2. Tạo feature branch (`git checkout -b feature/amazing-feature`)
3. Commit changes (`git commit -m 'Add amazing feature'`)
4. Push to branch (`git push origin feature/amazing-feature`)
5. Tạo Pull Request

---

## 📄 License

Dự án được phân phối dưới giấy phép **MIT**. Xem file [LICENSE](LICENSE) để biết thêm chi tiết.

---

<div align="center">

**Made with ❤️ and lots of ☕**

⭐ Star repo này nếu bạn thấy hữu ích!

</div>
