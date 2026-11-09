package upload

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// MaxSVGBytes caps an SVG logo upload.
const MaxSVGBytes = 512 << 10

// ErrSVGUnsafe reports an SVG that carries anything beyond static vector
// content (script, event handlers, external references, DTDs, ...). Handlers
// map it to 422 SVG_UNSAFE_CONTENT; the upload is rejected, never stripped.
var ErrSVGUnsafe = errors.New("svg contains unsafe content")

// SVGUnsafeError carries the reason an SVG was rejected.
type SVGUnsafeError struct{ Reason string }

func (e *SVGUnsafeError) Error() string { return "unsafe SVG content: " + e.Reason }

// Unwrap lets errors.Is(err, ErrSVGUnsafe) match.
func (e *SVGUnsafeError) Unwrap() error { return ErrSVGUnsafe }

func unsafeSVG(format string, args ...any) error {
	return &SVGUnsafeError{Reason: fmt.Sprintf(format, args...)}
}

const svgNS = "http://www.w3.org/2000/svg"

// editorNamespaces are metadata namespaces design tools (Inkscape, Illustrator)
// write into exported SVGs. Browsers do not render elements in them; their
// attributes are still checked like any other.
var editorNamespaces = map[string]bool{
	"http://www.inkscape.org/namespaces/inkscape":            true,
	"http://sodipodi.sourceforge.net/DTD/sodipodi-0.dtd":     true,
	"http://www.w3.org/1999/02/22-rdf-syntax-ns#":            true,
	"http://purl.org/dc/elements/1.1/":                       true,
	"http://creativecommons.org/ns#":                         true,
	"http://web.resource.org/cc/":                            true,
	"http://ns.adobe.com/AdobeIllustrator/10.0/":             true,
	"http://ns.adobe.com/AdobeSVGViewerExtensions/3.0/":      true,
	"http://ns.adobe.com/Extensibility/1.0/":                 true,
	"http://ns.adobe.com/Graphs/1.0/":                        true,
	"http://ns.adobe.com/SaveForWeb/1.0/":                    true,
	"http://ns.adobe.com/Variables/1.0/":                     true,
	"http://www.bohemiancoding.com/sketch/ns":                true,
	"http://www.serif.com/":                                  true,
	"http://ns.adobe.com/xap/1.0/":                           true,
	"http://ns.adobe.com/xap/1.0/mm/":                        true,
	"http://ns.adobe.com/xap/1.0/sType/ResourceEvent#":       true,
	"http://ns.adobe.com/xap/1.0/sType/ResourceRef#":         true,
	"http://ns.adobe.com/pdf/1.3/":                           true,
	"http://ns.adobe.com/photoshop/1.0/":                     true,
	"http://ns.adobe.com/tiff/1.0/":                          true,
	"http://ns.adobe.com/exif/1.0/":                          true,
	"http://iptc.org/std/Iptc4xmpCore/1.0/xmlns/":            true,
	"http://ns.adobe.com/xap/1.0/rights/":                    true,
	"http://www.w3.org/2000/01/rdf-schema#":                  true,
	"http://purl.org/dc/terms/":                              true,
	"http://ns.adobe.com/xap/1.0/sType/Dimensions#":          true,
	"http://ns.adobe.com/xap/1.0/g/":                         true,
	"http://ns.adobe.com/xap/1.0/g/img/":                     true,
	"http://ns.adobe.com/xap/1.0/sType/Font#":                true,
	"http://ns.adobe.com/xap/1.0/t/pg/":                      true,
	"http://ns.adobe.com/illustrator/1.0/":                   true,
	"http://ns.adobe.com/xap/1.0/bj/":                        true,
	"http://ns.adobe.com/xap/1.0/sType/ManifestItem#":        true,
	"http://ns.adobe.com/xap/1.0/sType/Version#":             true,
	"http://ns.adobe.com/xap/1.0/sType/Job#":                 true,
	"http://ns.adobe.com/xmp/1.0/DynamicMedia/":              true,
	"http://ns.adobe.com/xap/1.0/sType/ResourceRef":          true,
	"http://ns.adobe.com/xap/1.0/sType/ResourceEvent":        true,
	"http://ns.adobe.com/xap/1.0/sType/Area#":                true,
	"http://ns.adobe.com/xmp/sType/Area#":                    true,
	"http://ns.adobe.com/xmp/1.0/":                           true,
	"adobe:ns:meta/":                                         true,
	"http://www.figma.com/figma/ns":                          true,
	"http://www.w3.org/XML/1998/namespace":                   true,
	"https://boxy-svg.com":                                   true,
	"http://www.vectornator.io":                              true,
	"http://www.serif.com":                                   true,
	"http://schemas.microsoft.com/visio/2003/SVGExtensions/": true,
}

// svgElements is the allowlist of static SVG elements. Script, foreignObject,
// animation (animate/set/animateTransform/animateMotion), links (<a>),
// iframe/embed/object/audio/video and every unknown element are rejected.
var svgElements = map[string]bool{
	"svg": true, "g": true, "path": true, "rect": true, "circle": true, "ellipse": true,
	"line": true, "polyline": true, "polygon": true, "text": true, "tspan": true, "textPath": true,
	"defs": true, "linearGradient": true, "radialGradient": true, "stop": true,
	"clipPath": true, "mask": true, "use": true, "symbol": true, "title": true, "desc": true,
	"pattern": true, "image": true, "style": true, "metadata": true, "marker": true, "filter": true,
	"feBlend": true, "feColorMatrix": true, "feComponentTransfer": true, "feComposite": true,
	"feConvolveMatrix": true, "feDiffuseLighting": true, "feDisplacementMap": true,
	"feDistantLight": true, "feDropShadow": true, "feFlood": true, "feFuncA": true, "feFuncB": true,
	"feFuncG": true, "feFuncR": true, "feGaussianBlur": true, "feImage": true, "feMerge": true,
	"feMergeNode": true, "feMorphology": true, "feOffset": true, "fePointLight": true,
	"feSpecularLighting": true, "feSpotLight": true, "feTile": true, "feTurbulence": true,
}

// rasterDataURI is the only non-fragment reference allowed: an embedded raster image.
var rasterDataURI = regexp.MustCompile(`^data:image/(png|jpeg|jpg|gif|webp)(;[a-z0-9=._-]+)*,`)

// cssURL finds url( ... ) targets in attribute values and style sheets.
var cssURL = regexp.MustCompile(`url\(([^)]*)\)`)

// compact lowercases s and drops whitespace/control characters, so that
// "java\tscript:" or "JaVa Script:" (after XML entity decoding) is caught.
func compact(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if unicode.IsSpace(r) || unicode.IsControl(r) || r == 0x00ad || r == 0x200b || r == 0xfeff {
			continue
		}
		b.WriteRune(unicode.ToLower(r))
	}
	return b.String()
}

// safeRef reports whether a reference target is a same-document fragment or
// an embedded raster data URI.
func safeRef(v string) bool {
	c := compact(v)
	c = strings.Trim(c, `"'`)
	return c == "" || strings.HasPrefix(c, "#") || rasterDataURI.MatchString(c)
}

// checkValue rejects active schemes and external url() targets in any
// attribute value or style sheet text.
func checkValue(v string) error {
	c := compact(v)
	for _, bad := range []string{"javascript:", "vbscript:", "livescript:", "data:text/html", "data:image/svg", "data:application", "data:text/xml"} {
		if strings.Contains(c, bad) {
			return unsafeSVG("%q is not allowed", bad)
		}
	}
	for _, m := range cssURL.FindAllStringSubmatch(c, -1) {
		if !safeRef(m[1]) {
			return unsafeSVG("external url(%s) reference is not allowed", m[1])
		}
	}
	return nil
}

// checkCSS checks <style> text and style attributes.
func checkCSS(v string) error {
	c := compact(v)
	if strings.Contains(v, `\`) {
		return unsafeSVG("CSS escapes are not allowed")
	}
	for _, bad := range []string{"@import", "expression(", "behavior:", "-moz-binding", "url(http", "url(//"} {
		if strings.Contains(c, bad) {
			return unsafeSVG("%q is not allowed in styles", bad)
		}
	}
	return checkValue(v)
}

// LooksLikeSVG reports whether bytes look like an SVG/XML document.
func LooksLikeSVG(head []byte) bool {
	h := bytes.TrimLeft(bytes.TrimPrefix(head, []byte("\xef\xbb\xbf")), " \t\r\n")
	if len(h) == 0 || h[0] != '<' {
		return false
	}
	return bytes.Contains(bytes.ToLower(head), []byte("<svg"))
}

// ValidateSVG accepts only static vector SVG content. It parses the document
// with a strict XML tokenizer (no DTDs, no custom entities, so no XXE or
// entity expansion) and walks every element against an allowlist. Rejections
// return an *SVGUnsafeError (errors.Is ErrSVGUnsafe).
func ValidateSVG(data []byte) error {
	if len(data) > MaxSVGBytes {
		return ErrTooLarge
	}
	if !utf8.Valid(data) {
		return unsafeSVG("SVG must be UTF-8 encoded")
	}
	dec := xml.NewDecoder(bytes.NewReader(data))
	dec.Strict = true
	dec.Entity = nil
	// Only UTF-8 is accepted; any other declared encoding fails here.
	dec.CharsetReader = func(label string, input io.Reader) (io.Reader, error) {
		if strings.EqualFold(label, "utf-8") || strings.EqualFold(label, "utf8") {
			return input, nil
		}
		return nil, unsafeSVG("encoding %q is not allowed", label)
	}

	var (
		stack  []xml.Name
		sawSVG bool
	)
	for {
		tok, err := dec.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			var se *SVGUnsafeError
			if errors.As(err, &se) {
				return se
			}
			return unsafeSVG("not a well-formed SVG document")
		}
		switch t := tok.(type) {
		case xml.Directive:
			return unsafeSVG("DOCTYPE/ENTITY declarations are not allowed")
		case xml.ProcInst:
			if t.Target != "xml" {
				return unsafeSVG("processing instruction <?%s?> is not allowed", t.Target)
			}
		case xml.StartElement:
			if len(stack) == 0 {
				if sawSVG {
					return unsafeSVG("only one root <svg> element is allowed")
				}
				if t.Name.Local != "svg" || (t.Name.Space != svgNS && t.Name.Space != "") {
					return unsafeSVG("root element must be <svg>")
				}
				sawSVG = true
			}
			if err := checkElement(t); err != nil {
				return err
			}
			stack = append(stack, t.Name)
		case xml.EndElement:
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
		case xml.CharData:
			if len(stack) > 0 && stack[len(stack)-1].Local == "style" {
				if err := checkCSS(string(t)); err != nil {
					return err
				}
			}
		}
	}
	if !sawSVG {
		return unsafeSVG("root element must be <svg>")
	}
	return nil
}

func checkElement(t xml.StartElement) error {
	name := t.Name.Local
	switch {
	case t.Name.Space == svgNS || t.Name.Space == "":
		if !svgElements[name] {
			return unsafeSVG("<%s> element is not allowed", name)
		}
	case editorNamespaces[t.Name.Space]:
		// Editor metadata: not rendered; attributes still checked below.
	default:
		return unsafeSVG("<%s> in namespace %q is not allowed", name, t.Name.Space)
	}
	for _, a := range t.Attr {
		local := strings.ToLower(a.Name.Local)
		// Namespace declarations carry URIs, not references.
		if a.Name.Space == "xmlns" || (a.Name.Space == "" && local == "xmlns") {
			continue
		}
		if strings.HasPrefix(local, "on") {
			return unsafeSVG("event handler attribute %q is not allowed", a.Name.Local)
		}
		if local == "href" || local == "src" {
			if !safeRef(a.Value) {
				return unsafeSVG("external reference %q is not allowed", a.Value)
			}
			if name == "image" || name == "feImage" {
				if c := compact(a.Value); !strings.HasPrefix(c, "#") && !rasterDataURI.MatchString(c) {
					return unsafeSVG("<%s> must embed a PNG/JPEG/GIF/WebP data URI", name)
				}
			}
			continue
		}
		if local == "style" {
			if err := checkCSS(a.Value); err != nil {
				return err
			}
			continue
		}
		if err := checkValue(a.Value); err != nil {
			return err
		}
	}
	return nil
}
