package search

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"villageconnect/internal/models"
	"villageconnect/internal/repository"
)

type searchTestRepository struct {
	repository.Repository
	query   string
	product models.Product
	users   []models.User
}

func (r *searchTestRepository) SearchProducts(_ context.Context, query string) ([]models.Product, error) {
	r.query = query
	return []models.Product{{ID: "p1", Name: "Tomato"}}, nil
}

func (r *searchTestRepository) ListMarketplaceProducts(_ context.Context, query models.ProductQuery) (models.ProductPage, error) {
	r.query = query.Query
	return models.ProductPage{
		Products: []models.Product{{ID: "p1", Name: "Tomato"}},
		Total:    1, Page: query.Page, PageSize: query.PageSize, TotalPages: 1,
	}, nil
}

func (r *searchTestRepository) ListVillages(context.Context) ([]models.Village, error) {
	return []models.Village{{ID: "v1", Name: "Kudlu"}}, nil
}

func (r *searchTestRepository) ListCategories(context.Context) ([]models.Category, error) {
	return []models.Category{{ID: "c1", Name: "Vegetables"}, {ID: "c3", Name: "Grains"}}, nil
}

func (r *searchTestRepository) ListProducts(context.Context) ([]models.Product, error) {
	if r.product.ID == "" {
		r.product = models.Product{ID: "p1", Name: "Tomato"}
	}
	return []models.Product{r.product}, nil
}

func (r *searchTestRepository) GetProductByID(_ context.Context, id string) (*models.Product, error) {
	if r.product.ID == "" {
		r.product = models.Product{ID: "p1", Name: "Tomato"}
	}
	if id != r.product.ID {
		return nil, repository.ErrNotFound
	}
	product := r.product
	return &product, nil
}

func (r *searchTestRepository) ListUsers(context.Context) ([]models.User, error) {
	return r.users, nil
}

func (r *searchTestRepository) GetUserByID(_ context.Context, id string) (*models.User, error) {
	for _, user := range r.users {
		if user.ID == id {
			result := user
			return &result, nil
		}
	}
	return nil, repository.ErrNotFound
}

func TestProductsUsesRepositoryFallbackAndValidatesQuery(t *testing.T) {
	repo := &searchTestRepository{}
	service := NewService(repo, "", "")

	products, err := service.Products(context.Background(), "  tomato  ")
	if err != nil {
		t.Fatalf("Products returned error: %v", err)
	}
	if repo.query != "tomato" || len(products) != 1 {
		t.Fatalf("fallback result = query %q, products %#v", repo.query, products)
	}

	if _, err := service.Products(context.Background(), strings.Repeat("x", 201)); err != ErrInvalidQuery {
		t.Fatalf("long query error = %v, want %v", err, ErrInvalidQuery)
	}
}

func TestSearchProductsUsesMongoFallbackAndPreservesFilters(t *testing.T) {
	repo := &searchTestRepository{}
	service := NewService(repo, "", "")
	minimum, maximum := 10.0, 100.0
	available := true
	page, suggestions, err := service.SearchProducts(context.Background(), models.ProductQuery{
		Query: "  organic rice ", VillageID: "v1", Category: "Grains",
		MinPrice: &minimum, MaxPrice: &maximum, Available: &available,
		Page: 2, PageSize: 10,
	})
	if err != nil {
		t.Fatalf("SearchProducts returned error: %v", err)
	}
	if repo.query != "organic rice" || len(page.Products) != 1 ||
		page.Products[0].Name != "Tomato" || len(suggestions) != 1 || page.Page != 2 {
		t.Fatalf("unexpected MongoDB fallback result: page=%+v suggestions=%v query=%q", page, suggestions, repo.query)
	}
}

func TestProductSearchQuerySupportsFuzzyTextAutocompleteAndFilters(t *testing.T) {
	minimum, maximum := 10.0, 80.0
	available := true
	for _, queryText := range []string{"tomato", "tomatos", "organic rice", "fresh vegetables"} {
		body, err := productSearchRequest(models.ProductQuery{
			Query: queryText, Category: "Vegetables", VillageID: "v1", VillageName: "Kudlu",
			MinPrice: &minimum, MaxPrice: &maximum, Available: &available,
			Page: 1, PageSize: 8, Sort: "relevance",
		})
		if err != nil {
			t.Fatalf("productSearchRequest(%q) returned error: %v", queryText, err)
		}
		var request map[string]interface{}
		if err := json.Unmarshal(body, &request); err != nil {
			t.Fatal(err)
		}
		encoded := string(body)
		if !strings.Contains(encoded, `"fuzziness":"AUTO"`) ||
			!strings.Contains(encoded, `"name.autocomplete"`) ||
			!strings.Contains(encoded, `"category"`) ||
			!strings.Contains(encoded, `"villageId"`) ||
			!strings.Contains(encoded, `"price"`) ||
			!strings.Contains(encoded, `"available"`) ||
			!strings.Contains(encoded, `"_score"`) ||
			!strings.Contains(encoded, queryText) {
			t.Errorf("search query %q is missing fuzzy, autocomplete, filtering, or relevance clauses: %s", queryText, encoded)
		}
	}
}

func TestSearchQueryRejectsMalformedAndOversizedInputs(t *testing.T) {
	service := NewService(&searchTestRepository{}, "", "")
	for _, query := range []models.ProductQuery{
		{Query: strings.Repeat("x", 201), Page: 1, PageSize: 8},
		{MinPrice: floatPointer(30), MaxPrice: floatPointer(10), Page: 1, PageSize: 8},
		{Page: 0, PageSize: 8},
		{Page: 1, PageSize: 101},
	} {
		if _, _, err := service.SearchProducts(context.Background(), query); err != ErrInvalidQuery {
			t.Errorf("SearchProducts(%+v) error = %v, want %v", query, err, ErrInvalidQuery)
		}
	}
}

func TestElasticsearchSearchAndProductIndexSynchronization(t *testing.T) {
	repo := &searchTestRepository{
		product: models.Product{
			ID: "p1", Name: "Tomato", Category: "Vegetables", Village: "Kudlu",
			VillageID: "v1", SellerID: "s1", Seller: "Green Farm",
			Price: 30, Rating: 4.5, Available: true, Stock: 12,
		},
		users: []models.User{{ID: "s1", Name: "Green Farm", VillageID: "v1", Role: models.RoleSeller, Verified: true}},
	}
	var lastProductQuery string
	var indexed, deleted int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Elastic-Product", "Elasticsearch")
		switch {
		case r.URL.Path == "/":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"name":"test-es"}`))
		case r.Method == http.MethodHead:
			w.WriteHeader(http.StatusNotFound)
		case strings.HasSuffix(r.URL.Path, "/_search"):
			if strings.HasPrefix(r.URL.Path, "/products/") {
				body, _ := io.ReadAll(r.Body)
				lastProductQuery = string(body)
				source, _ := json.Marshal(repo.product)
				_, _ = fmt.Fprintf(w, `{"hits":{"total":{"value":1},"hits":[{"_source":%s}]}}`, source)
			} else {
				_, _ = w.Write([]byte(`{"hits":{"hits":[]}}`))
			}
		case strings.HasSuffix(r.URL.Path, "/_bulk"):
			_, _ = w.Write([]byte(`{"errors":false,"items":[]}`))
		case strings.HasSuffix(r.URL.Path, "/_delete_by_query"):
			_, _ = w.Write([]byte(`{"deleted":0}`))
		case strings.HasSuffix(r.URL.Path, "/_refresh"):
			_, _ = w.Write([]byte(`{"_shards":{"successful":1}}`))
		case strings.HasSuffix(r.URL.Path, "/_mapping"):
			_, _ = w.Write([]byte(`{"acknowledged":true}`))
		case r.Method == http.MethodPut && strings.Contains(r.URL.Path, "/_doc/"):
			indexed++
			_, _ = w.Write([]byte(`{"result":"created"}`))
		case r.Method == http.MethodPut:
			_, _ = w.Write([]byte(`{"acknowledged":true}`))
		case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/_doc/"):
			indexed++
			_, _ = w.Write([]byte(`{"result":"created"}`))
		case r.Method == http.MethodDelete:
			deleted++
			_, _ = w.Write([]byte(`{"result":"deleted"}`))
		default:
			http.Error(w, "unexpected Elasticsearch request", http.StatusNotFound)
		}
	}))
	defer server.Close()

	service := NewService(repo, server.URL, "products")
	if !service.Enabled() {
		t.Fatal("Elasticsearch service did not initialize against the test server")
	}
	for _, example := range []struct {
		query string
		name  string
	}{
		{query: "tomato", name: "Tomato"},
		{query: "tomatos", name: "Tomato"},
		{query: "organic rice", name: "Organic Rice"},
		{query: "fresh vegetables", name: "Fresh Vegetables"},
	} {
		repo.product.Name = example.name
		page, _, err := service.SearchProducts(context.Background(), models.ProductQuery{
			Query: example.query, Page: 1, PageSize: 8, Sort: "relevance",
		})
		if err != nil {
			t.Fatalf("search for %q returned error: %v", example.query, err)
		}
		if len(page.Products) != 1 || page.Products[0].Name != repo.product.Name {
			t.Fatalf("search for %q did not return MongoDB product: %+v", example.query, page)
		}
		if !strings.Contains(lastProductQuery, `"fuzziness":"AUTO"`) || !strings.Contains(lastProductQuery, example.query) {
			t.Errorf("Elasticsearch request for %q lacks fuzzy full-text query: %s", example.query, lastProductQuery)
		}
	}
	if err := service.IndexProduct(context.Background(), repo.product); err != nil {
		t.Fatalf("IndexProduct returned error: %v", err)
	}
	if err := service.DeleteProduct(context.Background(), repo.product.ID); err != nil {
		t.Fatalf("DeleteProduct returned error: %v", err)
	}
	if indexed != 1 || deleted != 1 {
		t.Fatalf("index synchronization calls = indexed %d, deleted %d", indexed, deleted)
	}
}

func floatPointer(value float64) *float64 {
	return &value
}
