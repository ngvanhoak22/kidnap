# PROJECT SPECIFICATION – Journey Safety Monitoring & Emergency Alert App

## 1. Tổng quan dự án

Xây dựng ứng dụng di động giúp người dùng giám sát an toàn trong quá trình di chuyển, đặc biệt khi đi taxi, xe khách, xe đường dài hoặc di chuyển một mình.

Người dùng chọn điểm đến và xác nhận tuyến đường dự kiến. Ứng dụng tự động theo dõi GPS trong chuyến đi, phát hiện khi người dùng di chuyển ra ngoài hành lang an toàn, yêu cầu xác nhận và cảnh báo người thân tin cậy nếu không nhận được phản hồi.

Ứng dụng miễn phí cho người dùng. Giai đoạn đầu phục vụ 100–1.000 người dùng, ưu tiên chi phí hạ tầng thấp, bảo mật và khả năng mở rộng về sau.

Ứng dụng là công cụ hỗ trợ cảnh báo, không được tuyên bố có khả năng xác định bắt cóc, bảo đảm cứu hộ hoặc thay thế cơ quan chức năng.

## 2. Technology Stack

### Mobile
- React Native + TypeScript.
- MapLibre React Native để render bản đồ.
- OpenFreeMap làm map tile provider.
- GPS API native, hỗ trợ background location trên Android và iOS.
- Firebase Cloud Messaging để nhận push notification.
- WebSocket để hiển thị các cập nhật hành trình cho người được cấp quyền khi cần.

Sử dụng React Native New Architecture tương thích MapLibre. Nếu dùng Expo, phải sử dụng development build thay vì chỉ dựa vào Expo Go.

### Backend
- Golang.
- Gin HTTP Framework.
- Kiến trúc Modular Monolith, phân tách rõ handler, service, repository, model và infrastructure.
- REST API cho các nghiệp vụ chính.
- WebSocket cho live tracking.
- Background Worker bằng Go để giám sát heartbeat, timeout xác nhận và gửi cảnh báo.

### Database
- PostgreSQL 17.
- PostGIS để lưu và truy vấn dữ liệu địa lý.
- TimescaleDB để lưu lịch sử GPS dạng hypertable.
- Sử dụng database migration để quản lý schema.

### External Services
- OpenFreeMap: Map Tiles.
- openrouteservice: Routing API, profile driving-car.
- Firebase FCM: Push Notification.
- SMS Provider: Thiết kế interface để dễ dàng tích hợp nhà cung cấp SMS Việt Nam.

### Infrastructure
- Docker Compose.
- VPS Linux.
- HTTPS.
- Environment variables để cấu hình.
- Không sử dụng Redis, Kafka, Kubernetes hoặc Microservices trong MVP.

## 3. Functional Requirements

### Module 1 – Authentication & Users

- Đăng ký, đăng nhập và đăng xuất.
- Quản lý hồ sơ người dùng.
- Bảo vệ các API bằng authentication.
- Có chức năng quản lý phiên đăng nhập và thu hồi phiên.
- Thiết kế cơ chế xác thực đơn giản, an toàn và phù hợp MVP.
- Không tự thêm SMS OTP trả phí khi chưa cần thiết.

### Module 2 – Trusted Contacts

- Người dùng thêm 1–3 người thân tin cậy.
- Người thân phải xác nhận lời mời trước khi trở thành trusted contact.
- Có thể chỉnh sửa, xóa hoặc thu hồi quyền chia sẻ.
- Cấu hình người nhận cảnh báo cho từng chuyến đi.
- Không cho phép theo dõi vị trí của một người chỉ bằng việc biết số điện thoại hoặc ID.
- Chỉ chia sẻ vị trí theo phạm vi và thời gian mà chủ tài khoản đã chấp thuận.

### Module 3 – Route Planning

- Lấy GPS hiện tại làm điểm bắt đầu.
- Người dùng chọn điểm đến trực tiếp trên bản đồ.
- Mobile gửi tọa độ điểm bắt đầu và điểm kết thúc đến backend.
- Backend gọi openrouteservice Directions API để tính tuyến đường dành cho ô tô.
- Nhận GeoJSON LineString từ routing API.
- Trả dữ liệu tuyến đường về React Native.
- MapLibre hiển thị tuyến đường trên nền OpenFreeMap.
- Người dùng xem trước và xác nhận tuyến đường.
- Nếu routing API hỗ trợ nhiều tuyến hợp lệ, có thể cho người dùng chọn tuyến phù hợp.
- Lưu tuyến được xác nhận bằng PostGIS.
- Không cần gọi lại routing API mỗi lần GPS thay đổi.
- Không coi tuyến do routing engine đề xuất là tuyến chắc chắn an toàn.

### Module 4 – Safe Corridor

- Tạo hành lang an toàn quanh tuyến đường dự kiến.
- Sử dụng PostGIS và các phép toán khoảng cách như ST_DWithin.
- Ngưỡng lệch tuyến có thể cấu hình.
- Giá trị thử nghiệm ban đầu: 150 mét.
- Không cảnh báo chỉ vì một mẫu GPS đơn lẻ nằm ngoài hành lang.
- Xem xét GPS accuracy, số mẫu liên tiếp và thời gian lệch tuyến.
- Cho phép xác nhận việc thay đổi tuyến khi cần.
- Không tự động coi mọi trường hợp lệch tuyến là nguy hiểm.

### Module 5 – Trip Management

- Tạo chuyến đi.
- Xem trước tuyến đường.
- Bắt đầu theo dõi.
- Xem trạng thái chuyến đi.
- Kết thúc chuyến.
- Hủy chuyến.
- Xem lịch sử chuyến đi.

Trạng thái chuyến đi đề xuất:
- DRAFT
- ACTIVE
- COMPLETED
- CANCELED

Trạng thái cảnh báo quản lý riêng, không gộp hoàn toàn vào trạng thái chuyến đi.

### Module 6 – GPS Tracking

Khi chuyến đi ACTIVE:

- Mobile thu thập tọa độ GPS.
- Hỗ trợ tracking khi khóa màn hình trong giới hạn được hệ điều hành cho phép.
- Chỉ tracking sau khi người dùng cấp quyền và chủ động bắt đầu chuyến.
- Gửi latitude, longitude, accuracy, speed và timestamp.
- Thử nghiệm chu kỳ gửi ban đầu 15 giây.
- Có thể điều chỉnh tần suất theo trạng thái chuyến đi.
- Lưu vị trí mới nhất của chuyến đi.
- Ghi lịch sử GPS vào TimescaleDB.
- Backend phải xử lý dữ liệu GPS đến trễ, trùng lặp hoặc không đúng thứ tự.
- Mobile có thể tạm lưu dữ liệu để đồng bộ sau khi có kết nối, nhưng phải phân biệt timestamp đo GPS với timestamp server nhận.
- Có heartbeat để server nhận biết thiết bị còn liên lạc hay không.

Không mặc định rằng background GPS luôn hoạt động chính xác trên mọi thiết bị.

### Module 7 – Deviation Detection

- Mobile có thể kiểm tra lệch tuyến cục bộ để cảnh báo sớm.
- Backend kiểm tra lại vị trí mới nhất so với tuyến đã xác nhận bằng PostGIS.
- Nếu nhiều mẫu GPS liên tiếp lệch ngoài ngưỡng và có độ chính xác phù hợp, tạo sự kiện bất thường.
- Gửi yêu cầu người dùng xác nhận an toàn.
- Nếu người dùng xác nhận an toàn, ghi nhận sự kiện và tiếp tục giám sát.
- Nếu người dùng yêu cầu hỗ trợ hoặc không phản hồi quá thời hạn cấu hình, chuyển sang quy trình cảnh báo.
- Chống tạo nhiều cảnh báo trùng cho cùng một sự cố.

Các ngưỡng 150 mét, 30–60 giây lệch tuyến và 60–120 giây chờ xác nhận chỉ là giá trị cấu hình thử nghiệm, không phải ngưỡng an toàn được bảo đảm.

### Module 8 – Emergency Alert

Hỗ trợ ba trường hợp:

**A. Người dùng bấm SOS**
- Kích hoạt cảnh báo ngay, không đợi phát hiện lệch tuyến.
- Gửi yêu cầu đến server.
- Server tạo sự kiện khẩn cấp.
- Gửi cảnh báo cho người thân đã được chọn.

**B. Lệch tuyến và không phản hồi**
- Gửi cảnh báo xác nhận đến người dùng.
- Backend quản lý deadline xác nhận.
- Nếu quá hạn mà không có phản hồi, gửi cảnh báo đến người thân.

**C. Mất liên lạc**
- Backend theo dõi heartbeat.
- Nếu mất heartbeat quá ngưỡng, tạo cảnh báo mất liên lạc.
- Cung cấp vị trí cuối cùng cùng timestamp và trạng thái liên lạc.
- Không diễn giải mất liên lạc thành bằng chứng của một vụ bắt cóc.

Notification Strategy:
- Sử dụng FCM cho push notification.
- Có cơ chế gửi SMS dự phòng cho cảnh báo cần leo thang.
- Có retry, idempotency và lưu lịch sử kết quả gửi.
- Có thể triển khai notification worker dựa trên PostgreSQL, chưa cần Redis.
- Không sử dụng quảng cáo, routing API hay map tile service như điều kiện bắt buộc để gửi SOS.

### Module 9 – Live Location Sharing

- Người thân tin cậy có thể xem hành trình trong phạm vi quyền được cấp.
- Hiển thị vị trí mới nhất, timestamp và độ chính xác GPS nếu có.
- Hiển thị tuyến đường dự kiến và các vị trí lịch sử được phép xem.
- Sử dụng WebSocket khi cần cập nhật realtime.
- Tự động ngừng quyền truy cập khi hết hạn hoặc bị thu hồi.
- Không sử dụng public link có thể bị người lạ đoán hoặc truy cập không cần xác thực.

### Module 10 – Trip History

- Lưu lịch sử GPS bằng TimescaleDB.
- Hypertable phân chia theo recorded_at.
- Truy vấn lịch sử theo trip_id và thời gian.
- Hỗ trợ vẽ lại hành trình trên MapLibre.
- Thiết kế chính sách lưu trữ giới hạn, dự kiến tối đa 30 ngày cho hành trình thông thường.
- Dữ liệu liên quan sự cố xử lý theo chính sách lưu trữ riêng.
- Người dùng có thể quản lý và yêu cầu xóa dữ liệu theo chính sách áp dụng.

## 4. Database Design

Thiết kế schema tối thiểu gồm:

1. users
2. user_sessions
3. trusted_contacts
4. trusted_contact_invitations
5. trips
6. trip_routes
7. trip_current_locations
8. trip_location_history (TimescaleDB hypertable)
9. trip_alerts
10. safety_confirmations
11. notification_deliveries
12. device_push_tokens

Yêu cầu:
- Định nghĩa đầy đủ primary key, foreign key, index và constraints.
- Dùng PostGIS geography/geometry đúng mục đích.
- Lưu tuyến dưới dạng LineString với SRID 4326.
- Lưu lịch sử GPS với recorded_at TIMESTAMPTZ.
- Tạo index (trip_id, recorded_at) trên hypertable.
- Các UNIQUE constraint trên hypertable phải tuân thủ yêu cầu của TimescaleDB về partition key.
- Không tạo index không cần thiết trên mọi bản ghi GPS.
- Có migrations cài extension PostGIS/TimescaleDB.
- Không lưu thông tin vị trí nhạy cảm trong application logs thông thường.

## 5. API Design

Thiết kế REST API theo nhóm:

Authentication:
- POST /api/v1/auth/register
- POST /api/v1/auth/login
- POST /api/v1/auth/logout

Trusted Contacts:
- POST /api/v1/trusted-contacts
- GET /api/v1/trusted-contacts
- DELETE /api/v1/trusted-contacts/:id
- API mời và xác nhận trusted contact

Routing:
- POST /api/v1/routes/preview

Trips:
- POST /api/v1/trips
- POST /api/v1/trips/:id/start
- POST /api/v1/trips/:id/complete
- POST /api/v1/trips/:id/cancel
- GET /api/v1/trips/:id
- GET /api/v1/trips/:id/history

Tracking:
- POST /api/v1/trips/:id/locations
- POST /api/v1/trips/:id/heartbeat

Safety & Alerts:
- POST /api/v1/trips/:id/confirm-safety
- POST /api/v1/trips/:id/sos
- GET /api/v1/trips/:id/alerts

Realtime:
- GET /api/v1/ws/trips/:id

Các endpoint chỉ là đề xuất ban đầu. Hãy thiết kế request/response, validation, authentication, authorization và error handling nhất quán. Chỉ chủ chuyến đi hoặc người được cấp quyền mới có thể truy cập dữ liệu liên quan.

## 6. Non-Functional Requirements

### Security & Privacy
- HTTPS bắt buộc.
- Xác thực và phân quyền mọi API nhạy cảm.
- Không đưa ORS API key hoặc SMS credentials vào mobile app.
- Bảo vệ dữ liệu vị trí và token truy cập.
- Không chia sẻ hoặc bán dữ liệu GPS cho bên quảng cáo.
- Có cơ chế đồng ý rõ ràng, thu hồi quyền chia sẻ và xóa dữ liệu.
- Hạn chế truy cập nội bộ đến dữ liệu hành trình.
- Thiết kế phù hợp với quy định bảo vệ dữ liệu cá nhân tại Việt Nam.

### Reliability
- Backend xử lý timeout độc lập với timer trên mobile.
- Khi ứng dụng mobile bị tắt hoặc mất mạng, server vẫn theo dõi deadline dựa vào heartbeat cuối cùng.
- Cảnh báo có cơ chế retry và chống gửi trùng.
- Có cơ chế theo dõi lỗi gửi notification.
- Có backup database và kiểm thử khôi phục.
- Có health check và monitoring.
- Thừa nhận rằng hệ thống không thể truyền cảnh báo nếu mất toàn bộ đường liên lạc, và một VPS đơn lẻ không bảo đảm tính sẵn sàng cao.

### Performance
- Mục tiêu ban đầu: 100–1.000 người dùng hoạt động.
- Tối ưu theo số chuyến và số thiết bị đang tracking đồng thời.
- Hỗ trợ batch GPS insertion khi phù hợp.
- Không gọi routing API với từng cập nhật GPS.
- Không thêm Redis hoặc Kafka trước khi có nhu cầu được đo đạc.
- Dùng PostGIS cho spatial queries.
- Dùng TimescaleDB cho time-series storage.

## 7. Mobile Screens

Thiết kế các màn hình:

1. Welcome / Login / Register
2. Home
3. Create Trip
4. Route Preview
5. Active Trip / Live Map
6. Safety Confirmation Modal
7. Emergency SOS
8. Trusted Contacts
9. Trip History
10. Trip Detail / Playback
11. Notifications
12. Settings / Privacy

Giao diện cần ưu tiên thao tác nhanh, dễ đọc, hỗ trợ tiếng Việt. Nút SOS dễ tiếp cận và không bị quảng cáo che khuất.

## 8. Out of Scope – MVP

Không triển khai:
- Redis.
- Kafka.
- Kubernetes.
- Microservices.
- AI phát hiện bắt cóc.
- Chấm điểm khu vực nguy hiểm.
- Camera hoặc ghi âm bí mật.
- Marketplace.
- Quảng cáo.
- Thanh toán.
- Tính năng định vị 3D.
- Admin dashboard phức tạp.
- Geocoding theo tên địa điểm, trừ khi được yêu cầu sau.

## 9. Development Deliverables

Hãy xây dựng dự án theo từng giai đoạn:

**Phase 1 – Foundation**
- Project structure Go + Gin.
- Docker Compose.
- PostgreSQL + PostGIS + TimescaleDB.
- Database migrations.
- Auth và trusted contacts.

**Phase 2 – Routing & Trips**
- Integration openrouteservice.
- Tạo và quản lý chuyến đi.
- Lưu tuyến vào PostGIS.
- Hiển thị MapLibre + OpenFreeMap trên React Native.

**Phase 3 – GPS Tracking**
- Background location.
- GPS ingestion.
- TimescaleDB hypertable.
- Heartbeat.
- Spatial deviation detection.

**Phase 4 – Safety & Emergency**
- Safety confirmation.
- SOS.
- Alert state machine.
- FCM.
- SMS provider adapter.
- Notification worker.

**Phase 5 – Live Sharing & Testing**
- Chia sẻ vị trí được phân quyền.
- WebSocket.
- Playback.
- Unit tests, integration tests.
- Kiểm thử mất mạng, GPS sai, khóa màn hình, hết pin, gửi thông báo thất bại và lỗi server.

## 10. Yêu cầu cách AI thực hiện

Trước khi lập trình:
1. Phân tích nghiệp vụ và các trường hợp ngoại lệ.
2. Đề xuất kiến trúc tổng thể.
3. Thiết kế ERD và database schema.
4. Thiết kế các trạng thái Trip và Alert.
5. Thiết kế API contracts.
6. Đưa ra cấu trúc source code backend và mobile.
7. Xác nhận các phụ thuộc, API và extension tương thích với phiên bản thực tế.

Sau đó triển khai từng phase, với code thực tế, migrations, cấu hình và test có thể chạy được. Không chỉ tạo pseudocode.

Ưu tiên kiến trúc đơn giản, dễ bảo trì và mở rộng. Không tự ý thêm công nghệ ngoài phạm vi MVP nếu chưa giải thích được nhu cầu.

Mục tiêu là một ứng dụng thực sự có thể thử nghiệm với 100–1.000 người dùng, không phải một bản demo chỉ hoạt động trong điều kiện lý tưởng.
