package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/Ken-Chy129/note-foundry/backend/internal/historyimport"
)

func main() {
	if err := run(context.Background(), os.Args[1:], os.Stdout); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context, arguments []string, stdout io.Writer) error {
	flags := flag.NewFlagSet("historyimport", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	source := flags.String("source", "", "directory containing historical document exports")
	output := flags.String("output", "", "empty directory for the generated staging package")
	allowedAssetHosts := flags.String("allowed-asset-hosts", "cdn.nlark.com", "comma-separated HTTPS hosts allowed for remote images")
	skipRemoteAssets := flags.Bool("skip-remote-assets", false, "leave remote image URLs in Markdown and report them as unresolved")
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	if strings.TrimSpace(*source) == "" {
		return errors.New("-source is required")
	}
	if strings.TrimSpace(*output) == "" {
		return errors.New("-output is required")
	}

	documents, scanIssues, err := historyimport.ScanSourceDirectory(ctx, *source)
	if err != nil {
		return err
	}
	prepared := historyimport.PrepareDocuments(documents)

	var fetcher historyimport.AssetFetcher
	if !*skipRemoteAssets {
		fetcher = historyimport.HTTPAssetFetcher{AllowedHosts: parseAllowedHosts(*allowedAssetHosts)}
	}
	manifest, err := historyimport.WriteStaging(ctx, *output, prepared, scanIssues, fetcher)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(stdout, "staging=%s documents=%d duplicates=%d issues=%d\n", *output, len(manifest.Documents), len(manifest.Duplicates), len(manifest.Issues))
	return err
}

func parseAllowedHosts(value string) map[string]bool {
	hosts := make(map[string]bool)
	for _, host := range strings.Split(value, ",") {
		host = strings.ToLower(strings.TrimSpace(host))
		if host != "" {
			hosts[host] = true
		}
	}
	return hosts
}
