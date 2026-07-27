package literature

import (
	"context"
	"encoding/xml"
	"fmt"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/rs/zerolog"
)

// Paper represents a paper from arXiv or INSPIRE.
type Paper struct {
	ArxivID       string    `json:"arxiv_id"`
	Title         string    `json:"title"`
	Authors       []string  `json:"authors"`
	Abstract      string    `json:"abstract"`
	Categories    []string  `json:"categories"`
	PublishedDate time.Time `json:"published_date"`
	PDFURL        string    `json:"pdf_url"`
}

// SearchOptions configures an arXiv search query.
type SearchOptions struct {
	Categories  []string
	MaxResults  int
	DateFrom    *time.Time
	DateTo      *time.Time
	SortBy      string
	SortOrder   string
}

// ArxivClient wraps the arXiv API.
type ArxivClient struct {
	baseURL string
	client  *resty.Client
	log     *zerolog.Logger
}

// NewArxivClient creates a new arXiv API client.
func NewArxivClient(baseURL string, log *zerolog.Logger) *ArxivClient {
	client := resty.New().
		SetTimeout(30 * time.Second).
		SetRetryCount(3).
		SetRetryWaitTime(1 * time.Second).
		SetRetryMaxWaitTime(8 * time.Second)

	return &ArxivClient{
		baseURL: baseURL,
		client:  client,
		log:     log,
	}
}

// arXiv API XML response structures
type arxivFeed struct {
	XMLName xml.Name   `xml:"feed"`
	Entries []arxivEntry `xml:"entry"`
}

type arxivEntry struct {
	XMLName xml.Name `xml:"entry"`
	ID      string   `xml:"id"`
	Title   string   `xml:"title"`
	Summary string   `xml:"summary"`
	Updated string   `xml:"updated"`
	Authors []struct {
		Name string `xml:"name"`
	} `xml:"author"`
	Categories []struct {
		Term string `xml:"term,attr"`
	} `xml:"category"`
	Links []struct {
		Href string `xml:"href,attr"`
		Rel  string `xml:"rel,attr"`
		Type string `xml:"type,attr"`
	} `xml:"link"`
}

// Search queries the arXiv API for papers matching the query.
func (c *ArxivClient) Search(ctx context.Context, query string, opts SearchOptions) ([]Paper, error) {
	if opts.MaxResults == 0 {
		opts.MaxResults = 20
	}
	if opts.SortBy == "" {
		opts.SortBy = "relevance"
	}
	if opts.SortOrder == "" {
		opts.SortOrder = "descending"
	}

	resp, err := c.client.R().
		SetContext(ctx).
		SetQueryParam("search_query", fmt.Sprintf("all:%s", query)).
		SetQueryParam("start", "0").
		SetQueryParam("max_results", fmt.Sprintf("%d", opts.MaxResults)).
		SetQueryParam("sortBy", opts.SortBy).
		SetQueryParam("sortOrder", opts.SortOrder).
		Get(c.baseURL)
	if err != nil {
		return nil, fmt.Errorf("arxiv API request failed: %w", err)
	}
	if resp.IsError() {
		return nil, fmt.Errorf("arxiv API returned status %d: %s", resp.StatusCode(), resp.String())
	}

	var feed arxivFeed
	if err := xml.Unmarshal(resp.Body(), &feed); err != nil {
		return nil, fmt.Errorf("failed to parse arXiv XML: %w", err)
	}

	papers := make([]Paper, 0, len(feed.Entries))
	for _, entry := range feed.Entries {
		paper := Paper{
			Title:    cleanXMLWhitespace(entry.Title),
			Abstract: cleanXMLWhitespace(entry.Summary),
		}

		// Extract arXiv ID from the entry ID
		if id := extractArxivID(entry.ID); id != "" {
			paper.ArxivID = id
		}

		// Extract authors
		for _, a := range entry.Authors {
			paper.Authors = append(paper.Authors, a.Name)
		}

		// Extract categories
		for _, cat := range entry.Categories {
			paper.Categories = append(paper.Categories, cat.Term)
		}

		// Extract PDF URL
		for _, link := range entry.Links {
			if link.Type == "application/pdf" {
				paper.PDFURL = link.Href
			}
		}

		// Parse published date
		if t, err := time.Parse(time.RFC3339, entry.Updated); err == nil {
			paper.PublishedDate = t
		}

		papers = append(papers, paper)
	}

	c.log.Info().
		Str("query", query).
		Int("results", len(papers)).
		Msg("arXiv search complete")

	return papers, nil
}

// cleanXMLWhitespace removes excessive whitespace from arXiv XML fields.
func cleanXMLWhitespace(s string) string {
	// Simple whitespace cleanup; production version would use strings.Fields + join
	result := make([]rune, 0, len(s))
	inSpace := true
	for _, r := range s {
		if r == '\n' || r == '\t' || r == ' ' {
			if !inSpace {
				result = append(result, ' ')
				inSpace = true
			}
		} else {
			result = append(result, r)
			inSpace = false
		}
	}
	return string(result)
}

// extractArxivID parses an arXiv ID from a URL like
// http://arxiv.org/abs/2304.12345v1
func extractArxivID(url string) string {
	// Find the pattern NNNN.NNNNN
	for i := 0; i < len(url)-9; i++ {
		if url[i] >= '0' && url[i] <= '9' && url[i+4] == '.' {
			id := url[i : i+9]
			valid := true
			for _, c := range id {
				if !((c >= '0' && c <= '9') || c == '.') {
					valid = false
					break
				}
			}
			if valid {
				return id
			}
		}
	}
	return ""
}