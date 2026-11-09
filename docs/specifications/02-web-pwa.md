# CHANGE REQUEST – Chuyển Mobile MVP sang Web + PWA

Tôi muốn cập nhật một số quyết định kỹ thuật của dự án Journey Safety Monitoring & Emergency Alert đã mô tả trước đó.

**Quan trọng:** Đây là yêu cầu điều chỉnh bản đặc tả hiện tại, không phải tạo một dự án mới. Hãy giữ nguyên toàn bộ nghiệp vụ, backend architecture, database design và các yêu cầu bảo mật đã thống nhất, ngoại trừ những nội dung được thay đổi dưới đây.

## 1. Thay đổi định hướng triển khai

Thay vì phát triển React Native ngay từ đầu, chúng ta sẽ triển khai **Web Application kết hợp Progressive Web App (PWA)** trước.

Mục tiêu:

- Phát triển và kiểm thử các nghiệp vụ trên trình duyệt.
- Cho phép người dùng cài website lên màn hình chính như một ứng dụng PWA.
- Không yêu cầu Android Studio, Android SDK hoặc Xcode.
- Giữ nguyên backend để sau này phát triển ứng dụng React Native.
- MVP ban đầu hướng đến 100–1.000 người dùng thử nghiệm.
- Phiên bản Web/PWA là bản thử nghiệm chức năng, chưa được coi là hệ thống giám sát an toàn đáng tin cậy khi điện thoại bị khóa.

## 2. Updated Technology Stack

### Frontend

- React + TypeScript.
- Vite.
- React Router.
- MapLibre GL JS.
- OpenFreeMap làm Map Tile Provider.
- vite-plugin-pwa để quản lý PWA Manifest và Service Worker.
- Web Geolocation API để lấy vị trí.
- Web Push API và Notifications API cho thông báo, nếu trình duyệt hỗ trợ.
- IndexedDB cho dữ liệu tạm cần thiết khi mất mạng.
- WebSocket để cập nhật hành trình realtime.

### Backend – Giữ nguyên

- Go + Gin.
- Modular Monolith.
- REST API.
- WebSocket.
- Background Workers.
- PostgreSQL 17.
- PostGIS.
- TimescaleDB.
- Không Redis, Kafka hoặc Kubernetes.

### External Services – Giữ nguyên hoặc điều chỉnh

- openrouteservice: Tính tuyến đường.
- OpenFreeMap: Cung cấp nền bản đồ.
- MapLibre GL JS: Hiển thị bản đồ.
- Web Push sử dụng Service Worker và VAPID; chưa bắt buộc tích hợp Firebase FCM cho web.
- Thiết kế Notification Provider interface để sau này hỗ trợ Firebase FCM Native và SMS Provider.

## 3. PWA Requirements

Ứng dụng cần hỗ trợ:

- Web App Manifest.
- Service Worker.
- Cài đặt PWA lên màn hình chính trên trình duyệt hỗ trợ.
- Application Icons.
- Splash Screen nếu nền tảng hỗ trợ.
- Cache các static assets cần thiết.
- Trang thông báo mất kết nối.
- Nhận Web Push khi trình duyệt và nền tảng hỗ trợ.
- Giao diện Responsive, ưu tiên Mobile First.

Không cache tùy tiện các API chứa dữ liệu cá nhân, GPS, token hoặc hành trình.

Không giả định background sync, Web Push hoặc Service Worker luôn chạy ổn định trên mọi thiết bị.

## 4. GPS Tracking trên Web

Thay Native Background Location bằng Web Geolocation API:

- Sử dụng navigator.geolocation.watchPosition().
- Yêu cầu người dùng cấp quyền truy cập vị trí.
- Theo dõi GPS khi website đang hoạt động và được trình duyệt cho phép.
- Gửi GPS về backend thông qua REST API.
- Hỗ trợ accuracy, speed, heading và recorded_at khi có dữ liệu.
- Lưu lịch sử vào TimescaleDB.
- Dùng PostGIS để kiểm tra lệch tuyến.
- Có heartbeat để backend theo dõi kết nối trong phiên.

### Giới hạn bắt buộc phải xử lý

Web/PWA không đảm bảo GPS chạy liên tục khi:

- Người dùng khóa màn hình.
- Chuyển sang ứng dụng khác.
- Trình duyệt đưa trang vào background.
- Hệ điều hành tạm dừng hoặc đóng trang web.

Vì vậy:

1. Không được mô tả PWA là giải pháp background tracking đáng tin cậy tương đương ứng dụng native.
2. Khi trang bị ẩn hoặc không có GPS mới, trạng thái hành trình có thể chuyển sang TRACKING_INTERRUPTED.
3. Phân biệt rõ mất heartbeat, mất GPS và phát hiện lệch tuyến.
4. Không được tự kết luận người dùng đang gặp nguy hiểm vì website ngừng gửi GPS.
5. Nếu người dùng đã đồng ý nhận cảnh báo mất liên lạc, backend có thể gửi thông báo cho người thân sau thời hạn cấu hình, nhưng nội dung phải ghi rõ "Mất liên lạc – vị trí cuối cùng", không phải "Phát hiện bắt cóc".
6. Khi người dùng quay lại ứng dụng, hệ thống tiếp tục tracking sau khi xác minh quyền truy cập và trạng thái chuyến đi.
7. Nếu có đồng bộ GPS bị trì hoãn, không dùng các điểm GPS cũ để tạo cảnh báo lệch tuyến realtime.

Yêu cầu hiển thị rõ trạng thái Tracking Active, Tracking Interrupted, GPS Unavailable và Offline trong giao diện.

## 5. Route Planning & Map Rendering

Luồng giữ nguyên:

1. Người dùng mở website.
2. Lấy GPS hiện tại.
3. Người dùng chọn điểm đến bằng cách click/tap trên bản đồ.
4. React gọi Go backend.
5. Backend gọi openrouteservice API.
6. ORS trả GeoJSON.
7. Backend lưu hoặc chuẩn bị dữ liệu tuyến đường.
8. Frontend dùng MapLibre GL JS hiển thị tuyến đường trên OpenFreeMap.
9. Người dùng xác nhận tuyến và bắt đầu chuyến đi.
10. PostGIS kiểm tra vị trí so với hành lang an toàn.

Sử dụng GeoJSON LineString và Polygon để hiển thị tuyến đường, vùng an toàn và GPS tracking.

Không gọi lại openrouteservice với mỗi bản ghi GPS.

## 6. Safety & Emergency Alert

Giữ nguyên các chức năng nghiệp vụ:

- SOS.
- Phát hiện lệch tuyến.
- Yêu cầu xác nhận an toàn.
- Cảnh báo khi không phản hồi.
- Cảnh báo mất liên lạc.
- Chia sẻ hành trình cho người thân tin cậy.

Điều chỉnh cho Web/PWA:

- Safety Confirmation hiển thị khi ứng dụng đang hoạt động.
- Web Push hỗ trợ thông báo nếu trình duyệt cho phép.
- Backend quản lý deadline xác nhận, không phụ thuộc vào JavaScript timer trên trình duyệt.
- Không dựa vào việc người dùng đã nhận hoặc nhìn thấy push notification để kết luận họ đang an toàn.
- SMS Provider được thiết kế bằng interface, có thể dùng mock provider khi phát triển.
- Thử nghiệm các trường hợp lỗi bằng dữ liệu GPS giả lập.

## 7. Database & Backend Architecture

Giữ nguyên:

- users.
- user_sessions.
- trusted_contacts.
- trusted_contact_invitations.
- trips.
- trip_routes.
- trip_current_locations.
- trip_location_history.
- trip_alerts.
- safety_confirmations.
- notification_deliveries.

Bổ sung nếu cần:

- web_push_subscriptions để lưu endpoint, public keys và trạng thái Web Push subscription.
- tracking_status hoặc session tracking state để phân biệt ACTIVE, INTERRUPTED và UNAVAILABLE.
- Trường lưu thời gian GPS cuối cùng và thời gian heartbeat cuối cùng.

Dùng TimescaleDB hypertable để lưu GPS history.

Dùng PostGIS để kiểm tra vị trí so với hành lang an toàn.

Không thay đổi cấu trúc backend chỉ vì frontend chuyển từ React Native sang Web.

## 8. Project Structure

Đề xuất:

journey-safety/
- backend/ – Go + Gin
- web/ – React + TypeScript + Vite + PWA
- infrastructure/ – Docker Compose
- docs/ – ERD, API Contracts, Architecture
- mobile/ – Chưa triển khai, dành cho React Native sau này

Backend phải độc lập với frontend.

Các nghiệp vụ authentication, routing, trips, tracking, alerts và trusted contacts được cung cấp qua API, không phụ thuộc vào Web hay Mobile.

## 9. Developer Experience

- Frontend chạy bằng Vite Dev Server.
- Backend chạy local bằng Go.
- PostgreSQL + PostGIS + TimescaleDB chạy bằng Docker Compose.
- Cấu hình CORS phù hợp.
- Sử dụng localhost trong môi trường phát triển.
- Production sử dụng HTTPS.
- API keys và credentials phải đặt ở backend hoặc server configuration.
- Có file .env.example.
- Có README hướng dẫn chạy toàn bộ hệ thống.
- Không yêu cầu Android SDK hoặc emulator.

## 10. GPS Simulator – Bắt buộc cho Development

Tạo công cụ giả lập GPS để kiểm thử mà không cần di chuyển thật.

Simulator cần hỗ trợ:

- Chạy theo tuyến đường bình thường.
- Giả lập đi lệch tuyến.
- Giả lập GPS accuracy thấp.
- Giả lập mất GPS.
- Giả lập mất kết nối.
- Giả lập thiết bị ngừng gửi heartbeat.
- Giả lập người dùng không phản hồi cảnh báo.
- Giả lập gửi GPS trùng hoặc đến sai thứ tự.

Simulator chỉ sử dụng trong development/test, không được bật như dữ liệu thật trong production.

## 11. Out of Scope

Tạm thời không triển khai:

- React Native App.
- Android Native Services.
- iOS Native Background Location.
- Android SDK.
- Firebase Mobile SDK.
- Redis.
- Kafka.
- Kubernetes.
- AI/ML.
- Quảng cáo, donate, affiliate và thanh toán.

## 12. Migration Plan

Hãy kiểm tra những gì đã được triển khai từ prompt trước.

Sau đó:

1. Xác định phần nào được giữ nguyên.
2. Xác định phần nào cần thay đổi.
3. Không xóa hoặc viết lại backend/database khi không cần thiết.
4. Thay thế React Native frontend bằng React Web + PWA.
5. Điều chỉnh các phần GPS, notification và Map Rendering cho trình duyệt.
6. Cập nhật tài liệu Architecture, ERD và API Contracts.
7. Cập nhật development roadmap và testing plan.
8. Tiếp tục triển khai từ trạng thái hiện tại của dự án.

Nếu đã có mã nguồn, ưu tiên refactor tối thiểu thay vì khởi tạo lại toàn bộ dự án.

**Yêu cầu quan trọng nhất:** Thiết kế Web MVP có giới hạn tracking rõ ràng, đồng thời giữ khả năng chuyển sang React Native trong tương lai mà không phải viết lại các nghiệp vụ backend.
