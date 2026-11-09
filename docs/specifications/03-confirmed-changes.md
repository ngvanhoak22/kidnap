# Các bổ sung đã được người dùng xác nhận

Ngày chốt: 2026-11-06 (Asia/Ho_Chi_Minh).

Thứ tự ưu tiên đặc tả: các quyết định trong file này, sau đó 02-web-pwa.md, sau đó 01-original.md. Nội dung không bị thay thế giữ nguyên.

## Geocoding — Phase 2

- Giữ ORS làm Routing Provider, không chuyển OSRM.
- Chỉ dùng HeiGIT Pelias Search, Autocomplete và Reverse Geocoding; base `https://api.heigit.org/pelias/v1`.
- Backend Go/Gin giữ API key; tạo GeocodingProvider interface.
- Tìm điểm đến theo tên/địa chỉ, autocomplete debounce 600 ms, ít nhất 3 ký tự, tối đa 5 gợi ý.
- Chọn kết quả lấy longitude/latitude rồi gọi ORS Directions qua backend; MapLibre GL JS + OpenFreeMap hiển thị.
- Tiếp tục cho tap bản đồ khi không tìm thấy; reverse thất bại không chặn chọn tọa độ.
- Xử lý timeout/rate limit/no results/network; kiểm thử tiếng Việt và địa điểm phổ biến Việt Nam.
- **Phạm vi tìm kiếm chỉ Việt Nam**; không thêm provider khác.

## Chia sẻ — Phase 4–5

- Người thân chỉ được xem khi người dùng **chủ động chia sẻ** hoặc **có cảnh báo** mới được xem.
- Contact được xác nhận và danh sách người nhận cảnh báo không tự cấp quyền xem mọi hành trình.
- Mọi grant vẫn giới hạn từng chuyến, thời gian, recipient đã đồng ý; có thể thu hồi; không có public tracking link.

## Lệnh bắt đầu

Người dùng yêu cầu bắt đầu **Phase 1**. Chưa triển khai Phase 2–5 trong đợt này. Phase 1 gồm cấu trúc Go/Gin, Docker Compose, PostgreSQL 17/PostGIS/TimescaleDB, migrations, authentication và trusted contacts.
