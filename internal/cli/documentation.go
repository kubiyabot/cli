package cli

import (
	"fmt"
	"slices"
	"strings"

	"github.com/charmbracelet/glamour"
	"github.com/go-resty/resty/v2"
	"github.com/kubiyabot/cli/internal/config"
	"github.com/spf13/cobra"
)

const (
	kubiyaBaseTrieveURL = "https://leaves.mintlify.com/api/mcp/config/kubiya"
	groupSearchURL      = "https://api.mintlifytrieve.com/api/chunk_group/group_oriented_search"
)

type TrieveResponse struct {
	ApiKey       string `json:"trieveApiKey"`
	DatasetID    string `json:"trieveDatasetId"`
	Organization string `json:"name"`
}

type SearchRequest struct {
	Query          string  `json:"query"`
	PageSize       int     `json:"page_size,omitempty"`
	SearchType     string  `json:"search_type,omitempty"`
	ExtendResults  bool    `json:"extend_results,omitempty"`
	ScoreThreshold float64 `json:"score_threshold,omitempty"`
}

type GroupResults struct {
	Results []struct {
		Group struct {
			Name string `json:"name"`
		} `json:"group"`
		Chuncks []struct {
			Chunk Chunk `json:"chunk"`
		} `json:"chunks"`
	} `json:"results"`
}
type Chunk struct {
	Title       string   `json:"title"`
	HtmlContent string   `json:"chunk_html"`
	TagSets     []string `json:"tag_set"`
}

func (c *Chunk) String() string {
	return c.HtmlContent
}

func (c Chunk) isCode() bool {
	return slices.Contains(c.TagSets, "code")
}

func newDocumentationCommand(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "documentation",
		Aliases: []string{"doc", "docs"},
		Short:   "🔍 Query the central knowledge base",
		Long:    `Query the central knowledge base for contextual information with intelligent search capabilities.`,
	}

	cmd.AddCommand(
		newQueryDocumentationCommand(cfg),
	)

	return cmd
}

func GetTrieveConfig() (*TrieveResponse, error) {
	restyClient := resty.New()
	resp, err := restyClient.R().
		SetResult(&TrieveResponse{}).
		Get(kubiyaBaseTrieveURL)

	if err != nil {
		return nil, fmt.Errorf("failed to fetch Trieve config: %w", err)
	}

	if resp.IsError() {
		return nil, fmt.Errorf("error response from Trieve: %s", resp.String())
	}

	return resp.Result().(*TrieveResponse), nil
}

func (tr *TrieveResponse) SearchDocumentationByGroup(query string) (*GroupResults, error) {
	req := &SearchRequest{
		Query:          query,
		PageSize:       10,
		SearchType:     "fulltext",
		ExtendResults:  true,
		ScoreThreshold: 1.0,
	}
	var response GroupResults

	restyClient := resty.New()
	resp, err := restyClient.R().
		SetHeader("Content-Type", "application/json").
		SetHeader("Authorization", fmt.Sprintf("Bearer %s", tr.ApiKey)).
		SetHeader("TR-Dataset", tr.DatasetID).
		SetHeader("TR-Organization", tr.Organization).
		SetHeader("X-API-VERSION", "V2").
		SetBody(req).
		SetResult(&response).
		Post(groupSearchURL)

	if err != nil {
		return nil, fmt.Errorf("failed to search documentation: %w", err)
	}
	if resp.IsError() {
		return nil, fmt.Errorf("error response from Trieve: %s", resp.String())
	}
	if len(response.Results) == 0 {
		return nil, fmt.Errorf("no results found for query: %s", query)
	}

	return &response, nil
}

func newQueryDocumentationCommand(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "query [prompt]",
		Aliases: []string{"q", "search"},
		Short:   "🔍 Query the knowledge base",
		Long:    `Query the Kubiya documentation for information on commands, features, and usage.`,
		Args:    cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Join all arguments as the prompt
			prompt := strings.Join(args, " ")

			trieve, err := GetTrieveConfig()
			cobra.CheckErr(err)
			resp, err := trieve.SearchDocumentationByGroup(prompt)
			cobra.CheckErr(err)

			for _, chunks := range resp.Results {
				for _, chunk := range chunks.Chuncks {
					out, err := glamour.Render(chunk.Chunk.HtmlContent, "dark")
					if err != nil {
						return err
					}
					fmt.Print(out)
				}

			}
			return nil
		},
	}

	return cmd
}
