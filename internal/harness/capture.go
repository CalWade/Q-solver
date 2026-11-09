package harness

import (
	captureapp "Q-Solver/internal/capture/application"
	capturedomain "Q-Solver/internal/capture/domain"
	captureinfra "Q-Solver/internal/capture/infrastructure"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
)

type fixtureScreen struct{}

func (fixtureScreen) Capture(context.Context, string) (image.Image, error) {
	frame := image.NewRGBA(image.Rect(0, 0, 96, 64))
	for y := 0; y < 64; y++ {
		for x := 0; x < 96; x++ {
			frame.Set(x, y, color.RGBA{uint8(x * 2), uint8(y * 3), 100, 255})
		}
	}
	return frame, nil
}
func (s *Server) preview(args []json.RawMessage) (any, error) {
	quality, err := argument[int](args, 0)
	if err != nil {
		return nil, err
	}
	sharpen, err := argument[float64](args, 1)
	if err != nil {
		return nil, err
	}
	gray, err := argument[bool](args, 2)
	if err != nil {
		return nil, err
	}
	raw, err := argument[bool](args, 3)
	if err != nil {
		return nil, err
	}
	mode, err := argument[string](args, 4)
	if err != nil {
		return nil, err
	}
	result, err := captureapp.New(fixtureScreen{}, captureinfra.Processor{}, s.windows).Capture(context.Background(), capturedomain.Options{Quality: quality, Sharpen: sharpen, Grayscale: gray, NoCompression: raw, Mode: mode})
	if err != nil {
		return nil, err
	}
	return map[string]string{"base64": "data:" + result.MIME + ";base64," + base64.StdEncoding.EncodeToString(result.Bytes), "size": fmt.Sprintf("%.2f KB", float64(len(result.Bytes))/1024)}, nil
}
