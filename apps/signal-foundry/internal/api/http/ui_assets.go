package http

import (
	"embed"
	"io/fs"
)

const (
	embeddedUIRootDir       = "embeddedui"
	embeddedUIDistDir       = "dist"
	embeddedUIDistIndexPath = embeddedUIDistDir + "/index.html"
)

//go:embed embeddedui
var embeddedUI embed.FS

func getEmbeddedUIFiles() fs.FS {
	return resolveEmbeddedUIFiles(embeddedUI)
}

func resolveEmbeddedUIFiles(embeddedFiles fs.FS) fs.FS {
	uiFiles, subErr := fs.Sub(embeddedFiles, embeddedUIRootDir+"/"+embeddedUIDistDir)
	if subErr != nil {
		return nil
	}

	if _, statErr := fs.Stat(uiFiles, "index.html"); statErr != nil {
		return nil
	}

	return uiFiles
}
