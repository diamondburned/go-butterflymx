package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"golang.org/x/oauth2"
	"libdb.so/go-butterflymx"
)

func main() {
	log.SetFlags(0)
	ctx := context.TODO()

	flow := butterflymx.NewAuthFlowClient()

	flowStart := flow.Start()
	log.Println("Visit the following URL to authorize the application:")
	fmt.Println(flowStart.URL())

	scanner := bufio.NewScanner(os.Stdin)
	var token *oauth2.Token

	for {
		log.Println()
		log.Println("After authorizing, paste the full redirected URL here:")
		if !scanner.Scan() {
			if err := scanner.Err(); err != nil {
				log.Fatalf("failed to read input: %v", err)
			}
			return
		}

		pastedURL := strings.TrimSpace(scanner.Text())
		if pastedURL == "" {
			return
		}

		var err error
		token, err = flow.Finish(ctx, flowStart, pastedURL)
		if err != nil {
			log.Printf("failed to finish oauth2 auth flow: %v", err)
			continue
		}
		break
	}

	log.Println()
	log.Println("Successfully obtained OAuth2 token:")
	fmt.Println("oauth2_token:", token.AccessToken)

	loginClient := butterflymx.NewDenizenLoginClient(oauth2.StaticTokenSource(token))

	apiToken, err := loginClient.APIToken(ctx, true)
	if err != nil {
		log.Fatalf("failed to get API token: %v", err)
	}

	log.Println()
	log.Println("Successfully obtained API token:")
	fmt.Println("api_token:", apiToken)
}
