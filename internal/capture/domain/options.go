package domain

import "fmt"

type Options struct {
	Quality                  int
	Sharpen                  float64
	Grayscale, NoCompression bool
	Mode                     string
}

func (o Options) Validate() error {
	if o.Mode != "window" && o.Mode != "fullscreen" {
		return fmt.Errorf("invalid capture mode %q", o.Mode)
	}
	if o.Quality < 1 || o.Quality > 100 {
		return fmt.Errorf("capture quality must be between 1 and 100")
	}
	if o.Sharpen < 0 || o.Sharpen > 5 {
		return fmt.Errorf("capture sharpening must be between 0 and 5")
	}
	return nil
}

type Result struct {
	Bytes []byte
	MIME  string
}
