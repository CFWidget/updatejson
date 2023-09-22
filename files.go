package main

import "embed"

//go:embed home.html
//go:embed app.css
//go:embed app.js
//go:embed favicon.ico
var webAssets embed.FS
