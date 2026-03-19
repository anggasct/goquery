package goquery

type PageMeta struct {
	Page       int   `json:"page"`
	Limit      int   `json:"limit"`
	Total      int64 `json:"total"`
	TotalPages int   `json:"totalPages"`
	HasNext    bool  `json:"hasNext"`
	HasPrev    bool  `json:"hasPrev"`
}

type PageResult[T any] struct {
	Items []T      `json:"items"`
	Meta  PageMeta `json:"meta"`
}

func BuildPageMeta(total int64, page, limit int) PageMeta {
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 10
	}

	totalPages := int((total + int64(limit) - 1) / int64(limit))
	return PageMeta{
		Page:       page,
		Limit:      limit,
		Total:      total,
		TotalPages: totalPages,
		HasNext:    page < totalPages,
		HasPrev:    page > 1,
	}
}
