package usecase_test

import (
	"context"
	"testing"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/authctx"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/qr"
	"github.com/google/uuid"
)

// TEC-1269: a v2 design round-trips through routes.qr_style, v1 documents still read, and
// GET returns the encoded matrices the browser engine needs to reproduce /qr/render exactly.
func TestIntegrationQRStyleV2(t *testing.T) {
	d, repo, svc := newRouteService(t)
	svc.SetQR(repo, nil)
	ctx := context.Background()
	owner := d.CreateUser(t)
	route := createSmartLink(t, repo, owner, "s/qr-v2-"+uuid.NewString(), "https://example.com/a")
	account := authctx.Account{ID: owner.AccountID, Role: "owner"}

	// A v1 document written before TEC-1269 reads back mapped onto the v2 fields.
	if _, err := d.Pool.Exec(ctx, `UPDATE routes SET qr_style = '{"foreground":"#1F2937","background":"#ffffff","module":"dots","eye":"circle","quiet_zone":5}' WHERE uuid = $1`, route.UUID); err != nil {
		t.Fatal(err)
	}
	v1, err := svc.GetQR(ctx, route.UUID, account)
	if err != nil {
		t.Fatal(err)
	}
	if v1.Style.BodyShape != "circle" || v1.Style.EyeFrameShape != "circle" || v1.Style.EyeBallShape != "circle" || v1.Style.QuietZone != 5 {
		t.Fatalf("v1 document: %+v", v1.Style)
	}

	style := qr.DefaultStyle()
	style.BodyShape, style.EyeFrameShape, style.EyeBallShape = "rounded-in-soft", "leaf", "clover"
	style.FillMode, style.GradientStart, style.GradientEnd, style.GradientAngle, style.GradientOnEyes = qr.FillRadial, "#1e3a8a", "#000000", 0, true
	style.Eyes[1] = qr.EyeStyle{FlipH: true, FrameColor: "#7c2d12"}
	style.Eyes[2] = qr.EyeStyle{FlipV: true, BallColor: "#14532d"}
	style.TransparentBackground = true
	saved, err := svc.PutQR(ctx, route.UUID, account, style)
	if err != nil {
		t.Fatal(err)
	}
	got, err := svc.GetQR(ctx, route.UUID, account)
	if err != nil {
		t.Fatal(err)
	}
	if got.Style != saved.Style || got.Style.BodyShape != "rounded-in-soft" || got.Style.Eyes != style.Eyes || !got.Style.TransparentBackground {
		t.Fatalf("round trip: %+v", got.Style)
	}
	if len(got.Matrices.M) < 21 || len(got.Matrices.H) < len(got.Matrices.M) {
		t.Fatalf("matrices: %d/%d rows", len(got.Matrices.M), len(got.Matrices.H))
	}
	m, err := qr.ParseMatrix(got.Matrices.M)
	if err != nil {
		t.Fatal(err)
	}
	server, _, err := svc.RenderQR(ctx, route.UUID, account, "svg", 1024)
	if err != nil {
		t.Fatal(err)
	}
	if qr.RenderSVG(m, got.Style, 1024, "") != string(server) {
		t.Fatal("browser-side render of the returned matrix differs from /qr/render")
	}
}
