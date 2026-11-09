# Journey Safety — Phase 1

Backend nền cho ứng dụng giám sát hành trình và cảnh báo người thân. UI Web/PWA, định tuyến, GPS và SOS sẽ được triển khai ở các phase sau; Phase 1 hiện cung cấp API authentication và trusted contacts có thể chạy và kiểm thử thật.

## Chạy nhanh bằng Docker

Cần Docker Engine + Docker Compose và Python 3. Không cần Node, Android SDK, Xcode hoặc Make để chạy Phase 1 bằng container.

```bash
./dev env
./dev up
curl http://localhost:8080/health/ready
```

`./dev env` sinh `.env` riêng với password ngẫu nhiên và quyền 0600; không ghi đè file hiện có. Mặc định API ở `127.0.0.1:8080`, DB ở `127.0.0.1:55432`. Đổi API_PORT/DB_PORT trong `.env` nếu cổng đang dùng; khi đổi DB_PORT cập nhật DATABASE_URL tương ứng. `./dev up` build API, chờ DB healthy, chạy migration rồi khởi động API. DB dùng volume riêng nên restart không mất dữ liệu.

```bash
./dev down  # dừng container, giữ volume
```

Đây là cấu hình local. Production cần HTTPS reverse proxy, `APP_ENV=production`, `COOKIE_SECURE=true`, origin HTTPS chính xác và DB role ứng dụng ít quyền. Không expose DB ra internet hoặc chạy cấu hình local như production.

## Chạy backend local bằng Go

Cần Go 1.27+. Mọi dependency đã pin trong `backend/go.mod` và `go.sum`.

```bash
./dev env
./dev db
./dev migrate
./dev api
```

Nếu trước đó đã chạy toàn bộ Compose, dừng service API trước để giải phóng cổng:

```bash
docker compose --env-file .env -f infrastructure/compose.yaml stop api
```

Các lệnh tương đương có trong Makefile nếu máy có Make. Script `dev` đọc `.env` như file shell; chỉ dùng file cấu hình do bạn tin cậy. Không commit file này.

## Chức năng Phase 1

- Đăng ký / đăng nhập email + mật khẩu, Argon2id, phiên opaque lưu hash trong DB.
- Cookie HttpOnly; CSRF + Origin/CORS allowlist; chống brute-force cơ bản trên một API instance.
- Xem/sửa tên hồ sơ, liệt kê phiên và thu hồi một hoặc mọi phiên.
- Mời người thân bằng email; accept cần đăng nhập đúng email và có mã mời bí mật. Hỗ trợ tên gợi nhớ, decline/cancel, hết hạn, giới hạn tối đa 3 quan hệ/lời mời đang chờ, thu hồi từ hai phía.
- Quan hệ tin cậy **không cấp quyền xem hành trình**. Quyền sẽ được tạo riêng khi người dùng chủ động chia sẻ hoặc có cảnh báo ở phase 4–5.

Mã mời chỉ xuất hiện khi tạo lời mời. Owner tự gửi invitation ID + mã qua kênh riêng; hệ thống chưa gửi email/SMS. Người nhận có thể đăng ký trước hoặc sau khi nhận lời mời. Email tài khoản chưa có verification/recovery provider; cần bổ sung trước pilot công khai. Không đưa mã mời, session hoặc CSRF token vào URL/log.

## Ví dụ API

Ví dụ dùng dữ liệu giả, chỉ phục vụ local development:

```bash
curl -i -c /tmp/journey.cookies \
  -H 'Content-Type: application/json' \
  -H 'Origin: http://localhost:5173' \
  -d '{"email":"demo@example.com","password":"mat-khau-thu-nghiem-123","display_name":"Người dùng thử"}' \
  http://localhost:8080/api/v1/auth/register

curl -b /tmp/journey.cookies \
  http://localhost:8080/api/v1/auth/session
```

Lấy `data.csrf_token` từ response đăng nhập hoặc GET session, rồi truyền qua `X-CSRF-Token` cho POST/PATCH/DELETE có xác thực. Frontend dùng `fetch(..., {credentials: 'include'})`; không đọc được cookie HttpOnly và không lưu session trong localStorage. Xóa cookie jar sau thử nghiệm. Xem đầy đủ request/response ở [API contracts](docs/api-contracts.md).

## Kiểm thử

```bash
./dev check        # go vet + unit tests (integration sẽ SKIP nếu chưa có test DSN)
./dev integration  # DB test riêng, kiểm thử HTTP + SQL thật, không dùng mock DB
./dev test-docker  # toàn bộ test + Go race detector, không cần gcc trên host
./dev restore-check # local: test API thật, restart DB/API và restore sang DB riêng
```

DB test dùng cổng 55433 và tmpfs, tách hoàn toàn volume DB dev. Suite chỉ nhận TEST_DATABASE_URL kết thúc bằng `_test`, tạo database UUID riêng rồi xóa database đó khi hoàn tất. Không trỏ test vào DB chứa dữ liệu thật. Race detector trong container có sẵn C compiler; Go local không cần bật CGO.

Kiểm thử bao gồm migrations down/up, PostGIS đo mét và TimescaleDB hypertable thật, auth/CSRF/revoke/expiry, sai email/mã mời, giới hạn đồng thời, accept/cancel cạnh tranh, từ chối/hủy/thu hồi/mời lại, log không chứa secret. Kết quả và phần chưa kiểm chứng: [testing.md](docs/testing.md).

## Migration / backup

```bash
./dev migration-status
./dev backup
```

Migration SQL được embed vào binary; chạy riêng có advisory lock. `cd backend && go run ./cmd/migrate down` sau khi load environment lùi **một** migration: từ version 3 bỏ label, lùi tiếp version 2 sẽ xóa bảng/dữ liệu foundation. Chỉ thử rollback trên DB thử nghiệm. Rollback không gỡ TimescaleDB/PostGIS.

Backup là custom-format pg_dump trong `backups/` (không commit). `./dev restore-check` chỉ chạy development khi hệ thống đã được `./dev up`; tạo hai tài khoản giả, kiểm tra contact qua API container, restart riêng DB/API dự án, backup/restore sang database UUID riêng rồi dọn dữ liệu thử nghiệm. Không chạy đồng thời nhiều restore-check hoặc trong lúc đang thao tác API. Chưa tự lên lịch hoặc đưa offsite; quy trình khôi phục và production: [operations.md](docs/operations.md).

## Tài liệu và phạm vi tiếp theo

- [Kiến trúc và quyết định](docs/architecture.md)
- [ERD / thiết kế schema](docs/data-model.md)
- [Trip / tracking / alert states](docs/states.md)
- [API contracts](docs/api-contracts.md)
- [Roadmap và nghiệm thu](docs/roadmap.md)

Phase 2 dùng ORS cho routing; HeiGIT Pelias cho search/autocomplete/reverse **chỉ Việt Nam**; MapLibre GL JS + OpenFreeMap. Chưa cần API key HeiGIT để chạy Phase 1. Chưa có frontend, live tracking, Web Push, SMS, quảng cáo, thanh toán hoặc simulator. PWA tương lai không bảo đảm GPS nền khi khóa điện thoại.
