package adapters

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"

	"github.com/PuerkitoBio/goquery"
	"github.com/architeacher/svc-web-analyzer/internal/domain"
	"github.com/architeacher/svc-web-analyzer/internal/infrastructure"
)

type HTMLAnalyzer struct {
	logger infrastructure.Logger
}

func NewHTMLAnalyzer(logger infrastructure.Logger) *HTMLAnalyzer {
	return &HTMLAnalyzer{
		logger: logger,
	}
}

func (a *HTMLAnalyzer) Analyze(ctx context.Context, url, html string, options domain.AnalysisOptions) (*domain.AnalysisData, error) {
	results := &domain.AnalysisData{}
	var wg sync.WaitGroup
	var mu sync.Mutex

	wg.Go(func() {
		version := a.ExtractHTMLVersion(html)
		mu.Lock()
		results.HTMLVersion = version
		mu.Unlock()
	})

	wg.Go(func() {
		title := a.ExtractTitle(html)
		mu.Lock()
		results.Title = title
		mu.Unlock()
	})

	if options.IncludeHeadings {
		wg.Go(func() {
			headings := a.ExtractHeadingCounts(html)
			mu.Lock()
			results.HeadingCounts = headings
			mu.Unlock()
		})
	}

	wg.Go(func() {
		links, err := a.ExtractLinks(html, url)
		if err != nil {
			a.logger.Warn().Err(err).Str("url", url).Msg("failed to extract links during analysis")
			mu.Lock()
			results.Links = domain.LinkAnalysis{
				TotalCount:        0,
				InternalCount:     0,
				ExternalCount:     0,
				ExternalLinks:     []domain.Link{},
				InaccessibleLinks: []domain.InaccessibleLink{},
			}
			mu.Unlock()
		} else {
			linkAnalysis := domain.LinkAnalysis{
				TotalCount:        len(links),
				ExternalLinks:     []domain.Link{},
				InaccessibleLinks: []domain.InaccessibleLink{},
			}

			for _, link := range links {
				switch link.Type {
				case domain.LinkTypeInternal:
					linkAnalysis.InternalCount++
				case domain.LinkTypeExternal:
					linkAnalysis.ExternalCount++
					linkAnalysis.ExternalLinks = append(linkAnalysis.ExternalLinks, link)
				}
			}

			mu.Lock()
			results.Links = linkAnalysis
			mu.Unlock()
		}
	})

	if options.DetectForms {
		wg.Go(func() {
			forms := a.ExtractForms(html, url)
			mu.Lock()
			results.Forms = forms
			mu.Unlock()
		})
	}

	wg.Wait()

	return results, nil
}

func (a *HTMLAnalyzer) ExtractHTMLVersion(html string) domain.HTMLVersion {
	html = strings.TrimSpace(html)

	// Check for HTML5 doctype (case-insensitive)
	html5Regex := regexp.MustCompile(`(?i)<!DOCTYPE\s+html\s*>`)
	if html5Regex.MatchString(html) {
		return domain.HTML5
	}

	// Check for HTML 4.01 doctypes
	html401Patterns := []string{
		`(?i)<!DOCTYPE\s+html\s+PUBLIC\s+"-//W3C//DTD\s+HTML\s+4\.01//EN"`,
		`(?i)<!DOCTYPE\s+html\s+PUBLIC\s+"-//W3C//DTD\s+HTML\s+4\.01\s+Transitional//EN"`,
		`(?i)<!DOCTYPE\s+html\s+PUBLIC\s+"-//W3C//DTD\s+HTML\s+4\.01\s+Frameset//EN"`,
	}

	for _, pattern := range html401Patterns {
		if matched, _ := regexp.MatchString(pattern, html); matched {
			return domain.HTML401
		}
	}

	// Check for XHTML 1.0 doctypes
	xhtml10Patterns := []string{
		`(?i)<!DOCTYPE\s+html\s+PUBLIC\s+"-//W3C//DTD\s+XHTML\s+1\.0\s+Strict//EN"`,
		`(?i)<!DOCTYPE\s+html\s+PUBLIC\s+"-//W3C//DTD\s+XHTML\s+1\.0\s+Transitional//EN"`,
		`(?i)<!DOCTYPE\s+html\s+PUBLIC\s+"-//W3C//DTD\s+XHTML\s+1\.0\s+Frameset//EN"`,
	}

	for _, pattern := range xhtml10Patterns {
		if matched, _ := regexp.MatchString(pattern, html); matched {
			return domain.XHTML10
		}
	}

	// Check for XHTML 1.1 doctype
	xhtml11Pattern := `(?i)<!DOCTYPE\s+html\s+PUBLIC\s+"-//W3C//DTD\s+XHTML\s+1\.1//EN"`
	if matched, _ := regexp.MatchString(xhtml11Pattern, html); matched {
		return domain.XHTML11
	}

	// If no doctype found or unrecognized, check for XML declaration (might be XHTML)
	xmlDeclPattern := `(?i)<\?xml\s+version`
	if matched, _ := regexp.MatchString(xmlDeclPattern, html); matched {
		return domain.XHTML10
	}

	return domain.Unknown
}

func (a *HTMLAnalyzer) ExtractTitle(html string) string {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		a.logger.Error().Err(err).Msg("failed to parse HTML for title extraction")

		return ""
	}

	title := strings.TrimSpace(doc.Find("title").First().Text())
	title = regexp.MustCompile(`\s+`).ReplaceAllString(title, " ")

	return title
}

func (a *HTMLAnalyzer) ExtractHeadingCounts(html string) domain.HeadingCounts {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		a.logger.Error().Err(err).Msg("failed to parse HTML for heading extraction")

		return domain.HeadingCounts{}
	}

	counts := domain.HeadingCounts{}
	var wg sync.WaitGroup
	var mu sync.Mutex

	headingTypes := []struct {
		tag   string
		field *int
	}{
		{"h1", &counts.H1},
		{"h2", &counts.H2},
		{"h3", &counts.H3},
		{"h4", &counts.H4},
		{"h5", &counts.H5},
		{"h6", &counts.H6},
	}

	for _, h := range headingTypes {
		wg.Go(func() {
			count := doc.Find(h.tag).Length()
			mu.Lock()
			*h.field = count
			mu.Unlock()
		})
	}

	wg.Wait()

	a.logger.Debug().
		Int("h1", counts.H1).
		Int("h2", counts.H2).
		Int("h3", counts.H3).
		Int("h4", counts.H4).
		Int("h5", counts.H5).
		Int("h6", counts.H6).
		Msg("extracted heading counts")

	return counts
}

func (a *HTMLAnalyzer) ExtractLinks(html, baseURL string) ([]domain.Link, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		a.logger.Error().Err(err).Msg("failed to parse HTML for link extraction")

		return nil, err
	}

	baseURLParsed, err := url.Parse(baseURL)
	if err != nil {
		a.logger.Error().Err(err).Msg("failed to parse base URL")

		return nil, err
	}

	var links []domain.Link
	seen := make(map[string]bool)

	doc.Find("a[href]").Each(func(i int, s *goquery.Selection) {
		href, exists := s.Attr("href")
		if !exists || href == "" {
			return
		}

		parsedURL, err := url.Parse(href)
		if err != nil {
			a.logger.Debug().
				Err(err).
				Str("href", href).
				Msg("failed to parse link URL")
			return
		}

		resolvedURL := baseURLParsed.ResolveReference(parsedURL)
		finalURL := resolvedURL.String()

		if seen[finalURL] {
			return
		}
		seen[finalURL] = true

		if finalURL == "" ||
			strings.HasPrefix(href, "#") ||
			strings.HasPrefix(href, "javascript:") ||
			strings.HasPrefix(href, "mailto:") ||
			strings.HasPrefix(href, "tel:") {
			return
		}

		linkType := domain.LinkTypeExternal
		if resolvedURL.Host == baseURLParsed.Host {
			linkType = domain.LinkTypeInternal
		}

		links = append(links, domain.Link{
			URL:  finalURL,
			Type: linkType,
		})
	})

	a.logger.Debug().
		Int("total_links", len(links)).
		Str("base_url", baseURL).
		Msg("extracted links")

	return links, nil
}

func (a *HTMLAnalyzer) ExtractForms(html string, baseURL string) domain.FormAnalysis {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		a.logger.Error().Err(err).Msg("failed to parse HTML for form extraction")

		return domain.FormAnalysis{}
	}

	baseURLParsed, err := url.Parse(baseURL)
	if err != nil {
		a.logger.Error().Err(err).Msg("failed to parse base URL for form analysis")

		return domain.FormAnalysis{}
	}

	var loginForms []domain.LoginForm
	totalForms := 0

	doc.Find("form").Each(func(i int, s *goquery.Selection) {
		totalForms++

		method := strings.ToUpper(strings.TrimSpace(s.AttrOr("method", http.MethodGet)))
		if method != strings.ToUpper(http.MethodPost) && method != strings.ToUpper(http.MethodGet) {
			method = strings.ToUpper(http.MethodGet)
		}

		action := s.AttrOr("action", "")
		if action != "" {
			if parsedAction, err := url.Parse(action); err == nil {
				resolvedAction := baseURLParsed.ResolveReference(parsedAction)
				action = resolvedAction.String()
			}
		}

		var fields []string
		fieldNames := make(map[string]bool)

		s.Find("input, select, textarea").Each(func(j int, field *goquery.Selection) {
			name := field.AttrOr("name", "")
			if name != "" && !fieldNames[name] {
				fields = append(fields, name)
				fieldNames[name] = true
			}
		})

		if a.isLikelyLoginForm(method, s) {
			loginForm := domain.LoginForm{
				Method: domain.FormMethod(method),
				Action: action,
				Fields: fields,
			}
			loginForms = append(loginForms, loginForm)
		}
	})

	analysis := domain.FormAnalysis{
		TotalCount:         totalForms,
		LoginFormsDetected: len(loginForms),
		LoginFormDetails:   loginForms,
	}

	a.logger.Debug().
		Int("total_forms", totalForms).
		Int("login_forms", len(loginForms)).
		Msg("extracted form analysis")

	return analysis
}

func (a *HTMLAnalyzer) isLikelyLoginForm(method string, formSelection *goquery.Selection) bool {
	if method != strings.ToUpper(http.MethodPost) {
		return false
	}

	hasPasswordInput := false
	formSelection.Find(fmt.Sprintf("input[type='%s']", domain.InputTypePassword)).Each(func(i int, s *goquery.Selection) {
		hasPasswordInput = true
	})

	return hasPasswordInput
}
