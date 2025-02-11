package main

import (
	"context"
	"embed"
	"html"
	"net/http"
	"os"
	"text/template"
	"time"

	"github.com/kelseyhightower/envconfig"
	"github.com/rs/zerolog"
)

type Settings struct {
	Port          string `envconfig:"PORT" default:"2999"`
	CanonicalHost string `envconfig:"CANONICAL_HOST" default:"njump.me"`
}

//go:embed static/*
var static embed.FS

//go:embed templates/*
var templates embed.FS

var (
	s               Settings
	tmpl            *template.Template
	templateMapping map[string]string
	log             = zerolog.New(os.Stderr).Output(zerolog.ConsoleWriter{Out: os.Stdout}).With().Timestamp().Logger()
)

func updateArchives(ctx context.Context) {
	// do this so we don't run this every time we restart it locally
	time.Sleep(10 * time.Minute)

	for {
		select {
		case <-ctx.Done():
			return
		default:
			loadNpubsArchive(ctx)
			loadRelaysArchive(ctx)
		}
		time.Sleep(24 * time.Hour)
	}
}

func main() {
	err := envconfig.Process("", &s)
	if err != nil {
		log.Fatal().Err(err).Msg("couldn't process envconfig.")
	}
	// initialize disk cache
	defer cache.initialize()()

	// initialize the function to update the npubs/relays archive
	ctx := context.Background()
	go updateArchives(ctx)

	// initialize templates
	// use a mapping to expressly link the templates and share them between more kinds/types
	templateMapping = map[string]string{
		"homepage":              "homepage.html",
		"profile":               "profile.html",
		"profile_sitemap":       "sitemap.xml",
		"note":                  "note.html",
		"telegram_instant_view": "telegram_instant_view.html",
		"address":               "other.html",
		"relay":                 "relay.html",
		"relay_sitemap":         "sitemap.xml",
		"archive":               "archive.html",
		"archive_sitemap":       "sitemap.xml",
		"robots":                "robots.txt",
	}

	funcMap := template.FuncMap{
		"basicFormatting":        func(input string) string { return basicFormatting(input, false, false) },
		"previewNotesFormatting": previewNotesFormatting,
		"escapeString":           html.EscapeString,
		"sanitizeXSS":            sanitizeXSS,
		"trimProtocol":           trimProtocol,
	}

	tmpl = template.Must(
		template.New("tmpl").
			Funcs(funcMap).
			ParseFS(templates, "templates/*"),
	)

	// routes
	http.HandleFunc("/", render)
	http.HandleFunc("/try", renderTry)
	http.HandleFunc("/robots.txt", renderRobots)
	http.HandleFunc("/njump/image/", generate)
	http.HandleFunc("/njump/proxy/", proxy)
	http.Handle("/njump/static/", http.StripPrefix("/njump/", http.FileServer(http.FS(static))))
	http.HandleFunc("/services/oembed", renderOEmbed)
	http.HandleFunc("/npubs-archive/", renderArchive)
	http.HandleFunc("/relays-archive/", renderArchive)
	http.HandleFunc("/npubs-archive.xml", renderArchive)
	http.HandleFunc("/relays-archive.xml", renderArchive)

	log.Print("listening at http://0.0.0.0:" + s.Port)
	if err := http.ListenAndServe("0.0.0.0:"+s.Port, nil); err != nil {
		log.Fatal().Err(err).Msg("")
	}

	select {}
}
