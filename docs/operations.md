# Vận hành Phase 1

## Local

`./dev up` chạy DB+migrate+API; `./dev down` không xóa volume. `docker compose --env-file .env -f infrastructure/compose.yaml ps` xem tình trạng; `... logs --tail 50 api` xem log. Không dùng `docker compose config` rồi chia sẻ output vì có thể chứa credentials được interpolate.

API chạy non-root, root filesystem read-only, không có Linux capabilities; DB và API chỉ bind host loopback. Image DB đã pin digest. Đây vẫn là một DB instance; tên image `timescaledb-ha` không tạo high availability tự động.

Database superuser của Compose chỉ tiện cho development/migration. Production phải tách role migration (DDL/extensions) khỏi role API (DML các bảng nghiệp vụ, không CREATE/role admin); API từ chối khởi động production nếu DB role có superuser/createdb/createrole/replication/bypassrls. Trong Compose cấu hình `API_DATABASE_URL` và `MIGRATION_DATABASE_URL` riêng (hostname `db`, port 5432). Khi chạy local bằng Go, dùng `DATABASE_URL` tương ứng từng lệnh. Role API cần USAGE schema public, SELECT/INSERT/UPDATE/DELETE trên users, user_sessions, trusted_contacts, trusted_contact_invitations; không cần quyền migration hay quản lý role. Tránh cấp membership vào role quản trị. Cấu hình DATABASE_URL không ghi vào log; backup và DB volume cần mã hóa ở tầng lưu trữ, quyền truy cập và bản sao offsite phù hợp.

## HTTPS

Dùng reverse proxy cùng origin cho frontend và `/api`, chuyển tiếp đến backend loopback. `APP_ENV=production`, `COOKIE_SECURE=true`, `ALLOWED_ORIGINS=https://ten-mien-cua-ban` (không slash cuối). Header proxy chỉ tin nếu proxy nằm trong `TRUSTED_PROXIES` cụ thể, mặc định không tin X-Forwarded-For. Không trust wildcard CIDR. Rate limit trong RAM không chia sẻ giữa nhiều instance.

## Backup / restore có TimescaleDB

`./dev backup` tạo dump quyền riêng tư (umask 077). Chọn bản sao bằng đường dẫn cụ thể; không ghi mật khẩu trực tiếp vào command line. Tự thiết lập lịch, mã hóa và offsite trước production.

Ví dụ khôi phục vào **DB mới riêng** tên `journey_restore_test`, chạy từ root dự án. Không thay tên bằng database đang phục vụ:

```bash
docker compose --env-file .env -f infrastructure/compose.yaml exec -T db \
  createdb -U journey journey_restore_test

docker compose --env-file .env -f infrastructure/compose.yaml exec -T db \
  psql -v ON_ERROR_STOP=1 -U journey -d journey_restore_test \
  -c 'CREATE EXTENSION IF NOT EXISTS timescaledb; CREATE EXTENSION IF NOT EXISTS postgis;'

docker compose --env-file .env -f infrastructure/compose.yaml exec -T db \
  psql -v ON_ERROR_STOP=1 -U journey -d journey_restore_test \
  -c 'SELECT timescaledb_pre_restore();'

# Thay backups/<file>.dump bằng đúng file backup vừa tạo.
docker compose --env-file .env -f infrastructure/compose.yaml exec -T db \
  pg_restore --exit-on-error --no-owner -U journey -d journey_restore_test < backups/<file>.dump

docker compose --env-file .env -f infrastructure/compose.yaml exec -T db \
  psql -v ON_ERROR_STOP=1 -U journey -d journey_restore_test \
  -c 'SELECT timescaledb_post_restore(); SELECT count(*) FROM users;'
```

Nếu đã đổi POSTGRES_USER, thay `journey` tương ứng. Sau khi đối chiếu row counts/schema và xác minh xong, có thể xóa riêng database restore test bằng `dropdb`. Không xóa volume ứng dụng để thử restore. TimescaleDB yêu cầu đúng quy trình pre/post restore khi đã có hypertables ở phase 3.

## Quan sát và giới hạn

`/health/live` cho process, `/health/ready` kiểm tra PostgreSQL 17/extensions/bảng foundation. Runtime SQL timeout 10s, idle transaction 15s, pool mặc định 10. Không bật query/body logs vì có thể có email, token và GPS ở các phase sau. Logs HTTP chỉ route template/request ID/status/latency.

Chưa đo công suất 100–1.000 người dùng/chuyến đồng thời. Cần kiểm thử tải và browser/device ở phase phù hợp. Xác minh email, khôi phục mật khẩu, account deletion, GPS retention và chính sách dữ liệu sự cố cần hoàn thiện trước pilot công khai; không coi Phase 1 là hệ thống cảnh báo hoàn chỉnh.
