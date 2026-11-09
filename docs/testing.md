# Kiểm thử Phase 1

## Lệnh tái lập

- `./dev check`: go vet và unit tests config/security. Integration bỏ qua rõ ràng nếu không có TEST_DATABASE_URL.
- `./dev integration`: chạy database test tmpfs riêng; tạo database UUID, migrate, gọi API bằng HTTP test server và PostgreSQL thật, xóa database tạm.
- `./dev test-docker`: unit/integration tests với `go test -race -count=1 -v ./...`; compiler C chỉ nằm trong container.
- `./dev restore-check`: development-only; test API container, hai tài khoản giả và contact có label, restart DB/API, backup/restore sang database UUID riêng, so sánh các hàng user/contact, rồi dọn dữ liệu thử nghiệm.

Không dùng emulator hoặc tài khoản bên ngoài. Tests tự tạo email/token giả; không log giá trị secret.

## Các trường hợp tự động

1. Migrations up/down/up/repeated up; rollback giữ extensions; PostGIS đo geography theo mét; tạo/insert/drop hypertable thử nghiệm.
2. Argon2id salt riêng; password đúng/sai/malformed hash; token random 256 bit; validation Unicode/email.
3. Đăng ký, email canonical, trùng email; login sai mật khẩu/email không tồn tại; profile.
4. HttpOnly/SameSite cookie; CSRF thiếu/sai; origin ngoài allowlist; JSON malformed/unknown/trailing/oversize; spoofed X-Forwarded-For không bypass auth limiter.
5. Session list, logout, revoke một/mọi phiên, expiry; user không thu hồi được phiên người khác.
6. Contact chỉ xuất hiện sau accept đúng email + token; không tự reciprocal; recipient có thể xem ai đã thêm mình.
7. Wrong token/wrong recipient/outsider cancel/revoke bị từ chối; token không có trong list hoặc app logs.
8. Giới hạn 3 slot qua 12 yêu cầu cạnh tranh; accept/cancel đồng thời chỉ có một kết quả hợp lệ.
9. Expiry không phụ thuộc worker; duplicate invite; decline/cancel; revoke/reinvite không replay mã cũ.
10. Database không chứa mật khẩu/session plaintext; production role guard từ chối DB administrator, chấp nhận role không có quyền admin.

## Ghi nhận thực nghiệm

Đã chạy ngày **2026-11-06**, trên môi trường local của dự án:

| Kiểm tra | Kết quả |
|---|---|
| `go vet ./...` + unit tests | PASS |
| `./dev integration` — 10 integration tests, DB thật | PASS |
| `./dev test-docker` — toàn bộ tests + race detector | PASS, không phát hiện data race |
| `./dev up` — build API/migrator, startup theo dependency, readiness | PASS |
| `./dev restore-check` — API container, restart persistence, pg_dump/restore riêng | PASS; đã dọn hai tài khoản và database thử nghiệm |
| PostGIS geography + TimescaleDB hypertable probe | PASS |

Phiên bản đã chạy thực tế: Go **1.27.3**, Gin **1.12.0**, pgx **5.12.0**, Goose **3.29.0**, PostgreSQL **17.11**, PostGIS **3.6.4**, TimescaleDB **2.31.0**. Docker image DB pin digest trong Compose. `go.mod`/`go.sum` khóa Go dependencies. DB image có kích thước khá lớn vì bao gồm các extensions; chưa tối ưu image database cho production.

API và DB development được giữ chạy tại localhost sau nghiệm thu; service database test có thể dừng bằng Compose riêng mà không ảnh hưởng development.

## Chưa nằm trong Phase 1

Chưa nghiệm thu routing/geocoding thực tế, dữ liệu tiếng Việt từ Pelias, UI/PWA/GPS, browser background/lock-screen behavior, notification delivery hoặc load 100–1.000 người dùng. Đây là các đầu việc của Phase 2–5, không được suy ra từ việc tests foundation pass.
