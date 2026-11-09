package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Error struct {
	Status  int
	Code    string
	Message string
}

func (e *Error) Error() string                         { return e.Code }
func NewError(status int, code, message string) *Error { return &Error{status, code, message} }
func Invalid(message string) *Error                    { return NewError(400, "INVALID_REQUEST", message) }
func NotFound() *Error                                 { return NewError(404, "NOT_FOUND", "Không tìm thấy tài nguyên.") }
func Unauthorized() *Error                             { return NewError(401, "UNAUTHENTICATED", "Vui lòng đăng nhập lại.") }
func Data(c *gin.Context, status int, value any)       { c.JSON(status, gin.H{"data": value}) }
func Fail(c *gin.Context, err error) {
	var e *Error
	if !errors.As(err, &e) {
		e = NewError(500, "INTERNAL_ERROR", "Không thể xử lý yêu cầu lúc này.")
	}
	c.AbortWithStatusJSON(e.Status, gin.H{"error": gin.H{"code": e.Code, "message": e.Message}, "request_id": c.GetString("request_id")})
}

func Bind(c *gin.Context, dst any) bool {
	media, _, err := mime.ParseMediaType(c.GetHeader("Content-Type"))
	if err != nil || media != "application/json" {
		Fail(c, NewError(415, "UNSUPPORTED_MEDIA_TYPE", "Yêu cầu Content-Type application/json."))
		return false
	}
	d := json.NewDecoder(c.Request.Body)
	d.DisallowUnknownFields()
	if err := d.Decode(dst); err != nil {
		failJSON(c, err)
		return false
	}
	var extra any
	if err := d.Decode(&extra); err != io.EOF {
		failJSON(c, err)
		return false
	}
	return true
}
func failJSON(c *gin.Context, err error) {
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		Fail(c, NewError(413, "PAYLOAD_TOO_LARGE", "Dữ liệu vượt giới hạn cho phép."))
		return
	}
	Fail(c, Invalid("JSON không hợp lệ hoặc chứa trường không được hỗ trợ."))
}
func ID(c *gin.Context) (string, bool) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		Fail(c, Invalid("ID phải là UUID hợp lệ."))
		return "", false
	}
	return id.String(), true
}
