package assets

import "testing"

func TestQQGroupQR_Embedded(t *testing.T) {
	if QQGroupQR == nil || len(QQGroupQR.Content()) < 100 {
		t.Fatal("QQ group QR not embedded")
	}
	// PNG magic
	b := QQGroupQR.Content()
	if len(b) < 8 || b[0] != 0x89 || b[1] != 'P' || b[2] != 'N' || b[3] != 'G' {
		t.Fatalf("expected PNG payload, got header %v", b[:8])
	}
	if QQGroupNumber != "555354813" {
		t.Fatal(QQGroupNumber)
	}
}
