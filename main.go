package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

type TVShow struct {
	ID           int     `json:"id"`
	Name         string  `json:"name"`
	Overview     string  `json:"overview"`
	VoteAverage  float64 `json:"vote_average"`
	PosterPath   string  `json:"poster_path"`
	FirstAirDate string  `json:"first_air_date"`
}

type TVShowResponse struct {
	Page         int      `json:"page"`
	Results      []TVShow `json:"results"`
	TotalResults int      `json:"total_results"`
	TotalPages   int      `json:"total_pages"`
}

type TVShowDetail struct {
	TVShow
	NumberOfEpisodes int    `json:"number_of_episodes"`
	NumberOfSeasons  int    `json:"number_of_seasons"`
	Status           string `json:"status"`
}

type SearchRequest struct {
	Query string `json:"query" binding:"required"`
}

type IDRequest struct {
	ID int `json:"id" binding:"required"`
}

var (
	searchCacheFile  = "/app/cache/search_cache.json"
	detailsCacheFile = "/app/cache/details_cache.json"
	cacheMutex       sync.Mutex
)

func loadCache(file string, cache interface{}) error {
	cacheMutex.Lock()
	defer cacheMutex.Unlock()

	if _, err := os.Stat(file); os.IsNotExist(err) {
		return nil
	}

	data, err := os.ReadFile(file)
	if err != nil {
		return err
	}

	return json.Unmarshal(data, cache)
}

func saveCache(file string, cache interface{}) error {
	cacheMutex.Lock()
	defer cacheMutex.Unlock()

	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(file, data, 0644)
}

func makeRequest(url string, bearerToken string) ([]byte, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %v", err)
	}

	req.Header.Add("accept", "application/json")
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", bearerToken))

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error making request: %v", err)
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response: %v", err)
	}

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status code %d: %s", res.StatusCode, string(body))
	}

	return body, nil
}

func main() {
	envFile, err := godotenv.Read(".env")
	if err != nil {
		fmt.Printf("Error loading .env file: %v\n", err)
		return
	}

	bearerToken := envFile["TMDB_BEARER_TOKEN"]
	if bearerToken == "" {
		fmt.Println("TMDB_BEARER_TOKEN environment variable is required")
		return
	}

	searchCache := make(map[string]json.RawMessage)
	detailsCache := make(map[string]json.RawMessage)

	loadCache(searchCacheFile, &searchCache)
	loadCache(detailsCacheFile, &detailsCache)

	r := gin.Default()

	r.POST("/tv/search", func(c *gin.Context) {
		var request SearchRequest
		if err := c.ShouldBindJSON(&request); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		queryKey := strings.ReplaceAll(request.Query, " ", "")

		if cachedData, found := searchCache[queryKey]; found {
			c.JSON(http.StatusOK, json.RawMessage(cachedData))
			return
		}

		encodedQuery := url.QueryEscape(request.Query)
		url := fmt.Sprintf("https://api.themoviedb.org/3/search/tv?include_adult=false&language=en-US&page=1&query=%s", encodedQuery)

		body, err := makeRequest(url, bearerToken)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		searchCache[queryKey] = body
		saveCache(searchCacheFile, searchCache)

		c.JSON(http.StatusOK, json.RawMessage(body))
	})

	r.POST("/tv/details", func(c *gin.Context) {
		var request IDRequest
		if err := c.ShouldBindJSON(&request); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		idKey := fmt.Sprintf("%d", request.ID)

		if cachedData, found := detailsCache[idKey]; found {
			c.JSON(http.StatusOK, json.RawMessage(cachedData))
			return
		}

		url := fmt.Sprintf("https://api.themoviedb.org/3/tv/%d?language=en-US", request.ID)

		body, err := makeRequest(url, bearerToken)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		detailsCache[idKey] = body
		saveCache(detailsCacheFile, detailsCache)

		c.JSON(http.StatusOK, json.RawMessage(body))
	})

	r.Run(":8080")
}
