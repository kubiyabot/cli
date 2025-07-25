package kubiya

import (
	"fmt"
	"slices"
	"strings"

	"github.com/go-resty/resty/v2"
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
	HtmlContent string   `json:"chunk_html"`
	TagSets     []string `json:"tag_set"`
	Metadata    struct {
		Title string `json:"title"`
		Icon  string `json:"icon"`
	} `json:"metadata"`
}

func (c *Chunk) String() string {
	if c.isCode() {
		return fmt.Sprintf("```%s\n```", c.HtmlContent)
	}
	return strings.ReplaceAll(c.HtmlContent, "\n", "\n\n")
}

func (c Chunk) isCode() bool {
	return slices.Contains(c.TagSets, "code")
}

func (c Chunk) StringTitle() string {
	return c.Metadata.Title
}

func (c Chunk) StringIcon() string {
	return c.Metadata.Icon
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
		SearchType:     "bm25",
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
