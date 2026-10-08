package sync

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"custom-rules/internal/types"
)

type SyncStage struct{}

func NewSyncStage() *SyncStage {
	return &SyncStage{}
}

func (s *SyncStage) Name() string { return "sync" }

func (s *SyncStage) Execute(ctx *types.BuildContext) error {
	fmt.Println("[Sync] Checking for updates...")
	if err := s.ensureAsset(ctx, ctx.Config.GeositeURL, ctx.Config.GeositeInput); err != nil {
		return fmt.Errorf("geosite sync failed: %w", err)
	}
	if err := s.ensureAsset(ctx, ctx.Config.GeoIPURL, ctx.Config.GeoIPInput); err != nil {
		return fmt.Errorf("geoIP sync failed: %w", err)
	}

	return nil
}

func (s *SyncStage) ensureAsset(ctx *types.BuildContext, url, targetPath string) error {
	if isFresh(targetPath) {
		fmt.Printf("  [Sync] Asset is fresh (<24h): %s (skipping download)\n", targetPath)
		return nil
	}

	return downloadFile(ctx, url, targetPath)

}

// isFreshToday checks if the target file exists and was modified on the current calendar day.
func isFresh(targetpath string) bool {
	info, err := os.Stat(targetpath)
	if err != nil {
		return false // File doesn't exist or is unreachable
	}
	return time.Since(info.ModTime()) < 24*time.Hour
}



// DownloadFile streams a URL directly to targetPath atomically via a temp file.
func downloadFile(ctx *types.BuildContext, url, targetPath string) error {
	fmt.Printf("  [Sync] Downloading %s ...\n", url)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP error %d: %s", resp.StatusCode, resp.Status)
	}

	tmpFile, err := os.CreateTemp(ctx.Config.AssetsDir, ".download-*.tmp")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()

	var success bool
	defer func() {
		tmpFile.Close()
		if !success {
			os.Remove(tmpPath)
		}
	}()

	if _, err := io.Copy(tmpFile, resp.Body); err != nil {
		return fmt.Errorf("download interrupted: %w", err)
	}

	if err := tmpFile.Sync(); err != nil {
		return fmt.Errorf("failed to sync: %w", err)
	}

	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("failed to close: %w", err)
	}

	if err := os.Rename(tmpPath, targetPath); err != nil {
		return fmt.Errorf("failed to rename: %w", err)
	}

	success = true
	return nil
}
