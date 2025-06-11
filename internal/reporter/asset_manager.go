package reporter

import (
	"embed"
	"encoding/base64"
	"fmt"
	"html/template"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/rs/zerolog"
)

// AssetManager manages embedding and copying assets
type AssetManager struct {
	logger zerolog.Logger
}

// NewAssetManager creates a new AssetManager
func NewAssetManager(logger zerolog.Logger) *AssetManager {
	return &AssetManager{
		logger: logger,
	}
}

// CopyEmbedDir copies directory from embed.FS to filesystem
func (am *AssetManager) CopyEmbedDir(efs embed.FS, srcDir, destDir string) error {
	return fs.WalkDir(efs, srcDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Calculate destination path
		relPath, err := filepath.Rel(srcDir, path)
		if err != nil {
			return fmt.Errorf("failed to get relative path for %s: %w", path, err)
		}
		destPath := filepath.Join(destDir, relPath)

		if d.IsDir() {
			// Create directory if it doesn't exist
			if err := os.MkdirAll(destPath, DirPermissions); err != nil {
				return fmt.Errorf("failed to create directory %s: %w", destPath, err)
			}
		} else {
			// Read file content from embed.FS
			data, err := efs.ReadFile(path)
			if err != nil {
				return fmt.Errorf("failed to read embedded file %s: %w", path, err)
			}
			// Write file to destination
			if err := os.WriteFile(destPath, data, FilePermissions); err != nil {
				return fmt.Errorf("failed to write file %s: %w", destPath, err)
			}
		}
		return nil
	})
}

// EmbedAssetContent reads and returns asset content from the provided embed filesystems
func (am *AssetManager) EmbedAssetContent(cssFS, jsFS embed.FS, isCSS bool) (string, error) {
	var assetData []byte
	var err error
	var defaultAssetPath string
	var embeddedFS embed.FS
	assetTypeStr := "JS"

	if isCSS {
		assetTypeStr = "CSS"
		defaultAssetPath = EmbeddedCSSPath
		embeddedFS = cssFS
	} else {
		defaultAssetPath = EmbeddedJSPath
		embeddedFS = jsFS
	}

	am.logger.Debug().Str("asset", defaultAssetPath).Msgf("Using default embedded %s asset.", assetTypeStr)
	assetData, err = embeddedFS.ReadFile(defaultAssetPath)
	if err != nil {
		am.logger.Error().Err(err).Str("asset", defaultAssetPath).Msgf("FATAL: Failed to read default embedded %s asset. This should not happen.", assetTypeStr)
		return "", fmt.Errorf("failed to read default embedded %s asset '%s': %w", assetTypeStr, defaultAssetPath, err)
	}

	return string(assetData), nil
}

// EmbedAssetContentWithPaths reads and returns asset content with custom paths
func (am *AssetManager) EmbedAssetContentWithPaths(cssFS, jsFS embed.FS, cssPath, jsPath string, isCSS bool) (string, error) {
	var assetData []byte
	var err error
	var assetPath string
	var embeddedFS embed.FS
	assetTypeStr := "JS"

	if isCSS {
		assetTypeStr = "CSS"
		assetPath = cssPath
		embeddedFS = cssFS
	} else {
		assetPath = jsPath
		embeddedFS = jsFS
	}

	am.logger.Debug().Str("asset", assetPath).Msgf("Using embedded %s asset.", assetTypeStr)
	assetData, err = embeddedFS.ReadFile(assetPath)
	if err != nil {
		am.logger.Error().Err(err).Str("asset", assetPath).Msgf("FATAL: Failed to read embedded %s asset. This should not happen.", assetTypeStr)
		return "", fmt.Errorf("failed to read embedded %s asset '%s': %w", assetTypeStr, assetPath, err)
	}

	return string(assetData), nil
}

// EmbedAssetsIntoPageData embeds CSS and JS into page data
func (am *AssetManager) EmbedAssetsIntoPageData(pageData PageDataInterface, cssFS, jsFS embed.FS, embedAssets bool) {
	if !embedAssets {
		return
	}

	// Embed CSS
	cssContent, cssErr := am.EmbedAssetContent(cssFS, jsFS, true)
	if cssErr != nil {
		am.logger.Warn().Err(cssErr).Msg("Failed to embed CSS, report styling might be affected.")
	}
	pageData.SetCustomCSS(template.CSS(cssContent))

	// Embed JS
	jsContent, jsErr := am.EmbedAssetContent(cssFS, jsFS, false)
	if jsErr != nil {
		am.logger.Warn().Err(jsErr).Msg("Failed to embed JS, report functionality might be affected.")
	}
	pageData.SetReportJs(template.JS(jsContent))
}

// EmbedAssetsIntoPageDataWithPaths embeds CSS and JS into page data with custom paths
func (am *AssetManager) EmbedAssetsIntoPageDataWithPaths(pageData PageDataInterface, cssFS, jsFS embed.FS, cssPath, jsPath string, embedAssets bool) {
	if !embedAssets {
		return
	}

	// Embed CSS
	cssContent, cssErr := am.EmbedAssetContentWithPaths(cssFS, jsFS, cssPath, jsPath, true)
	if cssErr != nil {
		am.logger.Warn().Err(cssErr).Msg("Failed to embed CSS, report styling might be affected.")
	}
	pageData.SetCustomCSS(template.CSS(cssContent))

	// Embed JS
	jsContent, jsErr := am.EmbedAssetContentWithPaths(cssFS, jsFS, cssPath, jsPath, false)
	if jsErr != nil {
		am.logger.Warn().Err(jsErr).Msg("Failed to embed JS, report functionality might be affected.")
	}
	pageData.SetReportJs(template.JS(jsContent))
}

// PageDataInterface interface for setting assets into page data
type PageDataInterface interface {
	SetCustomCSS(template.CSS)
	SetReportJs(template.JS)
}

// TODO: Check if this function is used
// EncodeFaviconToBase64 encodes favicon bytes to base64 string
func EncodeFaviconToBase64(faviconBytes []byte) string {
	return base64.StdEncoding.EncodeToString(faviconBytes)
}
