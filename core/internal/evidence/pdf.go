package evidence

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"time"
)

type PDFExtractor interface {
	Extract(context.Context, []byte) (string, error)
}
type Poppler struct{}
type limitedBuffer struct {
	bytes.Buffer
	Limit int
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	if b.Len()+len(p) > b.Limit {
		return 0, fmt.Errorf("extracted document exceeds limit")
	}
	return b.Buffer.Write(p)
}
func (Poppler) Extract(ctx context.Context, pdf []byte) (string, error) {
	if len(pdf) > 12<<20 || !bytes.HasPrefix(pdf, []byte("%PDF-")) {
		return "", fmt.Errorf("valid PDF under 12 MiB required")
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "pdftotext", "-layout", "-enc", "UTF-8", "-", "-")
	cmd.Stdin = bytes.NewReader(pdf)
	out := &limitedBuffer{Limit: 16 << 20}
	cmd.Stdout = out
	cmd.Stderr = &limitedBuffer{Limit: 4096}
	if e := cmd.Run(); e != nil {
		return "", fmt.Errorf("PDF extraction failed (unsupported, encrypted, scanned or oversized)")
	}
	if out.Len() < 50 {
		return "", fmt.Errorf("PDF has no usable text; manual/OCR preparation required")
	}
	return out.String(), nil
}
