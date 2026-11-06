package infrastructure

import (
	"Q-Solver/internal/capture/domain"
	"bytes"
	"image"
	"image/color"
	_ "image/jpeg"
	_ "image/png"
	"testing"
)

func TestProcessorProducesDecodablePNGAndJPEG(t *testing.T) {
	frame := image.NewRGBA(image.Rect(0, 0, 48, 32))
	frame.Set(2, 2, color.RGBA{255, 0, 0, 255})
	for _, raw := range []bool{true, false} {
		result, err := (Processor{}).Process(frame, domain.Options{NoCompression: raw, Quality: 80})
		if err != nil {
			t.Fatal(err)
		}
		decoded, format, err := image.Decode(bytes.NewReader(result.Bytes))
		if err != nil {
			t.Fatal(err)
		}
		if decoded.Bounds().Dx() != 48 || decoded.Bounds().Dy() != 32 {
			t.Fatal(decoded.Bounds())
		}
		if raw && (format != "png" || result.MIME != "image/png") {
			t.Fatal(result.MIME, format)
		}
		if !raw && (format != "jpeg" || result.MIME != "image/jpeg") {
			t.Fatal(result.MIME, format)
		}
	}
}
