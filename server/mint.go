package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"
)

// runTokenCommand implements the `token` subcommand, which mints an API token
// signed with ART_SESSION_SECRET. It exists so a deployment can hand out API
// credentials without enabling OIDC: the JWT it prints is accepted everywhere
// ART_API_TOKEN is. Only the token itself goes to stdout, so it can be captured
// with $(...); the human-readable summary goes to stderr.
func runTokenCommand(args []string) int {
	fs := flag.NewFlagSet("token", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	subject := fs.String("sub", "", "subject to stamp on the token (e.g. \"ci\")")
	name := fs.String("name", "", "optional human-readable name to store in the token")
	ttlHelp := fmt.Sprintf("how long the token stays valid (e.g. 720h, 30d); defaults to ART_SESSION_TTL (%s); 0 means never expires", sessionTTLLabel())
	ttlFlag := fs.String("ttl", "", ttlHelp)
	permsFlag := fs.String("perms", "", "comma-separated permissions to grant; the default is none, so the token can read but not change anything")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Mint an API token signed with ART_SESSION_SECRET.\n\nUsage:\n  upload_server token -sub <subject> [-ttl <duration>] [-name <name>] [-perms <list>]\n\nKnown permissions: %s\n\nFlags:\n", strings.Join(allPermissions, ", "))
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *subject == "" {
		fmt.Fprintln(os.Stderr, "error: -sub is required")
		fs.Usage()
		return 2
	}
	if !sessionAuthEnabled() {
		fmt.Fprintln(os.Stderr, "error: ART_SESSION_SECRET is not set, so no token can be signed")
		return 1
	}

	ttl := sessionTTL
	if *ttlFlag != "" {
		parsed, err := parseDuration(*ttlFlag)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: invalid -ttl: %v\n", err)
			return 2
		}
		ttl = parsed
	}

	now := time.Now()

	perms, err := parsePermissionList(*permsFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 2
	}

	token, err := mintSessionToken("mint", sessionIdentity{Subject: *subject, Name: *name}, perms, ttl, now)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	if ttl <= 0 {
		fmt.Fprintf(os.Stderr, "Minted a token for %q that never expires\n", *subject)
	} else {
		fmt.Fprintf(os.Stderr, "Minted a token for %q, valid for %s (until %s)\n", *subject, ttl, now.Add(ttl).Format(time.RFC3339))
	}
	fmt.Fprintf(os.Stderr, "Granted permissions: %s\n", listPermissions(perms))
	fmt.Println(token)

	return 0
}
