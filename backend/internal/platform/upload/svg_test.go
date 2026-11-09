package upload_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/upload"
)

func TestValidateSVGAcceptsStaticVectorContent(t *testing.T) {
	clean := []string{
		`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10"><path d="M0 0h10v10z" fill="#000"/></svg>`,
		`<?xml version="1.0" encoding="UTF-8"?>
<!-- Generator: test -->
<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" width="10" height="10">
  <title>Logo</title>
  <defs>
    <linearGradient id="g"><stop offset="0" stop-color="#fff"/><stop offset="1" style="stop-color:#000"/></linearGradient>
    <clipPath id="c"><rect width="5" height="5"/></clipPath>
    <style>.a{fill:url(#g)} .b { stroke: red; }</style>
    <filter id="f"><feGaussianBlur stdDeviation="1"/></filter>
  </defs>
  <g class="a" clip-path="url(#c)" filter="url(#f)"><circle cx="5" cy="5" r="4" fill="url('#g')"/></g>
  <use xlink:href="#c"/><use href="#c"/>
  <text x="1" y="9" font-family="Arial">ACME &amp; Co</text>
  <image width="1" height="1" href="data:image/png;base64,iVBORw0KGgo="/>
</svg>`,
		// Inkscape export with editor metadata namespaces.
		`<svg xmlns="http://www.w3.org/2000/svg" xmlns:inkscape="http://www.inkscape.org/namespaces/inkscape" xmlns:sodipodi="http://sodipodi.sourceforge.net/DTD/sodipodi-0.dtd" xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#" xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:cc="http://creativecommons.org/ns#" inkscape:version="1.3">
  <sodipodi:namedview id="nv" pagecolor="#ffffff"/>
  <metadata><rdf:RDF><cc:Work rdf:about=""><dc:format>image/svg+xml</dc:format></cc:Work></rdf:RDF></metadata>
  <g inkscape:label="Layer 1" inkscape:groupmode="layer"><rect width="1" height="1"/></g>
</svg>`,
	}
	for i, s := range clean {
		if err := upload.ValidateSVG([]byte(s)); err != nil {
			t.Errorf("clean svg %d rejected: %v", i, err)
		}
		sn, err := upload.SniffLogo(strings.NewReader(s))
		if err != nil || sn.ContentType != "image/svg+xml" || sn.Ext != "svg" {
			t.Errorf("clean svg %d via SniffLogo: %+v %v", i, sn, err)
		}
	}
}

func TestValidateSVGRejectsActiveContent(t *testing.T) {
	const ns = `xmlns="http://www.w3.org/2000/svg"`
	const xl = `xmlns:xlink="http://www.w3.org/1999/xlink"`
	bad := map[string]string{
		"script":                 `<svg ` + ns + `><script>alert(1)</script></svg>`,
		"script cdata":           `<svg ` + ns + `><script><![CDATA[alert(1)]]></script></svg>`,
		"onload":                 `<svg ` + ns + ` onload="alert(1)"/>`,
		"onclick upper":          `<svg ` + ns + `><rect ONCLICK="alert(1)"/></svg>`,
		"javascript href":        `<svg ` + ns + ` ` + xl + `><a xlink:href="javascript:alert(1)"><rect/></a></svg>`,
		"javascript use":         `<svg ` + ns + `><use href="javascript:alert(1)"/></svg>`,
		"entity obfuscated js":   `<svg ` + ns + `><use href="&#106;ava&#x09;script:alert(1)"/></svg>`,
		"case obfuscated js":     `<svg ` + ns + `><image href=" JaVaScRiPt:alert(1)"/></svg>`,
		"js in other attribute":  `<svg ` + ns + `><rect fill="javascript:alert(1)"/></svg>`,
		"vbscript":               `<svg ` + ns + `><use href="vbscript:msgbox(1)"/></svg>`,
		"data html":              `<svg ` + ns + `><image href="data:text/html,&lt;script&gt;alert(1)&lt;/script&gt;"/></svg>`,
		"data svg image":         `<svg ` + ns + `><image href="data:image/svg+xml;base64,PHN2Zz4="/></svg>`,
		"foreignObject":          `<svg ` + ns + `><foreignObject><div xmlns="http://www.w3.org/1999/xhtml">x</div></foreignObject></svg>`,
		"xhtml script namespace": `<svg ` + ns + ` xmlns:h="http://www.w3.org/1999/xhtml"><h:script>alert(1)</h:script></svg>`,
		"iframe":                 `<svg ` + ns + `><iframe src="https://evil.example"/></svg>`,
		"embed":                  `<svg ` + ns + `><embed src="x"/></svg>`,
		"object":                 `<svg ` + ns + `><object data="x"/></svg>`,
		"audio":                  `<svg ` + ns + `><audio src="x"/></svg>`,
		"video":                  `<svg ` + ns + `><video src="x"/></svg>`,
		"animate href":           `<svg ` + ns + `><a><animate attributeName="href" to="javascript:alert(1)"/></a></svg>`,
		"set onload":             `<svg ` + ns + `><set attributeName="onload" to="alert(1)"/></svg>`,
		"external href":          `<svg ` + ns + `><image href="https://evil.example/x.png"/></svg>`,
		"protocol-relative href": `<svg ` + ns + ` ` + xl + `><use xlink:href="//evil.example/s.svg#x"/></svg>`,
		"external use file":      `<svg ` + ns + `><use href="sprite.svg#icon"/></svg>`,
		"doctype":                `<!DOCTYPE svg PUBLIC "-//W3C//DTD SVG 1.1//EN" "http://www.w3.org/Graphics/SVG/1.1/DTD/svg11.dtd"><svg ` + ns + `/>`,
		"entity xxe":             `<!DOCTYPE svg [<!ENTITY xxe SYSTEM "file:///etc/passwd">]><svg ` + ns + `><text>&xxe;</text></svg>`,
		"billion laughs":         `<!DOCTYPE svg [<!ENTITY a "aaaa"><!ENTITY b "&a;&a;&a;">]><svg ` + ns + `><text>&b;</text></svg>`,
		"style url http":         `<svg ` + ns + `><style>rect{fill:url(http://evil.example/x)}</style></svg>`,
		"style url quoted ext":   `<svg ` + ns + `><style>rect{fill:url( "https://evil.example/x" )}</style></svg>`,
		"style attr url":         `<svg ` + ns + `><rect style="fill:url(//evil.example/x)"/></svg>`,
		"style import":           `<svg ` + ns + `><style>@import "https://evil.example/x.css";</style></svg>`,
		"style expression":       `<svg ` + ns + `><rect style="width:expression(alert(1))"/></svg>`,
		"style css escape":       `<svg ` + ns + `><style>rect{fill:\75 rl(http://evil.example)}</style></svg>`,
		"attr url external":      `<svg ` + ns + `><rect fill="url(https://evil.example/#g)"/></svg>`,
		"stylesheet PI":          `<?xml-stylesheet href="https://evil.example/x.css"?><svg ` + ns + `/>`,
		"root not svg":           `<html ` + ns + `><svg/></html>`,
		"html with svg inside":   `<html><body><svg ` + ns + `/></body></html>`,
		"unknown element":        `<svg ` + ns + `><handler/></svg>`,
		"link element":           `<svg ` + ns + `><a href="#x"><rect/></a></svg>`,
		"non-utf8 encoding":      `<?xml version="1.0" encoding="ISO-8859-1"?><svg ` + ns + `/>`,
		"malformed":              `<svg ` + ns + `><rect></svg>`,
		"unknown entity":         `<svg ` + ns + `><text>&nbsp;</text></svg>`,
		"feImage external":       `<svg ` + ns + `><filter><feImage href="https://evil.example/x.png"/></filter></svg>`,
	}
	for name, s := range bad {
		err := upload.ValidateSVG([]byte(s))
		if !errors.Is(err, upload.ErrSVGUnsafe) {
			t.Errorf("%s: expected ErrSVGUnsafe, got %v", name, err)
		}
	}
}

func TestValidateSVGSizeCap(t *testing.T) {
	big := `<svg xmlns="http://www.w3.org/2000/svg"><desc>` + strings.Repeat("a", upload.MaxSVGBytes) + `</desc></svg>`
	if err := upload.ValidateSVG([]byte(big)); !errors.Is(err, upload.ErrTooLarge) {
		t.Fatalf("oversized svg: %v", err)
	}
	if _, err := upload.SniffLogo(strings.NewReader(big)); !errors.Is(err, upload.ErrTooLarge) {
		t.Fatalf("oversized svg via SniffLogo: %v", err)
	}
}

func TestWriteErrorMapsUnsafeSVGTo422(t *testing.T) {
	err := upload.ValidateSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg"><script>x</script></svg>`))
	rec := httptest.NewRecorder()
	if !upload.WriteError(rec, httptest.NewRequest(http.MethodPost, "/", nil), err) {
		t.Fatal("unsafe svg not handled")
	}
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status %d", rec.Code)
	}
	var env struct {
		Error struct {
			Code    string `json:"code"`
			Details []struct {
				Message string `json:"message"`
			} `json:"details"`
		} `json:"error"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &env)
	if env.Error.Code != "SVG_UNSAFE_CONTENT" || len(env.Error.Details) == 0 || !strings.Contains(env.Error.Details[0].Message, "script") {
		t.Fatalf("envelope %s", rec.Body.String())
	}
}
