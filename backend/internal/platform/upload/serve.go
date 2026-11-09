package upload

import (
	"mime"
	"net/http"
	"path"
	"strings"
)

// sandboxCSP neuters a stored object even if a browser renders it as a
// document: no script, no subresources, unique opaque origin.
const sandboxCSP = "default-src 'none'; style-src 'unsafe-inline'; sandbox"

// InlineSafeMIME reports whether a stored MIME type may render inline on the
// app origin. Stored files are served same-origin (BFF / storage stream), so
// HTML, SVG, XML, JavaScript and every unknown type are forced to download:
// rendering them inline would run uploader-controlled script as the viewer.
func InlineSafeMIME(ct string) bool {
	switch normalize(ct) {
	case "image/png", "image/jpeg", "image/gif", "image/webp", "image/avif", "image/bmp", "image/x-icon",
		"application/pdf",
		"model/gltf-binary", "model/vnd.usdz+zip":
		return true
	}
	ct = normalize(ct)
	return strings.HasPrefix(ct, "video/") || strings.HasPrefix(ct, "audio/")
}

// extTypes covers extensions mime.TypeByExtension may not know on a minimal image.
var extTypes = map[string]string{
	".png":  "image/png",
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".gif":  "image/gif",
	".webp": "image/webp",
	".avif": "image/avif",
	".ico":  "image/x-icon",
	".svg":  "image/svg+xml",
	".pdf":  "application/pdf",
	".glb":  "model/gltf-binary",
	".usdz": "model/vnd.usdz+zip",
	".mp4":  "video/mp4",
	".webm": "video/webm",
	".mp3":  "audio/mpeg",
	".zip":  "application/zip",
	".vcf":  "text/vcard",
	".csv":  "text/csv",
	".json": "application/json",
	".xlsx": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
}

// ContentTypeForKey derives a Content-Type from an object key's extension;
// unknown extensions are application/octet-stream.
func ContentTypeForKey(key string) string {
	ext := strings.ToLower(path.Ext(key))
	if ct, ok := extTypes[ext]; ok {
		return ct
	}
	if ct := mime.TypeByExtension(ext); ct != "" {
		return ct
	}
	return "application/octet-stream"
}

// SetServeHeaders sets the headers for streaming a stored object: the type,
// nosniff always, and inline only for InlineSafeMIME types (attachment plus a
// sandbox CSP otherwise, e.g. SVG/HTML). download forces attachment.
func SetServeHeaders(w http.ResponseWriter, contentType, filename string, download bool) {
	if strings.TrimSpace(contentType) == "" {
		contentType = "application/octet-stream"
	}
	h := w.Header()
	h.Set("Content-Type", contentType)
	h.Set("X-Content-Type-Options", "nosniff")
	disp := "inline"
	if download || !InlineSafeMIME(contentType) {
		disp = "attachment"
	}
	if !InlineSafeMIME(contentType) {
		h.Set("Content-Security-Policy", sandboxCSP)
	}
	name := strings.Map(func(r rune) rune {
		if r == '"' || r == '\\' || r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, filename)
	if name == "" {
		h.Set("Content-Disposition", disp)
		return
	}
	h.Set("Content-Disposition", disp+`; filename="`+name+`"`)
}
