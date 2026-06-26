package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"ashn/packages/domain"

	"github.com/spf13/cobra"
)

var rawJSON bool

func main() {
	if err := rootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "ashn:", err)
		os.Exit(1)
	}
}

func rootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ashn",
		Short: "ASHN command-line scribe",
	}
	cmd.PersistentFlags().BoolVar(&rawJSON, "json", false, "print raw JSON")
	cmd.AddCommand(
		enrollCmd(),
		checkEligibilityCmd(),
		requestAuthCmd(),
		submitClaimCmd(),
		claimStatusCmd(),
		providersCmd(),
		configCmd(),
	)
	return cmd
}

func enrollCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "enroll <name> <rank> <guild> <region>",
		Short: "Send an 834 enrollment transaction",
		Args:  cobra.ExactArgs(4),
		RunE: func(_ *cobra.Command, args []string) error {
			req := domain.EnrollmentRequest{Name: args[0], Rank: domain.Rank(args[1]), Guild: args[2], Region: domain.Region(args[3])}
			return call(http.MethodPost, "/v1/adventurers", req)
		},
	}
}

func checkEligibilityCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "check-eligibility <adventurer-id> <provider-id>",
		Short: "Send a 270 eligibility inquiry and display the 271 response",
		Args:  cobra.ExactArgs(2),
		RunE: func(_ *cobra.Command, args []string) error {
			return call(http.MethodPost, "/v1/eligibility", domain.EligibilityRequest{AdventurerID: args[0], ProviderID: args[1]})
		},
	}
}

func requestAuthCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "request-auth <adventurer-id> <provider-id> <service-type>",
		Short: "Send a 278 prior authorization request",
		Args:  cobra.ExactArgs(3),
		RunE: func(_ *cobra.Command, args []string) error {
			req := domain.PriorAuthRequest{AdventurerID: args[0], ProviderID: args[1], ServiceType: args[2], IncidentSeverity: domain.SeverityDiamond}
			return call(http.MethodPost, "/v1/auth-requests", req)
		},
	}
}

func submitClaimCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "submit-claim <adventurer-id> <provider-id> <severity>",
		Short: "Send an 837 claim",
		Args:  cobra.ExactArgs(3),
		RunE: func(_ *cobra.Command, args []string) error {
			req := domain.ClaimRequest{AdventurerID: args[0], ProviderID: args[1], IncidentSeverity: normalizeSeverity(args[2]), AmountCents: 125000}
			return call(http.MethodPost, "/v1/claims", req)
		},
	}
}

func claimStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "claim-status <claim-id>",
		Short: "Send a 276 claim status inquiry",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			return call(http.MethodGet, "/v1/claims/"+args[0]+"/status", nil)
		},
	}
}

func providersCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "providers", Short: "Inspect providers"}
	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List all registered providers",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return call(http.MethodGet, "/v1/providers", nil)
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "show <id>",
		Short: "Show a provider",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			return call(http.MethodGet, "/v1/providers/"+args[0], nil)
		},
	})
	return cmd
}

func configCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "config", Short: "Configure ASHN CLI"}
	cmd.AddCommand(&cobra.Command{
		Use:   "set-url <url>",
		Short: "Save API gateway URL",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			path, err := configPath()
			if err != nil {
				return err
			}
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				return err
			}
			if err := os.WriteFile(path, []byte("url: "+args[0]+"\n"), 0o600); err != nil {
				return err
			}
			fmt.Println("ASHN API URL saved:", args[0])
			return nil
		},
	})
	return cmd
}

func call(method, path string, body any) error {
	var reader io.Reader
	if body != nil {
		payload, _ := json.Marshal(body)
		reader = bytes.NewReader(payload)
	}
	req, err := http.NewRequest(method, strings.TrimRight(configuredURL(), "/")+path, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	bytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if rawJSON {
		fmt.Println(string(bytes))
		return nil
	}
	var decoded map[string]any
	if err := json.Unmarshal(bytes, &decoded); err != nil {
		fmt.Println(string(bytes))
		return nil
	}
	if loreText, ok := decoded["lore"].(string); ok && loreText != "" {
		fmt.Println("Lore:", loreText)
	}
	pretty, _ := json.MarshalIndent(decoded, "", "  ")
	fmt.Println(string(pretty))
	return nil
}

func configuredURL() string {
	if value := os.Getenv("ASHN_API_URL"); value != "" {
		return value
	}
	path, err := configPath()
	if err == nil {
		if bytes, readErr := os.ReadFile(path); readErr == nil {
			for _, line := range strings.Split(string(bytes), "\n") {
				if strings.HasPrefix(line, "url:") {
					return strings.TrimSpace(strings.TrimPrefix(line, "url:"))
				}
			}
		}
	}
	return "http://localhost:8080"
}

func configPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		home, homeErr := os.UserHomeDir()
		if homeErr != nil {
			return "", homeErr
		}
		dir = home
	}
	return filepath.Join(dir, "ashn", "config.yaml"), nil
}

func normalizeSeverity(value string) domain.IncidentSeverity {
	switch strings.ToLower(value) {
	case "normal":
		return domain.SeverityNormal
	case "awakened":
		return domain.SeverityAwakened
	case "diamond":
		return domain.SeverityDiamond
	default:
		return domain.IncidentSeverity(value)
	}
}
