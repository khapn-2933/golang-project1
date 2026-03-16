# Foods & Drinks API

Hệ thống quản lý đồ ăn và đồ uống với Golang.

## Công nghệ sử dụng

- **Language**: Go 1.21+
- **Database**: MySQL 8.0+
- **ORM**: GORM
- **Migration**: golang-migrate
- **Config**: Viper

## Cấu trúc Project

```
.
├── cmd/
│   ├── server/          # Main server application
│   ├── migrate/         # Database migration tool
│   └── seed/            # Database seeder
├── internal/
│   ├── config/          # Configuration management
│   ├── models/          # Database models
│   ├── repository/      # Data access layer
│   ├── service/         # Business logic layer
│   ├── handler/         # HTTP handlers
│   ├── middleware/      # HTTP middlewares
│   └── routes/          # Route definitions
├── pkg/
│   ├── database/        # Database connection
│   └── utils/           # Utility functions
├── migrations/          # SQL migration files
├── config.yaml          # Configuration file
├── config.example.yaml  # Example configuration
├── Makefile            # Build and run commands
└── go.mod              # Go module file
```

## Setup

### 1. Clone và cài đặt dependencies

```bash
git clone <repository-url>
cd kha
go mod download
```

### 2. Cấu hình

Copy file cấu hình mẫu và chỉnh sửa:

```bash
cp config.example.yaml config.yaml
```

Set secrets via environment variables (recommended):

```bash
export DATABASE_PASSWORD="your-db-password"
export JWT_SECRET="your-jwt-secret"
export EMAIL_PASSWORD="your-smtp-password"
export CHATWORK_API_TOKEN="your-chatwork-token"
```

Cập nhật thông tin database trong `config.yaml`:

```yaml
database:
  host: "localhost"
  port: 3306
  username: "root"
  password: "your-password"
  dbname: "foods_drinks"
```

### 3. Tạo Database

```sql
CREATE DATABASE foods_drinks CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
```

### 4. Chạy Migration

```bash
# Chạy tất cả migrations
make migrate-up

# Hoặc chạy trực tiếp
go run ./cmd/migrate -command=up
```

### 5. Chạy Server

```bash
make run

# Hoặc
go run ./cmd/server
```

## Migration Commands

```bash
# Chạy tất cả migrations
make migrate-up

# Rollback tất cả migrations
make migrate-down

# Xem version hiện tại
make migrate-version

# Chạy N migrations
make migrate-up-steps STEPS=1

# Rollback N migrations
make migrate-down-steps STEPS=1

# Force version (khi bị dirty)
make migrate-force VERSION=12
```

## Test Email Notification Locally

Configure your local `config.yaml` to use MailHog (default values in `config.example.yaml`):

- SMTP host: `localhost`
- SMTP port: `1025`
- Mail inbox UI: `http://localhost:8025`

MailHog config snippet:

```yaml
email:
  enabled: true
  smtp_host: "localhost"
  smtp_port: 1025
  username: ""
  password: ""
  from_email: "no-reply@foods-drinks.local"
  from_name: "Foods & Drinks"
  admin_recipient: "admin@foods-drinks.local"
  subject_prefix: "[Foods & Drinks]"
  order_template_path: "templates/email/new_order.html"
  max_retries: 3
  retry_delay_seconds: 3
  max_workers: 4
  queue_size: 100
```

Note: current implementation uses plain SMTP via `net/smtp.SendMail` (no STARTTLS/SMTPS flow).
Use MailHog for local testing, or a relay that accepts plain SMTP.

Run services:

```bash
docker compose up -d
```

Then run server and create a new order. Email notification will be sent to MailHog and you can view it in the UI at `http://localhost:8025`.

## Test Chatwork Notification Locally

`docker-compose.yml` now includes a WireMock service to simulate Chatwork API at `http://localhost:8089`.

Note: this is a mock API server (WireMock), not the real Chatwork web UI. You can inspect mappings and requests via `http://localhost:8089/__admin`.

Use this config snippet in `config.yaml`:

```yaml
chatwork:
  enabled: true
  base_url: "http://localhost:8089"
  api_token: "local-chatwork-token"
  room_id: "dev-room"
  message_prefix: "[Foods & Drinks]"
  max_retries: 3
  retry_delay_seconds: 3
  max_workers: 2
  queue_size: 100
  timeout_seconds: 5
```

Start containers:

```bash
docker compose up -d
```

When creating a new order, app will send a POST request to `/v2/rooms/{room_id}/messages` on WireMock and log status in `order_notifications` with type `chatwork`.

## Database Schema

Hệ thống bao gồm 12 bảng:

1. **users** - Quản lý người dùng
2. **social_auths** - Đăng nhập qua mạng xã hội
3. **categories** - Danh mục sản phẩm
4. **products** - Sản phẩm (food/drink)
5. **product_images** - Hình ảnh sản phẩm
6. **carts** - Giỏ hàng
7. **cart_items** - Sản phẩm trong giỏ hàng
8. **orders** - Đơn hàng
9. **order_items** - Chi tiết đơn hàng
10. **ratings** - Đánh giá sản phẩm
11. **suggestions** - Đề xuất sản phẩm mới
12. **order_notifications** - Thông báo đơn hàng

## License

MIT
