//go:build wails

package main

import (
	"embed"
	"flag"
	"log"
	"os"
	"path/filepath"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

//go:embed frontend/*
var assets embed.FS

// findConfigFile searches for config file in multiple locations
func findConfigFile(filename string) string {
	// 1. Check if file exists at the path specified (if absolute or relative)
	if _, err := os.Stat(filename); err == nil {
		absPath, _ := filepath.Abs(filename)
		log.Printf("Found %s at: %s", filename, absPath)
		return absPath
	}

	// 2. Check next to executable
	if execPath, err := os.Executable(); err == nil {
		execDir := filepath.Dir(execPath)
		log.Printf("Executable dir: %s", execDir)

		// For macOS .app bundle: /path/to/build/bin/App.app/Contents/MacOS/talk
		// We need to go up 3 levels to reach /path/to/build/bin/
		parentDir := filepath.Dir(execDir) // Contents
		if filepath.Base(parentDir) == "Contents" {
			appBundle := filepath.Dir(parentDir) // App.app
			appDir := filepath.Dir(appBundle)    // build/bin
			candidatePath := filepath.Join(appDir, filename)
			log.Printf("Checking for config next to app bundle: %s", candidatePath)
			if _, err := os.Stat(candidatePath); err == nil {
				log.Printf("Found %s next to app bundle: %s", filename, candidatePath)
				return candidatePath
			}
		}

		// Check next to executable directly
		candidatePath := filepath.Join(execDir, filename)
		if _, err := os.Stat(candidatePath); err == nil {
			log.Printf("Found %s next to executable: %s", filename, candidatePath)
			return candidatePath
		}
	}

	// 3. Check in current working directory
	if cwd, err := os.Getwd(); err == nil {
		candidatePath := filepath.Join(cwd, filename)
		if _, err := os.Stat(candidatePath); err == nil {
			log.Printf("Found %s in current directory: %s", filename, candidatePath)
			return candidatePath
		}
	}

	// 4. Check in user home directory
	if homeDir, err := os.UserHomeDir(); err == nil {
		candidatePath := filepath.Join(homeDir, ".talk", filename)
		if _, err := os.Stat(candidatePath); err == nil {
			log.Printf("Found %s in home directory: %s", filename, candidatePath)
			return candidatePath
		}
	}

	// Return original filename if not found (will use defaults)
	log.Printf("Config file %s not found, will use defaults", filename)
	return filename
}

func main() {
	// Parse flags
	configPathFlag := flag.String("config", "config.yaml", "Path to config file")
	flag.Parse()

	// Find config file in multiple locations
	configPath = findConfigFile(*configPathFlag)
	log.Printf("Config file: %s", configPath)

	// Create application with options
	app := NewApp()

	err := wails.Run(&options.App{
		Title:             "AI Multi-Agent Debate",
		Width:             1400,
		Height:            900,
		MinWidth:          1000,
		MinHeight:         700,
		DisableResize:     false,
		Fullscreen:        false,
		Frameless:         false,
		StartHidden:       false,
		HideWindowOnClose: false,
		BackgroundColour:  &options.RGBA{R: 27, G: 38, B: 54, A: 1},
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		OnStartup:  app.startup,
		OnShutdown: app.shutdown,
		Bind: []interface{}{
			app,
		},
		// Mac specific options
		Mac: &mac.Options{
			TitleBar: &mac.TitleBar{
				TitlebarAppearsTransparent: false,
				HideTitle:                  false,
				HideTitleBar:               false,
				FullSizeContent:            false,
				UseToolbar:                 false,
				HideToolbarSeparator:       true,
			},
			About: &mac.AboutInfo{
				Title:   "AI Multi-Agent Debate",
				Message: "Ứng dụng Multi-Agent AI cho phép nhiều AI từ các providers khác nhau thảo luận, phản biện và góp ý với nhau.\n\nVersion: 1.0.0",
			},
		},
		// Windows specific options
		Windows: &windows.Options{
			WebviewIsTransparent: false,
			WindowIsTranslucent:  false,
			DisableWindowIcon:    false,
		},
	})

	if err != nil {
		log.Fatalf("Error: %v", err)
	}
}
