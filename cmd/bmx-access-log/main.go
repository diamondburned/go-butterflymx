//go:build goexperiment.jsonv2

package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"slices"
	"time"

	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"
	butterflymx "libdb.so/go-butterflymx"
)

func main() {
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

	var entries []accessEntry

	for _, tenant := range tenants {
		keychains, err := client.Keychains(ctx, tenant.ID.Number, butterflymx.ActiveAccessCode)
		if err != nil {
			log.Printf("warning: failed to fetch keychains for tenant %q: %v", tenant.Name, err)
			continue
		}

		for _, keychain := range keychains.Data {
			virtualKeys, err := butterflymx.CollectResults(keychain.Relationships.VirtualKeys.Resolve(keychains.Refs))
			if err != nil {
				log.Printf("warning: failed to fetch virtual keys for keychain %q: %v", keychain.Attributes.Name, err)
				continue
			}

			for _, virtualKey := range virtualKeys {
				doorReleases, err := butterflymx.CollectResults(virtualKey.Relationships.DoorReleases.Resolve(keychains.Refs))
				if err != nil {
					log.Printf("warning: failed to fetch door releases for virtual key %q: %v", virtualKey.Attributes.Name, err)
					continue
				}

				for _, doorRelease := range doorReleases {
					panel, err := doorRelease.Relationships.Panel.Data.Resolve(keychains.Refs)
					if err != nil {
						log.Printf("warning: failed to resolve panel for door release %q: %v", doorRelease.Attributes.Name, err)
						continue
					}

					entries = append(entries, accessEntry{
						Tenant:      &tenant,
						Keychain:    &keychain,
						VirtualKey:  virtualKey,
						DoorRelease: doorRelease,
						Panel:       panel,
						Timestamp:   doorRelease.Attributes.LoggedAt,
					})
				}
			}
		}
	}

	slices.SortFunc(entries, func(a, b accessEntry) int {
		return a.Timestamp.Compare(b.Timestamp)
	})

	printAccessLog(entries)
}

type accessEntry struct {
	Tenant      *butterflymx.Tenant
	Keychain    *butterflymx.Keychain
	VirtualKey  *butterflymx.VirtualKey
	DoorRelease *butterflymx.DoorRelease
	Panel       *butterflymx.Panel
	Timestamp   time.Time
}

func printAccessLog(entries []accessEntry) {
	if len(entries) == 0 {
		fmt.Println("No access log entries found.")
		return
	}

	headerStyle := lipgloss.NewStyle().Bold(true).Padding(0, 1)
	cellStyle := lipgloss.NewStyle().Padding(0, 1)
	borderStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))

	headers := []string{
		"Time",
		"Name",
		"Email",
		"PIN",
		"Release Method",
		"Door",
	}
	rowFunc := func(entry accessEntry) []string {
		return []string{
			entry.Timestamp.Local().Format(time.Stamp),
			entry.VirtualKey.Attributes.Name,
			entry.VirtualKey.Attributes.Email,
			entry.VirtualKey.Attributes.PINCode.String(),
			entry.DoorRelease.Attributes.ReleaseMethod,
			entry.Panel.Attributes.Name,
		}
	}

	t := table.New().
		Border(lipgloss.NormalBorder()).
		BorderStyle(borderStyle).
		StyleFunc(func(row, col int) lipgloss.Style {
			if row == table.HeaderRow {
				return headerStyle
			}
			return cellStyle
		}).
		Headers(headers...)
	for _, e := range entries {
		t.Row(rowFunc(e)...)
	}

	fmt.Println(t.Render())
}
