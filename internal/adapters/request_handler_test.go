package adapters

import (
	"testing"
	"time"

	"github.com/architeacher/svc-web-analyzer/internal/domain"
	"github.com/stretchr/testify/assert"
)

func TestRequestHandler_mapRequestOptionsToDomainOptions(t *testing.T) {
	t.Parallel()

	h := &RequestHandler{}

	tests := []struct {
		name     string
		input    *struct {
			CheckLinks      *bool `json:"check_links,omitempty"`
			DetectForms     *bool `json:"detect_forms,omitempty"`
			IncludeHeadings *bool `json:"include_headings,omitempty"`
			Timeout         *int  `json:"timeout,omitempty"`
		}
		expected domain.AnalysisOptions
	}{
		{
			name:  "nil options should use defaults",
			input: nil,
			expected: domain.AnalysisOptions{
				IncludeHeadings: true,
				CheckLinks:      true,
				DetectForms:     true,
				Timeout:         30 * time.Second,
			},
		},
		{
			name: "include_headings false should be respected",
			input: &struct {
				CheckLinks      *bool `json:"check_links,omitempty"`
				DetectForms     *bool `json:"detect_forms,omitempty"`
				IncludeHeadings *bool `json:"include_headings,omitempty"`
				Timeout         *int  `json:"timeout,omitempty"`
			}{
				IncludeHeadings: boolPtr(false),
			},
			expected: domain.AnalysisOptions{
				IncludeHeadings: false,
				CheckLinks:      true,
				DetectForms:     true,
				Timeout:         30 * time.Second,
			},
		},
		{
			name: "include_headings true should be respected",
			input: &struct {
				CheckLinks      *bool `json:"check_links,omitempty"`
				DetectForms     *bool `json:"detect_forms,omitempty"`
				IncludeHeadings *bool `json:"include_headings,omitempty"`
				Timeout         *int  `json:"timeout,omitempty"`
			}{
				IncludeHeadings: boolPtr(true),
			},
			expected: domain.AnalysisOptions{
				IncludeHeadings: true,
				CheckLinks:      true,
				DetectForms:     true,
				Timeout:         30 * time.Second,
			},
		},
		{
			name: "all options should be mapped correctly",
			input: &struct {
				CheckLinks      *bool `json:"check_links,omitempty"`
				DetectForms     *bool `json:"detect_forms,omitempty"`
				IncludeHeadings *bool `json:"include_headings,omitempty"`
				Timeout         *int  `json:"timeout,omitempty"`
			}{
				IncludeHeadings: boolPtr(false),
				CheckLinks:      boolPtr(false),
				DetectForms:     boolPtr(false),
				Timeout:         intPtr(60),
			},
			expected: domain.AnalysisOptions{
				IncludeHeadings: false,
				CheckLinks:      false,
				DetectForms:     false,
				Timeout:         60 * time.Second,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := h.mapRequestOptionsToDomainOptions(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func boolPtr(b bool) *bool {
	return &b
}

func intPtr(i int) *int {
	return &i
}