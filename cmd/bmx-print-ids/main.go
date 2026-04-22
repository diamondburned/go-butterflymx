//go:build goexperiment.jsonv2

package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"libdb.so/go-butterflymx"
)

var (
	printJSON = false
)

func init() {
	flag.BoolVar(&printJSON, "json", false, "output raw JSON instead of a tree (not yet implemented)")
}

func main() {
	log.SetFlags(0)
	flag.Parse()
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

	for ti, tenant := range tenants {
		isLastTenant := ti == len(tenants)-1
		tenantBranch := newConnector(nil, isLastTenant)
		fmt.Println(tenantBranch.nodef("tenant id=%v name=%q", tenant.ID, tenant.Name))

		keychains, err := client.Keychains(ctx, tenant.ID.Number, butterflymx.ActiveAccessCode)
		if err != nil {
			log.Printf("warning: failed to fetch keychains for tenant %q: %v", tenant.Name, err)
			continue
		}

		for ki, keychain := range keychains.Data {
			attrs := keychain.Attributes
			timeInfo := formatTimeRange(attrs.StartsAt, attrs.EndsAt)

			isLastKeychain := ki == len(keychains.Data)-1
			keychainBranch := newConnector(&tenantBranch, isLastKeychain)
			fmt.Println(keychainBranch.nodef("keychain id=%v name=%q %s", keychain.ID, attrs.Name, timeInfo))

			virtualKeys, err := butterflymx.CollectResults(keychain.Relationships.VirtualKeys.Resolve(keychains.Refs))
			if err != nil {
				log.Printf("warning: failed to fetch virtual keys for keychain %q: %v", attrs.Name, err)
				continue
			}

			for vi, vk := range virtualKeys {
				isLastKey := vi == len(virtualKeys)-1
				keyBranch := newConnector(&keychainBranch, isLastKey)

				vAttrs := vk.Attributes
				var suffix string
				if !vAttrs.SentAt.IsZero() {
					suffix = fmt.Sprintf(" (sent: %s)", vAttrs.SentAt.Local().Format(time.DateOnly))
				}

				fmt.Println(keyBranch.nodef("virtual key id=%v name=%q%s", vk.ID, vAttrs.Name, suffix))
			}
		}
	}
}

type connector struct {
	parent *connector
	prefix string
	last   bool
}

const (
	nodeCurr  = "├── "
	nodeLast  = "└── "
	childCurr = "│   "
	childLast = "    "
)

func newConnector(parent *connector, last bool) connector {
	var prefix string
	if parent != nil {
		prefix = parent.prefix + pickConnector(childCurr, childLast, parent.last) + prefix
	}
	return connector{
		parent: parent,
		prefix: prefix,
		last:   last,
	}
}

func pickConnector(curr, last string, isLast bool) string {
	if isLast {
		return last
	}
	return curr
}

func (c connector) node() string {
	return pickConnector(nodeCurr, nodeLast, c.last)
}

func (c connector) nodef(format string, args ...any) string {
	return c.prefix + c.node() + fmt.Sprintf(format, args...)
}

func formatTimeRange(start, end time.Time) string {
	if start.IsZero() && end.IsZero() {
		return ""
	}
	var parts []string
	if !start.IsZero() {
		parts = append(parts, "from: "+start.Local().Format(time.DateOnly))
	}
	if !end.IsZero() {
		parts = append(parts, "until: "+end.Local().Format(time.DateOnly))
	}
	return " (" + strings.Join(parts, ", ") + ")"
}
