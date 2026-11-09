# Roadmap và tiêu chí nghiệm thu

1. **Foundation (đã hoàn tất local):** Go/Gin module layers, PostgreSQL 17 + PostGIS + TimescaleDB trong Docker, migrations, cookie auth/CSRF/profile/session revocation, trusted contact invitation/accept/decline/cancel/revoke và sửa tên gợi nhớ, API contracts, integration tests với DB thật, race detector, restart persistence và backup/restore. Chưa có UI/PWA hay gửi thông báo.
2. **Route Planning & Trips:** React/Vite/PWA, MapLibre GL JS/OpenFreeMap, ORS và GeocodingProvider (HeiGIT Pelias only, Vietnam only), autocomplete 600 ms/3 ký tự/5 kết quả, search/reverse/tap fallback, route preview và trip lifecycle. Không yêu cầu Android SDK/Xcode.
3. **GPS:** watchPosition, heartbeat, IndexedDB queue, hypertable/dedup/out-of-order, PostGIS corridor, tracking states, development-only GPS simulator.
4. **Safety:** confirmation/SOS/heartbeat deadline, alert machine, PostgreSQL outbox worker, Web Push VAPID, SMS interface/mock; lỗi notification không làm mất incident.
5. **Sharing & validation:** explicit manual/alert grants, REST/WS authorization và revoke, playback/retention/xóa dữ liệu, device/browser tests, backup/restore và measured load test.

Simulator phải có normal/deviation/low-accuracy/GPS-loss/offline/heartbeat-loss/no-response/duplicate/out-of-order; không nhận simulator mode như dữ liệu thật ở production.

Phase 1 nghiệm thu: migrations up/down/up trên DB mới; phiên hết hạn/thu hồi; thiếu/sai CSRF; không truy cập contact người khác; mã mời sai/email sai; expiry; accept/cancel cạnh tranh; tối đa 3 slots cả pending; revoke/reinvite; dữ liệu nhạy cảm không có trong log; config production fail closed; Compose khởi động API ready và restart giữ dữ liệu. Khả năng 100–1.000 users chưa được chứng nhận trước load test các phase sau.
