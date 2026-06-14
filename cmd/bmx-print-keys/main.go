//go:build goexperiment.jsonv2

package main

import (
	"cmp"
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"slices"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"
	butterflymx "libdb.so/go-butterflymx"
)

type keyEntry struct {
	KeychainID butterflymx.ID
	KeyName    string
	PINCode    string
	QRCodeURL  string
}

func main() {
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage: %s [keychainIDs...]\n", os.Args[0])
	}
	flag.Parse()

	log.SetFlags(0)
	ctx := context.Background()

	apiToken := os.Getenv("BUTTERFLYMX_API_TOKEN")
	if apiToken == "" {
		log.Fatal("BUTTERFLYMX_API_TOKEN environment variable is required")
	}

	client := butterflymx.NewAPIClient(butterflymx.APIStaticToken(apiToken), nil)

	tenants, err := butterflymx.CollectResults(client.Tenants(ctx))
	if err != nil {
		log.Fatalf("failed to fetch tenants: %v", err)
	}
	if len(tenants) == 0 {
		log.Fatal("no tenants found for this account")
	}

	var entries []keyEntry

	for _, tenant := range tenants {
		keychains, err := client.Keychains(ctx, tenant.ID.Number, butterflymx.ActiveAccessCode)
		if err != nil {
			log.Printf("warning: failed to fetch keychains for tenant %q: %v", tenant.Name, err)
			continue
		}

		for _, keychain := range keychains.Data {
			if len(flag.Args()) > 0 && !slices.Contains(flag.Args(), fmt.Sprint(keychain.ID)) {
				continue
			}

			virtualKeys, err := butterflymx.CollectResults(keychain.Relationships.VirtualKeys.Resolve(keychains.Refs))
			if err != nil {
				log.Printf("warning: failed to fetch virtual keys for keychain %q: %v", keychain.Attributes.Name, err)
				continue
			}

			for _, vk := range virtualKeys {
				entries = append(entries, keyEntry{
					KeychainID: keychain.ID,
					KeyName:    vk.Attributes.Name,
					PINCode:    vk.Attributes.PINCode.String(),
					QRCodeURL:  vk.Attributes.QRCodeImageURL,
				})
			}
		}
	}

	slices.SortStableFunc(entries, func(a, b keyEntry) int {
		aIDPart, _, _ := strings.Cut(a.KeyName, ":")
		aID, _ := strconv.Atoi(aIDPart)

		bIDPart, _, _ := strings.Cut(b.KeyName, ":")
		bID, _ := strconv.Atoi(bIDPart)

		if aID != 0 && bID != 0 {
			return cmp.Compare(aID, bID)
		}

		return cmp.Compare(a.KeyName, b.KeyName)
	})

	printKeysTable(entries)
}

func printKeysTable(entries []keyEntry) {
	if len(entries) == 0 {
		fmt.Println("No keychains or virtual keys found.")
		return
	}

	headerStyle := lipgloss.NewStyle().Bold(true).Padding(0, 1)
	cellStyle := lipgloss.NewStyle().Padding(0, 1)
	borderStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))

	headers := []string{
		"Keychain ID",
		"Virtual Key Name",
		"PIN Code",
		"QR Code Link",
	}

	t := table.New().
		Border(lipgloss.NormalBorder()).
		BorderStyle(borderStyle).
		StyleFunc(func(row, col int) lipgloss.Style {
			if row == table.HeaderRow {
				return headerStyle
			}
			style := cellStyle
			if col == 3 && row >= 0 && row < len(entries) {
				url := entries[row].QRCodeURL
				if url != "" {
					style = style.Hyperlink(url)
				}
			}
			return style
		}).
		Headers(headers...)

	for _, entry := range entries {
		qrCellText := ""
		if entry.QRCodeURL != "" {
			qrCellText = "QR Code"
		}
		t.Row(
			strconv.Itoa(int(entry.KeychainID)),
			entry.KeyName,
			entry.PINCode,
			qrCellText,
		)
	}

	fmt.Println(t.Render())
}
