package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

func syncAssets() error {
	assets := map[string]string{
		GeositeInput: GeositeURL,
		GeoIPInput:   GeoIPURL,
	}

	for path, url := range assets {
		// 1. Check if file exists and is recent (today)
		info, err := os.Stat(path)
		if err == nil {
			// Skip if modified less than 24 hours ago
			if time.Since(info.ModTime()) < 24*time.Hour {
				fmt.Printf("  ⏭️  %s is up to date, skipping download.\n", path)
				continue
			}
		}

		// 2. Download to .tmp file (for Atomic Swap)
		tmpPath := path + ".tmp"
		fmt.Printf("  📥 Downloading %s...\n", path)
		if err := downloadFile(tmpPath, url); err != nil {
			return err
		}

		// 3. Atomic Swap
		if err := os.Rename(tmpPath, path); err != nil {
			return fmt.Errorf("failed to finalize %s: %w", path, err)
		}
		fmt.Printf("  ✅ Successfully updated %s\n", path)
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