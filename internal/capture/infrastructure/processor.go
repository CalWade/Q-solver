package infrastructure

import (
	"Q-Solver/internal/capture/domain"
	imageutil "Q-Solver/pkg/ImageUtil"
	"bytes"
	"image"
	"image/png"
)

type Processor struct{}

func (Processor) Process(frame image.Image, options domain.Options) (domain.Result, error) {
	if options.NoCompression {
		var buffer bytes.Buffer
		if err := png.Encode(&buffer, frame); err != nil {
			return domain.Result{}, err
		}
		return domain.Result{Bytes: buffer.Bytes(), MIME: "image/png"}, nil
	}
	data, err := imageutil.CompressForOCR(frame, options.Quality, options.Sharpen, options.Grayscale)
	return domain.Result{Bytes: data, MIME: "image/jpeg"}, err
}
