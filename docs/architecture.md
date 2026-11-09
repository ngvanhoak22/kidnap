# Journey Safety — kiến trúc và quyết định đã chốt

Trạng thái: Phase 1 hoàn tất và đã kiểm thử local ngày 2026-11-06. Đây là dự án hiện tại, không phải dự án mobile mới.

## Nghiệp vụ và phạm vi

- Web/PWA trước, React Native sau. Frontend tương lai: React/TypeScript/Vite, React Router, MapLibre GL JS, OpenFreeMap, vite-plugin-pwa, IndexedDB.
- Go + Gin modular monolith; PostgreSQL **17**, PostGIS và TimescaleDB trong cùng database. REST cho nghiệp vụ, WebSocket và Go workers ở các phase tiếp theo. Không Redis/Kafka/Kubernetes.
- ORS `driving-car` cho định tuyến; HeiGIT Pelias là nhà cung cấp geocoding duy nhất. Backend giữ API key. Tìm kiếm **chỉ Việt Nam** (`boundary.country=VNM`); autocomplete 600 ms, ít nhất 3 ký tự, tối đa 5 gợi ý; hủy/bỏ kết quả cũ. Chọn tọa độ trực tiếp trên bản đồ vẫn hoạt động khi geocoding lỗi. Reverse chỉ bổ sung nhãn, không quyết định tọa độ.
- Trusted contact phải xác nhận lời mời. Tối đa 3 contact đang hiệu lực, mỗi chiều là một quan hệ riêng. Một contact không tự có quyền xem GPS.
- Quyền xem từng chuyến chỉ được cấp khi **chủ chuyến chủ động chia sẻ** hoặc **cảnh báo đã leo thang tới người thân (ESCALATED)** cho những người nhận đã được chủ chuyến chọn. Bước yêu cầu người dùng xác nhận an toàn chưa cấp quyền xem cho người thân. Không mặc định chia sẻ lúc bắt đầu. Thu hồi contact phải vô hiệu quyền xem và đóng WebSocket liên quan ở phase 5.
- PWA chỉ là bản thử nghiệm chức năng; không bảo đảm GPS khi khóa màn hình, background hoặc hệ điều hành đóng trang. Tách mất GPS, mất heartbeat và lệch tuyến. Không dùng GPS cũ vừa đồng bộ để tạo cảnh báo realtime.

## Các lớp backend

`handler (Gin/HTTP) → service (validation/policy) → repository (pgx/SQL) → PostgreSQL`.

`internal/config`, `internal/platform/database`, `internal/platform/security`, `internal/platform/httpapi` là hạ tầng dùng chung. Module `auth` và `contacts` độc lập về nghiệp vụ; contacts tham chiếu người dùng qua khóa ngoại. Migration chạy bởi lệnh riêng, không tự chạy trong từng API replica. Database advisory lock của migration runner ngăn chạy migration đồng thời.

Phase sau bổ sung `routing`, `geocoding`, `trips`, `tracking`, `alerts`, `notifications`, `sharing`; các provider nằm sau interface. Không tạo endpoint giả trả thành công cho nghiệp vụ chưa triển khai.

## Authentication / invitation

- Email chuẩn hóa lowercase, mật khẩu 12–128 ký tự Unicode; Argon2id, salt ngẫu nhiên. Không SMS OTP. Email không được tự coi là đã xác minh.
- Session opaque 256 bit; database chỉ lưu SHA-256. Cookie HttpOnly, SameSite=Lax, Secure bắt buộc khi production. TTL tuyệt đối 7 ngày mặc định; mọi request xác thực kiểm tra expiry/revocation trong DB. GET session cung cấp CSRF token cho tab mới; token xác định từ session secret, không cần lưu localStorage.
- Request thay đổi dữ liệu có session cần `X-CSRF-Token`; CORS chỉ cho phép origin cấu hình, không wildcard. Login/register chỉ JSON, kiểm tra Origin khi có. Rate limit theo địa chỉ kết nối thật; không tin X-Forwarded-For từ internet. Dùng reverse proxy local loopback được cấu hình tường minh khi cần.
- Mời bằng email + mã ngẫu nhiên 256 bit dùng một lần, hết hạn mặc định 72 giờ. Người mời nhận mã **một lần** và tự gửi qua kênh riêng đã biết; mã không xuất hiện trong danh sách, log hoặc GET. Người nhận cần đăng nhập đúng email và cung cấp mã để accept. Biết email, user ID hay invitation ID đều không đủ. Không giả lập việc đã gửi email/SMS.
- Pending invitation giữ chỗ trong giới hạn 3 contact để không chấp nhận quá số lượng. Các thao tác thay đổi contact khóa hàng owner theo thứ tự nhất quán; accept và cancel cạnh tranh được xử lý trong transaction. Hết hạn tính bằng giờ database, không phụ thuộc worker.
- Người mời hủy lời mời; người nhận từ chối; cả hai phía được thu hồi quan hệ đã xác nhận. Thu hồi không hard-delete lịch sử xác nhận.

## Các trường hợp ngoại lệ

Sai mật khẩu/tài khoản không tồn tại cùng lỗi xác thực; session hết hạn/thu hồi trả 401; người ngoài truy cập tài nguyên trả 404; CSRF sai trả 403. Lời mời hết hạn, đã xử lý hoặc quá số contact trả lỗi nghiệp vụ nhất quán. Request lặp accept cùng mã cho cùng contact đang hiệu lực trả kết quả cũ; không phục hồi contact đã thu hồi bằng mã cũ.

Đăng ký email trùng trả 409: đây là đánh đổi MVP đã được ghi rõ, có thể tiết lộ tài khoản đã đăng ký. Không có API tra cứu user theo email. Trước pilot công khai cần bổ sung xác minh email và khôi phục mật khẩu qua kênh đã xác thực; mã mời giúp bảo vệ quan hệ tin cậy nhưng không thay thế xác minh email của tài khoản.

## Triển khai và quan sát

DB chỉ bind loopback trên host; backend chạy non-root trong container. HTTPS kết thúc ở reverse proxy khi production; cấu hình Secure cookie/origin được kiểm tra khi khởi động. Application log chỉ có request ID, route template, status, latency; không log body, query, email, mật khẩu, session, mã mời, token push hoặc GPS.

Liveness không gọi DB; readiness kiểm tra DB/extensions/schema. Graceful shutdown, pool giới hạn, timeout request. Backup dùng pg_dump và kiểm thử restore vào database riêng; không coi một VPS là HA. In-memory auth limiter chỉ dành một instance MVP; nhiều replica cần limiter dùng PostgreSQL hoặc lớp proxy chung.

## Cấu trúc

```
backend/
  cmd/api/                 # HTTP server
  cmd/migrate/             # embedded SQL migration runner
  internal/auth/           # model, service, repository, handler
  internal/contacts/       # model, service, repository, handler
  internal/config/
  internal/platform/{database,httpapi,security}/
  migrations/
  tests/integration/
infrastructure/            # compose, scripts, Dockerfile
docs/                     # đặc tả, ERD, API, trạng thái, kiểm thử
web/                      # chưa triển khai, Phase 2
mobile/                   # chưa triển khai, dự phòng React Native
```

## Tài liệu nguồn kiểm tra ngày 2026-11-06

- Gin: https://gin-gonic.com/en/docs/quickstart/
- Timescale HA image có PostGIS: https://www.tigerdata.com/docs/get-started/choose-your-path/install-timescaledb
- PG17 compatibility: https://www.tigerdata.com/docs/get-started/timescaledb-supported-platforms
- Argon2id: https://cheatsheetseries.owasp.org/cheatsheets/Password_Storage_Cheat_Sheet.html
- HeiGIT URL migration: https://ask.openrouteservice.org/t/deprecating-api-openrouteservice-org-in-favour-of-api-heigit-org/7912
- Pelias: https://github.com/pelias/documentation

Dependency/image thực tế được pin trong go.mod/go.sum và Compose; kết quả xác minh nằm trong testing.md.
