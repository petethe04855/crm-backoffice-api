package route

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

// onePixelPNG is a real 1x1 transparent PNG (valid magic bytes and body).
const onePixelPNG = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg=="

func pngBytes(t *testing.T) []byte {
	t.Helper()
	data, err := base64.StdEncoding.DecodeString(onePixelPNG)
	require.NoError(t, err)
	return data
}

// jpegBytes is a JPEG header plus filler: the server only reads magic bytes,
// so this is enough to prove the type is detected from content.
func jpegBytes() []byte {
	return append([]byte{0xFF, 0xD8, 0xFF, 0xE0}, bytes.Repeat([]byte{0x00}, 16)...)
}

func webpBytes() []byte {
	return append([]byte("RIFF\x24\x00\x00\x00WEBP"), bytes.Repeat([]byte{0x00}, 8)...)
}

// doUpload posts a multipart form to the upload route. An empty field name
// sends a part under "wrongfield" so the handler sees no "file".
func (e *testEnv) doUpload(t *testing.T, bearer, field, filename string, data []byte) apiResult {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	if field != "" {
		fw, err := w.CreateFormFile(field, filename)
		require.NoError(t, err)
		_, err = fw.Write(data)
		require.NoError(t, err)
	}
	require.NoError(t, w.Close())

	req := httptest.NewRequest(http.MethodPost, "/api/v1/crm/uploads/images", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	resp, err := e.app.Test(req, -1)
	require.NoError(t, err)
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	res := apiResult{Status: resp.StatusCode, Header: resp.Header, Raw: raw}
	require.NoError(t, json.Unmarshal(raw, &res.Body))
	return res
}

// uploadAs posts a valid image and returns the created id and URL.
func (e *testEnv) uploadAs(t *testing.T, bearer string, data []byte) (id, url string) {
	t.Helper()
	res := e.doUpload(t, bearer, "file", "image.png", data)
	require.Equal(t, http.StatusCreated, res.Status)
	require.True(t, res.Body.Success)

	var body struct {
		ID  string `json:"id"`
		URL string `json:"url"`
	}
	require.NoError(t, json.Unmarshal(res.Body.Data, &body))
	require.NotEmpty(t, body.ID)
	require.Equal(t, "/api/v1/files/"+body.ID, body.URL)
	return body.ID, body.URL
}

func TestImageUploadRBAC(t *testing.T) {
	e := newEnv(t)
	png := pngBytes(t)

	res := e.doUpload(t, "", "file", "image.png", png)
	require.Equal(t, http.StatusUnauthorized, res.Status)

	// sales holds promotions.view only; accountant holds neither manage perm.
	require.Equal(t, http.StatusForbidden, e.doUpload(t, token(t, 2, "sales"), "file", "image.png", png).Status)
	require.Equal(t, http.StatusForbidden, e.doUpload(t, token(t, 6, "accountant"), "file", "image.png", png).Status)

	// The owner holds promotions.manage.
	e.uploadAs(t, token(t, 1, "owner"), png)

	// A CRM admin also holds it, even when the ERP role is sales.
	res = e.do(t, http.MethodPut, "/api/v1/crm/staff-roles/2", token(t, 1, "owner"),
		map[string]string{"email": "sales@example.com", "role": "admin"})
	require.Equal(t, http.StatusOK, res.Status)
	e.uploadAs(t, token(t, 2, "sales"), png)
}

func TestImageUploadValidatesContentAndSize(t *testing.T) {
	e := newEnv(t)
	owner := token(t, 1, "owner")

	// A text file named .png is still rejected: the type comes from the bytes.
	res := e.doUpload(t, owner, "file", "image.png", []byte("not really a png"))
	require.Equal(t, http.StatusBadRequest, res.Status)
	require.Equal(t, "UNSUPPORTED_MEDIA_TYPE", res.Body.Error.Code)

	res = e.doUpload(t, owner, "file", "image.png", nil)
	require.Equal(t, http.StatusBadRequest, res.Status)

	res = e.doUpload(t, owner, "wrongfield", "image.png", pngBytes(t))
	require.Equal(t, http.StatusBadRequest, res.Status)

	tooBig := make([]byte, 2<<20+1)
	copy(tooBig, pngBytes(t))
	res = e.doUpload(t, owner, "file", "big.png", tooBig)
	require.Equal(t, http.StatusRequestEntityTooLarge, res.Status)
	require.Equal(t, "FILE_TOO_LARGE", res.Body.Error.Code)
}

func TestImageUploadDetectsTypeFromMagicBytes(t *testing.T) {
	e := newEnv(t)
	owner := token(t, 1, "owner")

	for _, c := range []struct {
		name string
		data []byte
		mime string
	}{
		{"photo.jpg", jpegBytes(), "image/jpeg"},
		{"banner.webp", webpBytes(), "image/webp"},
		{"image.png", pngBytes(t), "image/png"},
	} {
		_, url := e.uploadAs(t, owner, c.data)
		res := e.do(t, http.MethodGet, url, "", nil)
		require.Equal(t, http.StatusOK, res.Status, c.name)
		require.Equal(t, c.mime, res.Header.Get("Content-Type"), c.name)
		require.Equal(t, c.data, res.Raw, "the stored bytes come back unchanged")
		require.Contains(t, res.Header.Get("Cache-Control"), "immutable")
		require.Equal(t, "nosniff", res.Header.Get("X-Content-Type-Options"))
	}
}

func TestServeImageNotFound(t *testing.T) {
	e := newEnv(t)
	for _, id := range []string{"does-not-exist", "..%2f..%2fetc%2fpasswd"} {
		res := e.do(t, http.MethodGet, "/api/v1/files/"+id, "", nil)
		require.Equal(t, http.StatusNotFound, res.Status, id)
	}
}

func TestUploadedKeyIsAPlainUUID(t *testing.T) {
	// The handler names files with a fresh uuid, so a client-supplied name is
	// never used and the stored key can never contain a path separator.
	e := newEnv(t)
	id, _ := e.uploadAs(t, token(t, 1, "owner"), pngBytes(t))
	require.Len(t, id, 36)
	require.NotContains(t, id, "/")
	require.NotContains(t, id, "\\")
	require.NotContains(t, id, ".")
}
