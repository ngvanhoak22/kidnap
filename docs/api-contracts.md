# API contracts — v1

Phase 1 endpoints dưới đây sẽ được triển khai và kiểm thử. Các phase sau bổ sung riêng; không có API định tuyến/GPS hoạt động ở Phase 1.

## Quy ước

JSON UTF-8; timestamps RFC3339 UTC; UUID cho ID. Thành công `{ "data": ... }`; lỗi `{ "error": { "code": "...", "message": "..." }, "request_id": "..." }`. Không trả SQL/stack trace. `X-Request-ID` do server tạo. Dữ liệu API `Cache-Control: no-store`. Với endpoint nhận payload: tối đa 32 KiB, chỉ một JSON object, reject unknown fields. Các lệnh không nhận payload không dùng body. List tối đa 100 item; session list tối đa 100 phiên mới nhất.

Cookie `journey_session` ở development, `__Host-journey_session` ở production; HttpOnly, Path=/, SameSite=Lax. Dùng `credentials: 'include'`. Với POST/PATCH/DELETE đã đăng nhập, gửi `X-CSRF-Token` lấy từ login/register hoặc GET `/auth/session`. Không lưu session trong localStorage. Mọi API nhạy cảm cần session hợp lệ. Origin phải thuộc allowlist nếu được gửi; preflight ngoài allowlist bị từ chối. HTTPS bắt buộc production, cùng site cho web/API (khuyến nghị reverse proxy cùng origin).

## Auth / profile

| Method | Path `/api/v1` | Request | Response |
|---|---|---|---|
| POST | /auth/register | `{email,password,display_name,device_name?}` | 201 `{user,csrf_token,expires_at}` + cookie |
| POST | /auth/login | `{email,password,device_name?}` | 200 như register |
| GET | /auth/session | — | 200 `{user,csrf_token,expires_at}` |
| POST | /auth/logout | `{}` | 204, revoke current + clear cookie |
| GET | /auth/sessions | — | 200 `[{id,device_name,created_at,last_seen_at,expires_at,current}]` |
| DELETE | /auth/sessions/:id | — | 204; chỉ session thuộc current user; xóa cookie nếu là current |
| DELETE | /auth/sessions | — | 204; revoke mọi session và clear cookie |
| GET | /users/me | — | 200 `{id,email,display_name,created_at}` |
| PATCH | /users/me | `{display_name}` | 200 user updated |

Email trim/lowercase, định dạng đơn giản không display name, <=254 bytes. Password 12–128 code points và <=1024 bytes, không trim; display_name trim 1–80 ký tự, device_name <=80. Profile chưa cho đổi email/mật khẩu trong Phase 1. Không có password reset/verification email provider ở Phase 1.

## Trusted contacts

| Method | Path `/api/v1` | Request | Response |
|---|---|---|---|
| POST | /trusted-contacts/invitations | `{email}` | 201 `{invitation,invitation_token}`; token chỉ trả lần này |
| GET | /trusted-contacts/invitations | — | 200 `{incoming:[],outgoing:[]}` (tối đa 100 mỗi phía) |
| POST | /trusted-contacts/invitations/:id/accept | `{token}` | 200 contact (email session phải đúng recipient) |
| POST | /trusted-contacts/invitations/:id/decline | `{}` | 204; chỉ recipient |
| DELETE | /trusted-contacts/invitations/:id | — | 204; chỉ owner, pending |
| GET | /trusted-contacts | — | 200 active contacts thuộc owner |
| GET | /trusted-contacts/trusted-by | — | 200 các quan hệ owner khác đã mời current user |
| PATCH | /trusted-contacts/:id | `{label}` | 200 contact; chỉ owner được sửa tên gợi nhớ 0–80 ký tự |
| DELETE | /trusted-contacts/:id | — | 204; owner hoặc recipient thu hồi |

Invitation public: `{id,owner_id,owner_name,invitee_email,status,created_at,expires_at}`. Contact: `{id,owner_id,contact_user_id,display_name,label,invitation_id,confirmed_at}`; display_name là tên phía còn lại; label là tên gợi nhớ owner đã đặt, hiển thị với hai bên trong quan hệ. Không cho sửa recipient/email của quan hệ đã chấp nhận; cần revoke và mời lại. Không trả password/hash/token, không trả thông tin chuyến đi. Pending chỉ có hiệu lực trước expires_at; danh sách phản ánh EXPIRED theo giờ DB ngay cả trước cleanup. Giới hạn 3 = active + pending chưa hết hạn của owner, minimum 0; không bắt buộc có contact ngay lúc đăng ký.

Invitation ID + token được owner gửi qua kênh riêng; recipient login rồi accept. Không có email/SMS delivery trong phase này. Token không phải public tracking link và không cấp quyền chuyến đi. Acceptance cần token và đúng email; accept lặp chỉ idempotent khi contact vẫn còn hiệu lực và cùng invitation.

## Errors / health

400 `INVALID_REQUEST`; 401 `UNAUTHENTICATED`/`INVALID_CREDENTIALS`; 403 `ORIGIN_NOT_ALLOWED`/`CSRF_INVALID`; 404 `NOT_FOUND`; 409 `EMAIL_EXISTS`/`CONTACT_EXISTS`/`CONTACT_LIMIT`/`INVITATION_EXISTS`/`INVITATION_INACTIVE`; 410 `INVITATION_EXPIRED`; 413 `PAYLOAD_TOO_LARGE`; 415 `UNSUPPORTED_MEDIA_TYPE`; 429 `RATE_LIMITED`; 500 `INTERNAL_ERROR`; 503 `NOT_READY`/`BUSY`.

GET `/health/live`: process 200. GET `/health/ready`: DB ping + required extension tables + applied foundation schema, 200 hoặc 503; không expose connection string.

## Contract dự kiến Phase 2 (chưa có handler)

- GET `/api/v1/geocoding/search?q=...` và `/autocomplete?q=...`: auth, 3+ ký tự, max 5 kết quả, country=VNM ở backend, focus tùy GPS được đồng ý. Adapter chỉ nhận các query đã allowlist, không proxy URL tùy ý.
- GET `/api/v1/geocoding/reverse?longitude=...&latitude=...`: validate ranges, country=VNM; không có nhãn vẫn cho dùng tọa độ tap.
- POST `/api/v1/routes/preview`: `{start:[lon,lat],end:[lon,lat]}`, backend gọi ORS. Empty geocode trả `data:[]`, không coi là server error. Timeout/rate-limit/network có error code riêng, không expose upstream credential/query URL.
