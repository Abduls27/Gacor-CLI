package ui

type ProgressEvent struct {
	Stage string // "download" | "extract" | "initialize" | "configure" | "start" | "done"
	Label string // teks bebas, misal "PHP 8.4.26"
	Done  int64
	Total int64
}
