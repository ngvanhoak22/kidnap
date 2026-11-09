# Trạng thái nghiệp vụ

## Trip — Phase 2+

```
DRAFT -> ACTIVE -> COMPLETED
  |         |
  +---------+---> CANCELED
```

COMPLETE/CANCEL chấm dứt tracking và quyền xem; không mở lại bằng request lặp. Các lệnh kết thúc lặp cùng kết quả có thể idempotent. Route thay đổi cần owner xác nhận version mới; không tự cập nhật corridor khi xe đi lệch.

## Tracking độc lập — Phase 3+

`ACTIVE`, `INTERRUPTED`, `GPS_UNAVAILABLE`, `OFFLINE`; trạng thái hiển thị dựa trên visibility/GPS/heartbeat với timestamp riêng. Client visibility chỉ là hint, server dùng timestamp đã nhận; không tin client để bỏ deadline đã có. Khi quay lại: xác thực session, fetch trip hiện tại, xin GPS, reconcile queued samples; không tiếp tục chuyến đã kết thúc.

## Alert độc lập — Phase 4+

```
DEVIATION: detection -> AWAITING_CONFIRMATION -> RESOLVED_SAFE
                                  |        -> ESCALATED -> RESOLVED
SOS: ---------------------------------------> ESCALATED -> RESOLVED
CONTACT_LOSS (đã opt-in): ------------------> ESCALATED -> RESOLVED
```

Deadline do DB/server quản lý; các event có idempotency key. SAFE không tự phê duyệt tuyến mới và không làm mất cảnh báo khác. Realtime sample cần đủ mới + accuracy hợp lệ + nhiều mẫu/thời gian ngoài corridor. Giá trị thử nghiệm 150 m, 30–60 s lệch, 60–120 s xác nhận, đều cấu hình; không phải ngưỡng an toàn bảo đảm.

Push delivery không chứng minh đã đọc hoặc an toàn. Mất GPS/mất heartbeat không phải bằng chứng nguy hiểm. Nội dung contact-loss phải nói rõ “Mất liên lạc – vị trí cuối cùng” và timestamp.

## Invitation — Phase 1

`PENDING -> ACCEPTED | DECLINED | CANCELED | EXPIRED`.

Owner tạo/cancel, recipient đúng email + mã bí mật accept; recipient có thể decline. Hết hạn kiểm tra trực tiếp trong DB; không chờ worker. Accepted không cấp quyền GPS. Contact chỉ có hiệu lực khi revoked_at IS NULL; chấp nhận mã cũ sau thu hồi không khôi phục quan hệ.
