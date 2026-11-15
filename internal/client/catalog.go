package client

import (
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

func (c *Client) Search(term string, limit int) ([]Product, error) {
	products, err := c.AllProducts()
	if err != nil {
		return nil, err
	}
	return SearchProducts(products, term, limit), nil
}

func SearchProducts(products []Product, term string, limit int) []Product {
	needle := fold(strings.TrimSpace(term))
	var out []Product
	for _, p := range products {
		if needle == "" || strings.Contains(fold(searchText(p)), needle) {
			out = append(out, p)
		}
	}
	SortProducts(out)
	return limitProducts(out, limit)
}

func (c *Client) AllProducts() ([]Product, error) {
	started := time.Now()
	if c.baseURL == BaseURL && c.store != nil {
		cache, err := c.store.LoadCatalogCache()
		if err == nil {
			age := time.Since(cache.SavedAt)
			if age >= 0 && age < 10*time.Minute {
				slog.Debug("catalog cache hit", "component", "client", "operation", "catalog_cache", "event_name", "catalog.cache.hit", "item_count", len(cache.Products), "age_ms", age.Milliseconds())
				return cache.Products, nil
			}
			slog.Debug("catalog cache stale", "component", "client", "operation", "catalog_cache", "event_name", "catalog.cache.stale", "age_ms", age.Milliseconds())
		} else {
			slog.Debug("catalog cache unavailable", "component", "client", "operation", "catalog_cache", "event_name", "catalog.cache.unavailable", "error_type", fmt.Sprintf("%T", err))
		}
	}
	out, complete, err := c.productPages("/products.json", 10)
	if err != nil {
		slog.Error("catalog fetch failed", "component", "client", "operation", "catalog_fetch", "event_name", "catalog.fetch.failed", "duration_ms", time.Since(started).Milliseconds(), "error_type", fmt.Sprintf("%T", err))
		return nil, err
	}
	slog.Debug("catalog fetch completed", "component", "client", "operation", "catalog_fetch", "event_name", "catalog.fetch.completed", "item_count", len(out), "complete", complete, "duration_ms", time.Since(started).Milliseconds())
	if complete && c.baseURL == BaseURL && c.store != nil {
		if err := c.store.SaveCatalogCache(CatalogCache{SavedAt: time.Now(), Products: out}); err != nil {
			slog.Warn("catalog cache save failed", "component", "client", "operation", "catalog_cache", "event_name", "catalog.cache.save_failed", "error_type", fmt.Sprintf("%T", err))
		}
	}
	return out, nil
}

func (c *Client) Collections() ([]Collection, error) {
	const maxPages = 10
	var out []Collection
	for page := 1; page <= maxPages; page++ {
		var cr collectionsResp
		u := fmt.Sprintf("%s/collections.json?limit=250&page=%d", c.baseURL, page)
		if err := c.doJSON(http.MethodGet, u, nil, &cr); err != nil {
			return nil, err
		}
		out = append(out, cr.Collections...)
		if len(cr.Collections) < 250 {
			return out, nil
		}
	}
	var cr collectionsResp
	u := fmt.Sprintf("%s/collections.json?limit=250&page=%d", c.baseURL, maxPages+1)
	if err := c.doJSON(http.MethodGet, u, nil, &cr); err != nil {
		return nil, err
	}
	if len(cr.Collections) > 0 {
		return nil, fmt.Errorf("collections pagination exceeded maximum of %d pages", maxPages)
	}
	return out, nil
}

func (c *Client) CollectionProducts(handle string, limit int) ([]Product, error) {
	out, complete, err := c.productPages("/collections/"+url.PathEscape(handle)+"/products.json", 100)
	if err != nil {
		return nil, err
	}
	if complete {
		SortProducts(out)
	}
	return limitProducts(out, limit), nil
}

func limitProducts(products []Product, limit int) []Product {
	if limit > 0 && len(products) > limit {
		return products[:limit]
	}
	return products
}

func (c *Client) productPages(path string, maxPages int) ([]Product, bool, error) {
	const pageSize = 250
	var out []Product
	for page := 1; page <= maxPages; page++ {
		var pr productsResp
		u := fmt.Sprintf("%s%s?limit=%d&page=%d", c.baseURL, path, pageSize, page)
		if err := c.doJSON(http.MethodGet, u, nil, &pr); err != nil {
			return nil, false, err
		}
		out = append(out, pr.Products...)
		if len(pr.Products) < pageSize {
			return out, true, nil
		}
	}
	var pr productsResp
	u := fmt.Sprintf("%s%s?limit=%d&page=%d", c.baseURL, path, pageSize, maxPages+1)
	if err := c.doJSON(http.MethodGet, u, nil, &pr); err != nil {
		return nil, false, err
	}
	if len(pr.Products) > 0 {
		return nil, false, fmt.Errorf("catalog pagination exceeded maximum of %d pages", maxPages)
	}
	return out, true, nil
}

func (c *Client) Product(ref string) (*Product, error) {
	if strings.IndexFunc(ref, func(r rune) bool { return r < '0' || r > '9' }) >= 0 {
		var p Product
		u := c.baseURL + "/products/" + url.PathEscape(ref) + ".js"
		if err := c.doJSON(http.MethodGet, u, nil, &p); err == nil {
			p.Type = firstNonEmpty(p.Type, productTypeFromTags(p.Tags))
			return &p, nil
		}
	}
	products, err := c.AllProducts()
	if err != nil {
		return nil, err
	}
	for _, p := range products {
		if productMatches(p, ref) {
			return &p, nil
		}
	}
	return nil, fmt.Errorf("product %q not found", ref)
}

func productMatches(p Product, ref string) bool {
	if strconv.FormatInt(p.ID, 10) == ref || p.Handle == ref {
		return true
	}
	for _, v := range p.Variants {
		if strconv.FormatInt(v.ID, 10) == ref || strings.EqualFold(v.SKU, ref) {
			return true
		}
	}
	return false
}

func productTypeFromTags(tags []string) string {
	if len(tags) == 0 {
		return ""
	}
	return tags[0]
}
