package search

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"net/http"
	"strings"
	"sync"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esapi"

	"villageconnect/internal/models"
	"villageconnect/internal/repository"
)

var ErrInvalidQuery = errors.New("invalid search query")

const searchResultLimit = 100

type SellerResult struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	VillageID string `json:"villageId,omitempty"`
	Verified  bool   `json:"verified"`
}

func (s *Service) bulkIndex(ctx context.Context, index string, documents []indexedDocument) error {
	if len(documents) == 0 {
		return nil
	}
	const batchSize = 500
	for start := 0; start < len(documents); start += batchSize {
		end := start + batchSize
		if end > len(documents) {
			end = len(documents)
		}
		if err := s.bulkIndexBatch(ctx, index, documents[start:end]); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) bulkIndexBatch(ctx context.Context, index string, documents []indexedDocument) error {
	var payload bytes.Buffer
	for _, document := range documents {
		meta, err := json.Marshal(map[string]interface{}{
			"index": map[string]string{"_index": index, "_id": document.ID},
		})
		if err != nil {
			return err
		}
		source, err := json.Marshal(document.Source)
		if err != nil {
			return err
		}
		payload.Write(meta)
		payload.WriteByte('\n')
		payload.Write(source)
		payload.WriteByte('\n')
	}
	response, err := esapi.BulkRequest{Body: &payload, Refresh: "wait_for"}.Do(ctx, s.getClient())
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode >= http.StatusBadRequest {
		return fmt.Errorf("bulk index %s: %s", index, response.Status())
	}
	var result struct {
		Errors bool `json:"errors"`
		Items  []map[string]struct {
			Status int             `json:"status"`
			Error  json.RawMessage `json:"error"`
		} `json:"items"`
	}
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return err
	}
	if result.Errors {
		for _, item := range result.Items {
			for _, outcome := range item {
				if outcome.Status >= http.StatusBadRequest {
					return fmt.Errorf("bulk index %s item failed (%d): %s", index, outcome.Status, outcome.Error)
				}
			}
		}
		return fmt.Errorf("bulk index %s reported item failures", index)
	}
	return nil
}

type Service struct {
	repository  repository.Repository
	mu          sync.RWMutex
	connectMu   sync.Mutex
	reindexMu   sync.Mutex
	client      *elasticsearch.Client
	endpoint    string
	index       string
	sellerIndex string
}

type indexedDocument struct {
	ID     string
	Source interface{}
}

func NewService(repo repository.Repository, endpoint, index string) *Service {
	service := &Service{
		repository: repo,
		endpoint:   strings.TrimSpace(endpoint),
		index:      strings.TrimSpace(index),
	}
	if service.endpoint == "" || service.index == "" {
		log.Printf("Elasticsearch endpoint or index is not configured; using MongoDB search")
		return service
	}
	service.sellerIndex = service.index + "_sellers"

	if err := service.connect(context.Background()); err != nil {
		log.Printf("Elasticsearch unavailable; using MongoDB search: %v", err)
		return service
	}
	if err := service.Reindex(context.Background()); err != nil {
		log.Printf("Elasticsearch reindex failed; MongoDB remains authoritative: %v", err)
		service.mu.Lock()
		service.client = nil
		service.mu.Unlock()
	}
	return service
}

func (s *Service) connect(ctx context.Context) error {
	s.connectMu.Lock()
	defer s.connectMu.Unlock()
	s.mu.RLock()
	connected := s.client != nil
	s.mu.RUnlock()
	if connected {
		return nil
	}
	if s.endpoint == "" || s.index == "" {
		return errors.New("Elasticsearch endpoint or index is not configured")
	}
	client, err := elasticsearch.NewClient(elasticsearch.Config{Addresses: []string{s.endpoint}})
	if err != nil {
		return fmt.Errorf("create Elasticsearch client: %w", err)
	}
	response, err := client.Ping()
	if err != nil {
		return fmt.Errorf("ping Elasticsearch: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("Elasticsearch returned %s", response.Status())
	}
	s.mu.Lock()
	s.client = client
	s.mu.Unlock()
	if err := s.ensureIndexes(ctx); err != nil {
		s.mu.Lock()
		s.client = nil
		s.mu.Unlock()
		return fmt.Errorf("ensure Elasticsearch indexes: %w", err)
	}
	return nil
}

func (s *Service) Enabled() bool {
	return s != nil && s.getClient() != nil
}

func (s *Service) getClient() *elasticsearch.Client {
	s.connectMu.Lock()
	defer s.connectMu.Unlock()
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.client
}

func (s *Service) Products(ctx context.Context, query string) ([]models.Product, error) {
	if len(strings.TrimSpace(query)) > 200 {
		return nil, ErrInvalidQuery
	}
	if !s.Enabled() {
		return s.repository.SearchProducts(ctx, strings.TrimSpace(query))
	}
	page, _, err := s.SearchProducts(ctx, models.ProductQuery{
		Query: query, Page: 1, PageSize: 20, Sort: "relevance",
	})
	return page.Products, err
}

func (s *Service) SearchProducts(ctx context.Context, query models.ProductQuery) (models.ProductPage, []string, error) {
	query.Query = strings.TrimSpace(query.Query)
	if len(query.Query) > 200 || len(query.Category) > 100 ||
		query.Page < 1 || query.PageSize < 1 || query.PageSize > searchResultLimit ||
		(query.MinPrice != nil && (*query.MinPrice < 0 || isInvalidNumber(*query.MinPrice))) ||
		(query.MaxPrice != nil && (*query.MaxPrice < 0 || isInvalidNumber(*query.MaxPrice))) ||
		(query.MinPrice != nil && query.MaxPrice != nil && *query.MinPrice > *query.MaxPrice) {
		return models.ProductPage{}, nil, ErrInvalidQuery
	}
	if query.VillageID != "" && query.VillageName == "" {
		villages, err := s.repository.ListVillages(ctx)
		if err != nil {
			return models.ProductPage{}, nil, err
		}
		name := ""
		found := false
		for _, village := range villages {
			if village.ID == query.VillageID {
				name = village.Name
				found = true
			}
		}
		if !found {
			return models.ProductPage{}, nil, ErrInvalidQuery
		}
		matches := 0
		for _, village := range villages {
			if strings.EqualFold(village.Name, name) {
				matches++
			}
		}
		if matches == 1 {
			query.VillageName = name
		}
	}
	if query.Sort == "" {
		query.Sort = "relevance"
	}
	switch query.Sort {
	case "relevance", "name_asc", "name_desc", "price_asc", "price_desc", "rating_desc":
	default:
		return models.ProductPage{}, nil, ErrInvalidQuery
	}

	if !s.Enabled() {
		page, err := s.repository.ListMarketplaceProducts(ctx, query)
		return page, collectSuggestions(page.Products), err
	}
	page, err := s.searchProductIndex(ctx, query)
	if err != nil {
		log.Printf("Elasticsearch product search failed; using MongoDB search: %v", err)
		page, err = s.repository.ListMarketplaceProducts(ctx, query)
		return page, collectSuggestions(page.Products), err
	}
	if page.Total == 0 {
		fallbackPage, fallbackErr := s.repository.ListMarketplaceProducts(ctx, query)
		if fallbackErr != nil {
			return models.ProductPage{}, nil, fallbackErr
		}
		return fallbackPage, collectSuggestions(fallbackPage.Products), nil
	}

	// Elasticsearch contains only a derived index. Re-read matching records from
	// MongoDB so returned product fields remain authoritative and deleted records
	// left by an interrupted indexing operation are never exposed.
	fresh := make([]models.Product, 0, len(page.Products))
	for _, indexedProduct := range page.Products {
		product, err := s.repository.GetProductByID(ctx, indexedProduct.ID)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				continue
			}
			return models.ProductPage{}, nil, err
		}
		if !matchesFreshProduct(*product, query) {
			continue
		}
		fresh = append(fresh, *product)
	}
	page.Products = fresh
	return page, collectSuggestions(fresh), nil
}

func matchesFreshProduct(product models.Product, query models.ProductQuery) bool {
	if query.VillageID != "" &&
		(product.VillageID != "" && product.VillageID != query.VillageID ||
			product.VillageID == "" && (query.VillageName == "" || !strings.EqualFold(product.Village, query.VillageName))) {
		return false
	}
	if query.Category != "" && !strings.EqualFold(product.Category, query.Category) {
		return false
	}
	if query.MinPrice != nil && product.Price < *query.MinPrice ||
		query.MaxPrice != nil && product.Price > *query.MaxPrice {
		return false
	}
	if query.Available != nil && *query.Available != (product.Available && product.Stock > 0) {
		return false
	}
	if query.Query == "" {
		return true
	}
	catalogText := strings.ToLower(strings.Join([]string{
		product.Name, product.Description, product.Category, product.Seller, product.Village,
	}, " "))
	for _, term := range strings.Fields(strings.ToLower(query.Query)) {
		if fuzzyTermMatches(catalogText, term) {
			return true
		}
	}
	return false
}

func fuzzyTermMatches(text, term string) bool {
	for _, word := range strings.Fields(text) {
		if strings.HasPrefix(word, term) || editDistanceAtMost(word, term, allowedEdits(term)) {
			return true
		}
	}
	return false
}

func allowedEdits(term string) int {
	if len(term) <= 2 {
		return 0
	}
	if len(term) <= 5 {
		return 1
	}
	return 2
}

func editDistanceAtMost(left, right string, limit int) bool {
	leftRunes, rightRunes := []rune(left), []rune(right)
	if len(leftRunes)-len(rightRunes) > limit || len(rightRunes)-len(leftRunes) > limit {
		return false
	}
	previous := make([]int, len(rightRunes)+1)
	for j := range previous {
		previous[j] = j
	}
	for i, leftRune := range leftRunes {
		current := make([]int, len(rightRunes)+1)
		current[0] = i + 1
		rowMinimum := current[0]
		for j, rightRune := range rightRunes {
			cost := 0
			if leftRune != rightRune {
				cost = 1
			}
			current[j+1] = min(
				current[j]+1,
				previous[j+1]+1,
				previous[j]+cost,
			)
			if current[j+1] < rowMinimum {
				rowMinimum = current[j+1]
			}
		}
		if rowMinimum > limit {
			return false
		}
		previous = current
	}
	return previous[len(rightRunes)] <= limit
}

func (s *Service) SearchSellers(ctx context.Context, query string) ([]SellerResult, error) {
	query = strings.TrimSpace(query)
	if len(query) > 200 {
		return nil, ErrInvalidQuery
	}
	if !s.Enabled() {
		return s.searchSellersInMongo(ctx, query)
	}

	boolQuery := map[string]interface{}{"filter": []interface{}{map[string]interface{}{"term": map[string]interface{}{"verified": true}}}}
	if query != "" {
		boolQuery["must"] = map[string]interface{}{
			"multi_match": map[string]interface{}{
				"query": query, "fields": []string{"name^3", "name.autocomplete"},
				"fuzziness": "AUTO", "prefix_length": 1,
			},
		}
	}
	body, err := json.Marshal(map[string]interface{}{
		"query": map[string]interface{}{"bool": boolQuery},
		"size":  20,
		"sort":  []interface{}{map[string]interface{}{"_score": "desc"}, map[string]interface{}{"name.keyword": "asc"}},
	})
	if err != nil {
		return nil, err
	}
	response, err := esapi.SearchRequest{Index: []string{s.sellerIndex}, Body: bytes.NewReader(body)}.Do(ctx, s.getClient())
	if err != nil {
		log.Printf("Elasticsearch seller search failed; using MongoDB: %v", err)
		return s.searchSellersInMongo(ctx, query)
	}
	defer response.Body.Close()
	if response.StatusCode >= http.StatusBadRequest {
		log.Printf("Elasticsearch seller search returned %s; using MongoDB", response.Status())
		return s.searchSellersInMongo(ctx, query)
	}
	var result struct {
		Hits struct {
			Hits []struct {
				Source SellerResult `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		log.Printf("Unable to decode Elasticsearch seller search; using MongoDB: %v", err)
		return s.searchSellersInMongo(ctx, query)
	}
	sellers := make([]SellerResult, 0, len(result.Hits.Hits))
	for _, hit := range result.Hits.Hits {
		user, err := s.repository.GetUserByID(ctx, hit.Source.ID)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				continue
			}
			return nil, err
		}
		if user.Role == models.RoleSeller && user.Verified {
			sellers = append(sellers, SellerResult{
				ID: user.ID, Name: user.Name, VillageID: user.VillageID, Verified: user.Verified,
			})
		}
	}
	if len(sellers) == 0 {
		return s.searchSellersInMongo(ctx, query)
	}
	return sellers, nil
}

func (s *Service) searchSellersInMongo(ctx context.Context, query string) ([]SellerResult, error) {
	users, err := s.repository.ListUsers(ctx)
	if err != nil {
		return nil, err
	}
	results := make([]SellerResult, 0)
	for _, user := range users {
		if user.Role == models.RoleSeller && user.Verified &&
			(query == "" || strings.Contains(strings.ToLower(user.Name), strings.ToLower(query))) {
			results = append(results, SellerResult{ID: user.ID, Name: user.Name,
				VillageID: user.VillageID, Verified: user.Verified})
		}
	}
	return results, nil
}

func (s *Service) IndexProduct(ctx context.Context, product models.Product) error {
	client := s.getClient()
	if client == nil {
		return errors.New("Elasticsearch unavailable; product was saved only in MongoDB")
	}
	payload, err := json.Marshal(product)
	if err != nil {
		return err
	}
	response, err := esapi.IndexRequest{
		Index: s.index, DocumentID: product.ID, Body: bytes.NewReader(payload), Refresh: "wait_for",
	}.Do(ctx, client)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode >= http.StatusBadRequest {
		return fmt.Errorf("index product %s: %s", product.ID, response.Status())
	}
	return nil
}

func (s *Service) DeleteProduct(ctx context.Context, productID string) error {
	client := s.getClient()
	if client == nil {
		return errors.New("Elasticsearch unavailable; product deletion was not synchronized")
	}
	response, err := esapi.DeleteRequest{
		Index: s.index, DocumentID: productID, Refresh: "wait_for",
	}.Do(ctx, client)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNotFound {
		return nil
	}
	if response.StatusCode >= http.StatusBadRequest {
		return fmt.Errorf("delete product %s from search index: %s", productID, response.Status())
	}
	return nil
}

func (s *Service) Reindex(ctx context.Context) (returnErr error) {
	s.reindexMu.Lock()
	defer s.reindexMu.Unlock()
	if err := s.connect(ctx); err != nil {
		return err
	}
	if !s.Enabled() {
		return errors.New("Elasticsearch is unavailable")
	}
	products, err := s.repository.ListProducts(ctx)
	if err != nil {
		return fmt.Errorf("load MongoDB products for reindex: %w", err)
	}
	users, err := s.repository.ListUsers(ctx)
	if err != nil {
		return fmt.Errorf("load MongoDB sellers for reindex: %w", err)
	}
	if err := s.clearIndex(ctx, s.index); err != nil {
		return err
	}
	if err := s.clearIndex(ctx, s.sellerIndex); err != nil {
		return err
	}
	productDocs := make([]indexedDocument, 0, len(products))
	for _, product := range products {
		productDocs = append(productDocs, indexedDocument{ID: product.ID, Source: product})
	}
	if err := s.bulkIndex(ctx, s.index, productDocs); err != nil {
		return err
	}
	sellerDocs := make([]indexedDocument, 0)
	for _, user := range users {
		if user.Role != models.RoleSeller || !user.Verified {
			continue
		}
		sellerDocs = append(sellerDocs, indexedDocument{
			ID:     user.ID,
			Source: SellerResult{ID: user.ID, Name: user.Name, VillageID: user.VillageID, Verified: user.Verified},
		})
	}
	if err := s.bulkIndex(ctx, s.sellerIndex, sellerDocs); err != nil {
		return err
	}
	if err := s.refresh(ctx, s.index); err != nil {
		return err
	}
	return s.refresh(ctx, s.sellerIndex)
}

func (s *Service) IndexSeller(ctx context.Context, user models.User) error {
	client := s.getClient()
	if client == nil {
		return errors.New("Elasticsearch unavailable; seller was not indexed")
	}
	seller := SellerResult{ID: user.ID, Name: user.Name, VillageID: user.VillageID, Verified: user.Verified}
	payload, err := json.Marshal(seller)
	if err != nil {
		return err
	}
	response, err := esapi.IndexRequest{
		Index: s.sellerIndex, DocumentID: user.ID, Body: bytes.NewReader(payload), Refresh: "wait_for",
	}.Do(ctx, client)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode >= http.StatusBadRequest {
		return fmt.Errorf("index seller %s: %s", user.ID, response.Status())
	}
	return nil
}

func (s *Service) searchProductIndex(ctx context.Context, query models.ProductQuery) (models.ProductPage, error) {
	client := s.getClient()
	if client == nil {
		return models.ProductPage{}, errors.New("Elasticsearch is unavailable")
	}
	body, err := productSearchRequest(query)
	if err != nil {
		return models.ProductPage{}, err
	}
	response, err := esapi.SearchRequest{Index: []string{s.index}, Body: bytes.NewReader(body)}.Do(ctx, client)
	if err != nil {
		return models.ProductPage{}, err
	}
	defer response.Body.Close()
	if response.StatusCode >= http.StatusBadRequest {
		return models.ProductPage{}, fmt.Errorf("Elasticsearch product search returned %s", response.Status())
	}
	var result struct {
		Hits struct {
			Total struct {
				Value int64 `json:"value"`
			} `json:"total"`
			Hits []struct {
				Source models.Product `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return models.ProductPage{}, err
	}
	products := make([]models.Product, 0, len(result.Hits.Hits))
	for _, hit := range result.Hits.Hits {
		products = append(products, hit.Source)
	}
	total := result.Hits.Total.Value
	totalPages := int((total + int64(query.PageSize) - 1) / int64(query.PageSize))
	return models.ProductPage{
		Products: products, Total: total, Page: query.Page, PageSize: query.PageSize, TotalPages: totalPages,
	}, nil
}

func productSearchRequest(query models.ProductQuery) ([]byte, error) {
	filter := make([]interface{}, 0, 5)
	if query.VillageID != "" {
		villageFilter := []interface{}{map[string]interface{}{"term": map[string]interface{}{"villageId": query.VillageID}}}
		if query.VillageName != "" {
			villageFilter = append(villageFilter, map[string]interface{}{
				"bool": map[string]interface{}{
					"must": []interface{}{
						map[string]interface{}{"term": map[string]interface{}{"village": query.VillageName}},
					},
					"must_not": []interface{}{map[string]interface{}{"exists": map[string]interface{}{"field": "villageId"}}},
				},
			})
		}
		filter = append(filter, map[string]interface{}{"bool": map[string]interface{}{"should": villageFilter, "minimum_should_match": 1}})
	}
	if query.Category != "" {
		filter = append(filter, map[string]interface{}{"term": map[string]interface{}{"category": query.Category}})
	}
	if query.MinPrice != nil || query.MaxPrice != nil {
		priceRange := map[string]interface{}{}
		if query.MinPrice != nil {
			priceRange["gte"] = *query.MinPrice
		}
		if query.MaxPrice != nil {
			priceRange["lte"] = *query.MaxPrice
		}
		filter = append(filter, map[string]interface{}{"range": map[string]interface{}{"price": priceRange}})
	}
	if query.Available != nil {
		if *query.Available {
			filter = append(filter,
				map[string]interface{}{"term": map[string]interface{}{"available": true}},
				map[string]interface{}{"range": map[string]interface{}{"stock": map[string]interface{}{"gt": 0}}})
		} else {
			filter = append(filter, map[string]interface{}{"bool": map[string]interface{}{
				"should": []interface{}{
					map[string]interface{}{"term": map[string]interface{}{"available": false}},
					map[string]interface{}{"range": map[string]interface{}{"stock": map[string]interface{}{"lte": 0}}},
				},
				"minimum_should_match": 1,
			}})
		}
	}
	boolQuery := map[string]interface{}{"filter": filter}
	if query.Query != "" {
		boolQuery["should"] = []interface{}{
			map[string]interface{}{"multi_match": map[string]interface{}{
				"query": query.Query, "fields": []string{"name^5", "description^2", "seller^3", "category^2", "village"},
				"type": "best_fields", "fuzziness": "AUTO", "prefix_length": 1, "operator": "or",
			}},
			map[string]interface{}{"multi_match": map[string]interface{}{
				"query":  query.Query,
				"fields": []string{"name.autocomplete", "name.autocomplete._2gram", "name.autocomplete._3gram"},
				"type":   "bool_prefix",
				"boost":  2,
			}},
		}
		boolQuery["minimum_should_match"] = 1
	}
	requestBody := map[string]interface{}{
		"from":  (query.Page - 1) * query.PageSize,
		"size":  query.PageSize,
		"query": map[string]interface{}{"bool": boolQuery},
	}
	switch query.Sort {
	case "name_asc":
		requestBody["sort"] = []interface{}{map[string]interface{}{"name.keyword": "asc"}}
	case "name_desc":
		requestBody["sort"] = []interface{}{map[string]interface{}{"name.keyword": "desc"}}
	case "price_asc":
		requestBody["sort"] = []interface{}{map[string]interface{}{"price": "asc"}}
	case "price_desc":
		requestBody["sort"] = []interface{}{map[string]interface{}{"price": "desc"}}
	case "rating_desc":
		requestBody["sort"] = []interface{}{map[string]interface{}{"rating": "desc"}}
	default:
		requestBody["sort"] = []interface{}{"_score", map[string]interface{}{"name.keyword": "asc"}}
	}
	return json.Marshal(requestBody)
}

func (s *Service) clearIndex(ctx context.Context, index string) error {
	body := strings.NewReader(`{"query":{"match_all":{}}}`)
	response, err := esapi.DeleteByQueryRequest{
		Index: []string{index}, Body: body, Refresh: boolPointer(true), Conflicts: "proceed",
	}.Do(ctx, s.getClient())
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode >= http.StatusBadRequest {
		return fmt.Errorf("clear search index %s: %s", index, response.Status())
	}
	return nil
}

func (s *Service) refresh(ctx context.Context, index string) error {
	response, err := esapi.IndicesRefreshRequest{Index: []string{index}}.Do(ctx, s.getClient())
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode >= http.StatusBadRequest {
		return fmt.Errorf("refresh search index %s: %s", index, response.Status())
	}
	return nil
}

func (s *Service) ensureIndexes(ctx context.Context) error {
	if err := s.ensureProductIndex(ctx); err != nil {
		return err
	}
	return s.ensureSellerIndex(ctx)
}

func (s *Service) ensureProductIndex(ctx context.Context) error {
	exists, err := s.indexExists(ctx, s.index)
	if err != nil {
		return err
	}
	if !exists {
		mapping := `{
			"settings": {"number_of_shards": 1, "number_of_replicas": 0},
			"mappings": {"properties": {
				"id": {"type": "keyword"},
				"name": {"type": "text", "fields": {
					"keyword": {"type": "keyword", "ignore_above": 256},
					"autocomplete": {"type": "search_as_you_type"}
				}},
				"description": {"type": "text"},
				"category": {"type": "keyword"},
				"village": {"type": "keyword"},
				"villageId": {"type": "keyword"},
				"seller": {"type": "keyword"},
				"sellerId": {"type": "keyword"},
				"price": {"type": "float"},
				"rating": {"type": "float"},
				"available": {"type": "boolean"},
				"stock": {"type": "integer"}
			}}
		}`
		return s.createIndex(ctx, s.index, mapping)
	}
	mapping := `{"properties":{
		"id":{"type":"keyword"},
		"name":{"type":"text","fields":{"keyword":{"type":"keyword","ignore_above":256},"autocomplete":{"type":"search_as_you_type"}}},
		"description":{"type":"text"},
		"category":{"type":"keyword"},
		"village":{"type":"keyword"},
		"villageId":{"type":"keyword"},
		"seller":{"type":"keyword"},
		"sellerId":{"type":"keyword"},
		"price":{"type":"float"},
		"rating":{"type":"float"},
		"available":{"type":"boolean"},
		"stock":{"type":"integer"}
	}}`
	return s.putMapping(ctx, s.index, mapping)
}

func (s *Service) ensureSellerIndex(ctx context.Context) error {
	exists, err := s.indexExists(ctx, s.sellerIndex)
	if err != nil {
		return err
	}
	mapping := `{
		"settings":{"number_of_shards":1,"number_of_replicas":0},
		"mappings":{"properties":{
			"id":{"type":"keyword"},
			"name":{"type":"text","fields":{"keyword":{"type":"keyword","ignore_above":256},"autocomplete":{"type":"search_as_you_type"}}},
			"villageId":{"type":"keyword"},
			"verified":{"type":"boolean"}
		}}
	}`
	if !exists {
		return s.createIndex(ctx, s.sellerIndex, mapping)
	}
	return s.putMapping(ctx, s.sellerIndex, `{"properties":{
		"id":{"type":"keyword"},
		"name":{"type":"text","fields":{"keyword":{"type":"keyword","ignore_above":256},"autocomplete":{"type":"search_as_you_type"}}},
		"villageId":{"type":"keyword"},
		"verified":{"type":"boolean"}
	}}`)
}

func (s *Service) indexExists(ctx context.Context, index string) (bool, error) {
	response, err := esapi.IndicesExistsRequest{Index: []string{index}}.Do(ctx, s.client)
	if err != nil {
		return false, err
	}
	defer response.Body.Close()
	switch response.StatusCode {
	case http.StatusOK:
		return true, nil
	case http.StatusNotFound:
		return false, nil
	default:
		return false, fmt.Errorf("check search index %s: %s", index, response.Status())
	}
}

func (s *Service) createIndex(ctx context.Context, index, mapping string) error {
	response, err := esapi.IndicesCreateRequest{Index: index, Body: strings.NewReader(mapping)}.Do(ctx, s.client)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode >= http.StatusBadRequest {
		return fmt.Errorf("create search index %s: %s", index, response.Status())
	}
	return nil
}

func (s *Service) putMapping(ctx context.Context, index, mapping string) error {
	response, err := esapi.IndicesPutMappingRequest{Index: []string{index}, Body: strings.NewReader(mapping)}.Do(ctx, s.client)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode >= http.StatusBadRequest {
		return fmt.Errorf("update search mapping %s: %s", index, response.Status())
	}
	return nil
}

func collectSuggestions(products []models.Product) []string {
	suggestions := make([]string, 0, len(products))
	for _, product := range products {
		if product.Name != "" {
			suggestions = append(suggestions, product.Name)
			if len(suggestions) == 5 {
				break
			}
		}
	}
	return suggestions
}

func isInvalidNumber(value float64) bool {
	return math.IsNaN(value) || math.IsInf(value, 0)
}

func boolPointer(value bool) *bool {
	return &value
}
