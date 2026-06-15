package weblog

import (
	"embed"
	"fmt"
	"net/url"
	"strings"
)

//go:embed assets/*
var assets embed.FS

// RenderLogViewerHTML renders the live Docker log viewer page.
func RenderLogViewerHTML(
	name string,
	session string,
	container string,
) string {

	streamURL := fmt.Sprintf(
		"/stream?session=%s&container=%s",
		url.QueryEscape(session),
		url.QueryEscape(container),
	)

	htmlBytes, err := assets.ReadFile(
		"assets/logviewer.html",
	)
	if err != nil {
		return errorPage(err)
	}

	cssBytes, err := assets.ReadFile(
		"assets/logviewer.css",
	)
	if err != nil {
		return errorPage(err)
	}

	jsBytes, err := assets.ReadFile(
		"assets/logviewer.js",
	)
	if err != nil {
		return errorPage(err)
	}

	html := string(htmlBytes)

	replacements := map[string]string{
		"{{TITLE}}":      name,
		"{{STREAM_URL}}": streamURL,
		"{{CSS}}":        string(cssBytes),
		"{{JS}}":         string(jsBytes),
	}

	for old, newValue := range replacements {
		html = strings.ReplaceAll(
			html,
			old,
			newValue,
		)
	}

	return html
}

func errorPage(err error) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<title>Error</title>
</head>
<body style="font-family:sans-serif;padding:20px">
<h2>Failed to load log viewer assets</h2>
<pre>%v</pre>
</body>
</html>`, err)
}