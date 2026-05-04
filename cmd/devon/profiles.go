package main

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/ElioNeto/devon/internal/config"
	"github.com/spf13/cobra"
)

func newProfilesCommand() *cobra.Command {
	profilesCmd := &cobra.Command{
		Use:   "profiles",
		Short: "Gerencia perfis de provider (devon.toml)",
	}

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "Lista perfis configurados e status da API key",
		RunE:  runProfilesList,
	}

	testCmd := &cobra.Command{
		Use:   "test",
		Short: "Testa conectividade de cada perfil configurado",
		RunE:  runProfilesTest,
	}

	profilesCmd.AddCommand(listCmd, testCmd)
	return profilesCmd
}

func runProfilesList(_ *cobra.Command, _ []string) error {
	tc, err := config.LoadToml()
	if err != nil {
		return fmt.Errorf("falha ao carregar devon.toml: %w", err)
	}
	if tc == nil {
		fmt.Println("Nenhum devon.toml encontrado. Crie um a partir de .devon.toml.example")
		return nil
	}

	fmt.Println("Perfis configurados (devon.toml):")
	fmt.Println()

	for _, p := range tc.Profiles {
		keyStatus := "—"
		if p.APIKeyEnv != "" && os.Getenv(p.APIKeyEnv) != "" {
			keyStatus = "✔"
		}
		fmt.Printf("  ● %-8s %-12s %-34s key: %s\n", p.Name, p.Provider, p.Model, keyStatus)
	}

	fmt.Println()
	if tc.Defaults.Profile != "" {
		fmt.Printf("Padrão: %s\n", tc.Defaults.Profile)
	}

	return nil
}

// testSingleProfile tests connectivity for a single profile and returns pass/fail.
func testSingleProfile(p config.Profile, client *http.Client, passed *int) bool {
	url := p.BaseURL + "/models"

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		fmt.Printf("  %-8s → %-40s [FAIL] %s\n", p.Name, p.BaseURL, err.Error())
		return false
	}

	if apiKey := p.ResolveAPIKey(); apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("  %-8s → %-40s [FAIL] %s\n", p.Name, p.BaseURL, err.Error())
		return false
	}
	resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 400 {
		fmt.Printf("  %-8s → %-40s [PASS] HTTP %d\n", p.Name, p.BaseURL, resp.StatusCode)
		*passed++
		return true
	}
	fmt.Printf("  %-8s → %-40s [FAIL] HTTP %d\n", p.Name, p.BaseURL, resp.StatusCode)
	return false
}

func runProfilesTest(_ *cobra.Command, _ []string) error {
	tc, err := config.LoadToml()
	if err != nil {
		return fmt.Errorf("falha ao carregar devon.toml: %w", err)
	}
	if tc == nil {
		fmt.Println("Nenhum devon.toml encontrado. Crie um a partir de .devon.toml.example")
		return nil
	}

	fmt.Println("Testando perfis...")
	fmt.Println()

	passed := 0
	total := len(tc.Profiles)

	client := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12},
		},
	}

	// Track which profiles we've already tested to avoid duplicates
	tested := make(map[string]bool)

	for _, p := range tc.Profiles {
		testSingleProfile(p, client, &passed)
		tested[p.Name] = true
	}

	// Also test profiles referenced in [agent_routing] section
	if tc.AgentRouting != nil {
		routingProfiles := []struct {
			field string
			name  string
		}{
			{"explore_profile", tc.AgentRouting.ExploreProfile},
			{"plan_profile", tc.AgentRouting.PlanProfile},
			{"code_profile", tc.AgentRouting.CodeProfile},
		}
		for _, rp := range routingProfiles {
			if rp.name == "" || tested[rp.name] {
				continue
			}
			// Resolve the profile by name
			p, err := config.ResolveProfile(tc, rp.name)
			if err != nil {
				fmt.Printf("  [agent_routing] %s=%q → perfil não encontrado\n", rp.field, rp.name)
				continue
			}
			total++
			fmt.Printf("  [agent_routing] %s=%q:\n", rp.field, rp.name)
			testSingleProfile(*p, client, &passed)
			tested[p.Name] = true
		}
	}

	fmt.Println()
	fmt.Printf("Resultado: %d/%d perfis acessíveis.\n", passed, total)

	return nil
}
