package main

import (
	"fmt"
	"os"

	"github.com/sipeed/picoclaw/pkg/pairing"
)

func maskCode(code string) string {
	if len(code) <= 2 {
		return "**"
	}
	return "**" + code[len(code)-2:]
}

func pairingCmd() {
	if len(os.Args) < 3 {
		pairingHelp()
		return
	}

	subcommand := os.Args[2]

	cfg, err := loadConfig()
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		os.Exit(1)
	}

	workspace := cfg.WorkspacePath()
	pm := pairing.NewPairingManager(workspace + "/pairing")

	fmt.Printf("[DEBUG] Storage path: %s/pairing\n", workspace)
	fmt.Printf("[DEBUG] Pending count: %d\n", len(pm.ListPending()))

	switch subcommand {
	case "list":
		pm.Cleanup()
		approved := pm.ListApproved()
		pending := pm.ListPending()

		fmt.Println("=== Approved Devices ===")
		if len(approved) == 0 {
			fmt.Println("No approved devices")
		}
		for _, a := range approved {
			fmt.Printf("  %s:%s (approved: %s)\n", a.Channel, a.SenderID, a.ApprovedAt.Format("2006-01-02 15:04"))
		}

		fmt.Println("\n=== Pending Requests ===")
		if len(pending) == 0 {
			fmt.Println("No pending requests")
		}
		for _, p := range pending {
			fmt.Printf(
				"  %s:%s (code: %s, expires: %s)\n",
				p.Channel, p.SenderID, maskCode(p.Code), p.ExpiresAt.Format("15:04"),
			)
		}

	case "approve":
		if len(os.Args) < 4 {
			fmt.Println("Usage: picoclaw pairing approve <code>")
			fmt.Println("       picoclaw pairing approve <channel> <sender_id> <code>")
			os.Exit(1)
		}

		// Support simplified approve with just code
		if len(os.Args) == 4 {
			code := os.Args[3]
			fmt.Printf("[DEBUG] Trying to approve with code: %s\n", code)
			fmt.Printf("[DEBUG] All pending codes:\n")
			for _, p := range pm.ListPending() {
				fmt.Printf("  - %s:%s = %s\n", p.Channel, p.SenderID, p.Code)
			}
			_, err := pm.ApproveByCode(code)
			if err != nil {
				fmt.Printf("Error: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("✓ Approved with code: %s\n", code)
			return
		}

		// Original approve with channel, sender_id, and code
		channel := os.Args[3]
		senderID := os.Args[4]
		code := os.Args[5]

		_, err := pm.Approve(channel, senderID, code)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("✓ Approved: %s:%s\n", channel, senderID)

	case "revoke":
		if len(os.Args) < 5 {
			fmt.Println("Usage: picoclaw pairing revoke <channel> <sender_id>")
			os.Exit(1)
		}
		channel := os.Args[3]
		senderID := os.Args[4]

		ok := pm.Revoke(channel, senderID)
		if ok {
			fmt.Printf("✓ Revoked: %s:%s\n", channel, senderID)
		} else {
			fmt.Printf("Not found: %s:%s\n", channel, senderID)
		}

	case "generate":
		if len(os.Args) < 5 {
			fmt.Println("Usage: picoclaw pairing generate <channel> <sender_id> [sender_name]")
			os.Exit(1)
		}
		channel := os.Args[3]
		senderID := os.Args[4]
		senderName := ""
		if len(os.Args) > 5 {
			senderName = os.Args[5]
		}

		code, err := pm.GenerateCode(channel, senderID, senderName)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("✓ Generated code for %s:%s\n", channel, senderID)
		fmt.Printf("  Code: %s (expires in 5 minutes)\n", code)

	default:
		pairingHelp()
	}
}

func pairingHelp() {
	fmt.Println("Usage: picoclaw pairing <command>")
	fmt.Println("")
	fmt.Println("Commands:")
	fmt.Println("  list                      List approved and pending pairings")
	fmt.Println("  approve <code>            Approve a pairing request using just the code")
	fmt.Println("  approve <channel> <id> <code>  Approve a pairing request (full form)")
	fmt.Println("  revoke <channel> <id>    Revoke an approved device")
	fmt.Println("  generate <channel> <id> [name]  Generate a pairing code")
	fmt.Println("")
	fmt.Println("Examples:")
	fmt.Println("  picoclaw pairing list")
	fmt.Println("  picoclaw pairing approve 123456")
	fmt.Println("  picoclaw pairing approve telegram 123456 123456")
	fmt.Println("  picoclaw pairing revoke telegram 123456")
	fmt.Println("  picoclaw pairing generate telegram 123456 John")
}
