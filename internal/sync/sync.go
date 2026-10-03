package sync

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"custom-rules/internal/types"
)

// SyncStage downloads the upstream geosite/geoip data files if they are
// missing or older than 24 hours.
type SyncStage struct{}

func NewSyncStage() *SyncStage {
	return &SyncStage{}
}

func (s *SyncStage) Name() string { return "sync" }

func (s *SyncStage) Execute(ctx *types.BuildContext) error {
	cfg := ctx.Config
	assets := map[string]string{
		cfg.GeositeInput: cfg.GeositeURL,
		cfg.GeoIPInput:   cfg.GeoIPURL,
	}

	for path, url := range assets {
		info, err := os.Stat(path)
		if err == nil {
			if time.Since(info.ModTime()) < 24*time.Hour {
				fmt.Printf(" ⏭️ %s is up to date, skipping download.\n", path)
				continue
			}
		}

		tmpPath := path + ".tmp"
		fmt.Printf(" 📥 Downloading %s...\n", path)
		if err := downloadFile(tmpPath, url); err != nil {
			return err
		}

		if err := os.Rename(tmpPath, path); err != nil {
			return fmt.Errorf("failed to finalize %s: %w", path, err)
		}
		fmt.Printf(" ✅ Successfully updated %s\n", path)
	}

	return nil
}

func downloadFile(filepath string, url string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status: %s", resp.Status)
	}

	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}
