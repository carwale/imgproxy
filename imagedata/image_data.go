package imagedata

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/imgproxy/imgproxy/v3/config"
	"github.com/imgproxy/imgproxy/v3/ierrors"
	"github.com/imgproxy/imgproxy/v3/imagetype"
	"github.com/imgproxy/imgproxy/v3/security"
)

var (
	Watermark     *ImageData
	CWWatermark     *ImageData
	BWWatermark     *ImageData
	BWWatermarkV2     *ImageData
	ArtifactMap map[string] *ImageData = make(map[string]*ImageData)
	FallbackImage *ImageData
)

type ImageData struct {
	Type    imagetype.Type
	Data    []byte
	Headers map[string]string

	cancel     context.CancelFunc
	cancelOnce sync.Once
}

func (d *ImageData) Close() {
	d.cancelOnce.Do(func() {
		if d.cancel != nil {
			d.cancel()
		}
	})
}

func (d *ImageData) SetCancel(cancel context.CancelFunc) {
	d.cancel = cancel
}

func Init() error {
	initRead()

	if err := initDownloading(); err != nil {
		return err
	}

	if err := loadWatermarkAndArtifacts(); err != nil {
		return err
	}

	if err := loadFallbackImage(); err != nil {
		return err
	}

	return nil
}

func loadWatermarkAndArtifacts() error {
	ctx := context.Background()

	watermarkVars := map[string]**ImageData{
        "cw_watermark":    &CWWatermark,
        "bw_watermark":    &BWWatermark,
        "bw_watermark_v2": &BWWatermarkV2,
    }

	// Download watermarks
	for key, url := range config.WatermarkPaths {
		download, err := Download(ctx, url, "watermark", DownloadOptions{}, security.DefaultOptions())
		if err != nil {
			return fmt.Errorf("failed to download watermark from %s: %w", url, err)
		}
		*watermarkVars[key] = download // Assign the downloaded image data to the pointer
	}

	// Download artifacts
	for artifactType, artifactPath := range config.Artifacts {
		sizes, exists := config.ArtifactsSizesMap[artifactType]
		if !exists {
			continue
		}

		for _, size := range sizes {
			artifactURL := strings.Replace(artifactPath, "*", size, 1)
			artifact, err := Download(ctx, artifactURL, "watermark", DownloadOptions{}, security.DefaultOptions())
			if err != nil {
				return fmt.Errorf("failed to download artifact %s (%s): %w", artifactType, size, err)
			}
			ArtifactMap[fmt.Sprintf("%s_%s", artifactType, size)] = artifact
		}
	}

	return nil
}


func loadFallbackImage() (err error) {
	switch {
	case len(config.FallbackImageData) > 0:
		FallbackImage, err = FromBase64(config.FallbackImageData, "fallback image", security.DefaultOptions())
	case len(config.FallbackImagePath) > 0:
		FallbackImage, err = FromFile(config.FallbackImagePath, "fallback image", security.DefaultOptions())
	case len(config.FallbackImageURL) > 0:
		FallbackImage, err = Download(context.Background(), config.FallbackImageURL, "fallback image", DownloadOptions{Header: nil, CookieJar: nil}, security.DefaultOptions())
	default:
		FallbackImage, err = nil, nil
	}

	if FallbackImage != nil && err == nil && config.FallbackImageTTL > 0 {
		if FallbackImage.Headers == nil {
			FallbackImage.Headers = make(map[string]string)
		}
		FallbackImage.Headers["Fallback-Image"] = "1"
	}

	return err
}

func FromBase64(encoded, desc string, secopts security.Options) (*ImageData, error) {
	dec := base64.NewDecoder(base64.StdEncoding, strings.NewReader(encoded))
	size := 4 * (len(encoded)/3 + 1)

	imgdata, err := readAndCheckImage(dec, size, secopts)
	if err != nil {
		return nil, fmt.Errorf("Can't decode %s: %s", desc, err)
	}

	return imgdata, nil
}

func FromFile(path, desc string, secopts security.Options) (*ImageData, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("Can't read %s: %s", desc, err)
	}

	fi, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("Can't read %s: %s", desc, err)
	}

	imgdata, err := readAndCheckImage(f, int(fi.Size()), secopts)
	if err != nil {
		return nil, fmt.Errorf("Can't read %s: %s", desc, err)
	}

	return imgdata, nil
}

func Download(ctx context.Context, imageURL, desc string, opts DownloadOptions, secopts security.Options) (*ImageData, error) {
	imgdata, err := download(ctx, imageURL, opts, secopts)
	if err != nil {
		return nil, ierrors.Wrap(
			err, 0,
			ierrors.WithPrefix(fmt.Sprintf("Can't download %s", desc)),
		)
	}

	return imgdata, nil
}

func Upload(ctx context.Context, imageURL, desc string, data *ImageData) (error) {
	err := upload(ctx, imageURL, data.Data)
	if err != nil {
		return ierrors.Wrap(
			err, 0,
			ierrors.WithPrefix(fmt.Sprintf("Can't upload %s", desc)),
		)
	}
	return nil
}
