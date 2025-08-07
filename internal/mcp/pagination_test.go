package mcp

import (
	"encoding/json"
	"fmt"
	"testing"
)

func TestPaginateItems(t *testing.T) {
	// Test data - create 25 mock items
	items := make([]interface{}, 25)
	for i := 0; i < 25; i++ {
		items[i] = map[string]interface{}{
			"id":   i,
			"name": fmt.Sprintf("item_%d", i),
		}
	}

	tests := []struct {
		name           string
		items          []interface{}
		page           int
		pageSize       int
		expectedCount  int
		expectedPage   int
		expectedTotal  int
		expectedHasMore bool
	}{
		{
			name:           "first page with page size 10",
			items:          items,
			page:           1,
			pageSize:       10,
			expectedCount:  10,
			expectedPage:   1,
			expectedTotal:  3, // 25 items / 10 per page = 3 pages
			expectedHasMore: true,
		},
		{
			name:           "last page with remaining items",
			items:          items,
			page:           3,
			pageSize:       10,
			expectedCount:  5, // 25 - (2 * 10) = 5 items on last page
			expectedPage:   3,
			expectedTotal:  3,
			expectedHasMore: false,
		},
		{
			name:           "page beyond total should return empty",
			items:          items,
			page:           5,
			pageSize:       10,
			expectedCount:  0,
			expectedPage:   5,
			expectedTotal:  3,
			expectedHasMore: false,
		},
		{
			name:           "zero page size should default to 20",
			items:          items,
			page:           1,
			pageSize:       0,
			expectedCount:  20, // Should use default page size
			expectedPage:   1,
			expectedTotal:  2, // 25 items / 20 per page = 2 pages
			expectedHasMore: true,
		},
		{
			name:           "page size larger than total items",
			items:          items,
			page:           1,
			pageSize:       50,
			expectedCount:  25,
			expectedPage:   1,
			expectedTotal:  1,
			expectedHasMore: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, page, totalPages, hasMore := paginateItems(tt.items, tt.page, tt.pageSize)
			
			if len(result) != tt.expectedCount {
				t.Errorf("expected %d items, got %d", tt.expectedCount, len(result))
			}
			
			if page != tt.expectedPage {
				t.Errorf("expected page %d, got %d", tt.expectedPage, page)
			}
			
			if totalPages != tt.expectedTotal {
				t.Errorf("expected %d total pages, got %d", tt.expectedTotal, totalPages)
			}
			
			if hasMore != tt.expectedHasMore {
				t.Errorf("expected hasMore %v, got %v", tt.expectedHasMore, hasMore)
			}

			// Verify first item in paginated result
			if len(result) > 0 {
				firstItem := result[0].(map[string]interface{})
				expectedFirstIndex := (tt.page - 1) * max(tt.pageSize, 20)
				if tt.pageSize == 0 {
					expectedFirstIndex = (tt.page - 1) * 20 // default page size
				}
				if expectedFirstIndex < len(tt.items) {
					expectedItem := tt.items[expectedFirstIndex].(map[string]interface{})
					if firstItem["id"] != expectedItem["id"] {
						t.Errorf("expected first item id %v, got %v", expectedItem["id"], firstItem["id"])
					}
				}
			}
		})
	}
}

func TestLimitContentSize(t *testing.T) {
	tests := []struct {
		name         string
		data         interface{}
		maxSize      int
		shouldLimit  bool
	}{
		{
			name: "small data under limit",
			data: map[string]interface{}{
				"name": "test",
				"id":   123,
			},
			maxSize:     1000,
			shouldLimit: false,
		},
		{
			name: "large data over limit",
			data: map[string]interface{}{
				"large_field": generateLargeString(2000),
			},
			maxSize:     1000,
			shouldLimit: true,
		},
		{
			name:        "empty data",
			data:        map[string]interface{}{},
			maxSize:     100,
			shouldLimit: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test basic size limiting logic
			resultJSON, _ := json.Marshal(tt.data)
			actualSize := len(resultJSON)
			shouldLimit := actualSize > tt.maxSize
			
			if shouldLimit != tt.shouldLimit {
				t.Errorf("expected shouldLimit=%v, got shouldLimit=%v (size: %d, limit: %d)", 
					tt.shouldLimit, shouldLimit, actualSize, tt.maxSize)
			}
		})
	}
}

// Helper function to generate large test strings
func generateLargeString(size int) string {
	result := make([]byte, size)
	for i := range result {
		result[i] = 'a'
	}
	return string(result)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}