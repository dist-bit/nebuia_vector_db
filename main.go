package main

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/goccy/go-json"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"github.com/kataras/golog"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"gonum.org/v1/gonum/mat"
)

// Structs
type StoreRequest struct {
	CollectionName string     `json:"collection_name"`
	Documents      []Document `json:"documents"`
}

type ChunkData struct {
	Text          string    `json:"text"`
	Embedding     Embedding `json:"embedding"`
	Metadata      Metadata  `json:"metadata"`
	SemanticScore float64   `json:"semantic_score"`
}

type Embedding struct {
	Vector []float64 `json:"vector"`
}

type Metadata struct {
	Source interface{} `json:"source"`
	Name   string      `json:"name"`
}

type SearchRequest struct {
	CollectionName string    `json:"collection_name"`
	QueryEmbedding Embedding `json:"query_embedding"`
	TopK           int       `json:"top_k"`
}

type MultiSearchRequest struct {
	Collections    []string  `json:"collections"`
	QueryEmbedding Embedding `json:"query_embedding"`
	TopK           int       `json:"top_k"`
}

type Document struct {
	Text     string      `json:"text"`
	Metadata Metadata    `json:"metadata"`
	Chunks   []ChunkData `json:"chunks"`
}

type DeleteCollectionRequest struct {
	CollectionName string `json:"collection_name"`
}

type SearchResult struct {
	EmbeddingID    string    `json:"embedding_id"`
	Similarity     float64   `json:"similarity"`
	Position       int       `json:"position"`
	Metadata       *Metadata `json:"metadata,omitempty"`
	Text           string    `json:"text,omitempty"`
	CollectionName string    `json:"collection_name"`
}

// Global variables
var (
	minioClient     *minio.Client
	bucketName      string
	collectionLocks sync.Map
)

func loadEnvVariables() error {
	if err := godotenv.Load(); err != nil {
		return fmt.Errorf("error loading .env file: %v", err)
	}

	requiredVars := []string{
		"MINIO_ENDPOINT",
		"MINIO_ACCESS_KEY",
		"MINIO_SECRET_KEY",
		"MINIO_BUCKET_NAME",
	}

	for _, v := range requiredVars {
		if os.Getenv(v) == "" {
			return fmt.Errorf("required environment variable %s is not set", v)
		}
	}

	bucketName = os.Getenv("MINIO_BUCKET_NAME")
	return nil
}

func main() {
	// load config values
	if err := loadEnvVariables(); err != nil {
		golog.Fatal("Error loading environment variables: ", err)
	}
	// Setup fiber with optimized config
	app := fiber.New(fiber.Config{
		Prefork:      true,
		ServerHeader: "NebuIA",
		BodyLimit:    200 * 1024 * 1024,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		JSONEncoder:  json.Marshal,
		JSONDecoder:  json.Unmarshal,
		Concurrency:  256 * 1024,
	})

	setupMinioClient()
	setupRoutes(app)

	golog.Fatal(app.Listen("0.0.0.0:5489"))
}

func setupMinioClient() {
	var err error
	minioClient, err = minio.New(os.Getenv("MINIO_ENDPOINT"), &minio.Options{
		Creds: credentials.NewStaticV4(
			os.Getenv("MINIO_ACCESS_KEY"),
			os.Getenv("MINIO_SECRET_KEY"),
			"",
		),
		Secure: true,
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 100,
			IdleConnTimeout:     90 * time.Second,
		},
	})
	if err != nil {
		golog.Fatal("Error creating MinIO client: ", err)
	}

	// Ensure bucket exists
	ctx := context.Background()
	exists, err := minioClient.BucketExists(ctx, bucketName)
	if err != nil {
		golog.Fatal("Error checking bucket: ", err)
	}
	if !exists {
		err = minioClient.MakeBucket(ctx, bucketName, minio.MakeBucketOptions{})
		if err != nil {
			golog.Fatal("Error creating bucket: ", err)
		}
	}
}

func setupRoutes(app *fiber.App) {
	app.Post("/store", storeDocument)
	app.Post("/search", searchDocuments)
	app.Post("/multi_search", multiSearchDocuments)
	app.Post("/delete_collection", deleteCollection)
}

func getCollectionLock(collectionName string) *sync.Mutex {
	lock, _ := collectionLocks.LoadOrStore(collectionName, &sync.Mutex{})
	return lock.(*sync.Mutex)
}

func searchCollection(collectionName string, queryEmbedding []float64, topK int) ([]SearchResult, error) {
	// Pre-allocate results slice with capacity
	results := make([]SearchResult, 0, topK*2)

	// Normalize query embedding once
	queryNorm := mat.Norm(mat.NewVecDense(len(queryEmbedding), queryEmbedding), 2)
	normalizedQuery := make([]float64, len(queryEmbedding))
	for i := range queryEmbedding {
		normalizedQuery[i] = queryEmbedding[i] / queryNorm
	}

	ctx := context.Background()
	objectCh := minioClient.ListObjects(ctx, bucketName, minio.ListObjectsOptions{
		Prefix:    collectionName + "/",
		Recursive: true,
	})

	var (
		mu      sync.Mutex
		wg      sync.WaitGroup
		errChan = make(chan error, 1)
	)

	for object := range objectCh {
		if object.Err != nil {
			errChan <- object.Err
			continue
		}

		if !strings.HasSuffix(object.Key, "_doc.json") {
			continue
		}

		wg.Add(1)
		go func(objKey string) {
			defer wg.Done()

			doc, err := loadDocument(collectionName, strings.TrimSuffix(objKey[len(collectionName)+1:], "_doc.json"))
			if err != nil {
				select {
				case errChan <- err:
				default:
				}
				return
			}

			processDocument(doc, normalizedQuery, &results, &mu)
		}(object.Key)
	}

	wg.Wait()
	close(errChan)

	if err := <-errChan; err != nil {
		return nil, err
	}

	// Use sort.Slice with pre-allocated capacity
	if len(results) > topK {
		sort.Slice(results, func(i, j int) bool {
			return results[i].Similarity > results[j].Similarity
		})
		results = results[:topK]
	}

	return results, nil
}

func processDocument(doc Document, queryEmbedding []float64, results *[]SearchResult, mu *sync.Mutex) {
	localResults := make([]SearchResult, 0, len(doc.Chunks))

	for i, chunk := range doc.Chunks {
		similarity := dotProduct(queryEmbedding, chunk.Embedding.Vector)
		result := SearchResult{
			EmbeddingID:    doc.Metadata.Name,
			Similarity:     similarity,
			Position:       i + 1,
			Metadata:       &chunk.Metadata,
			Text:           chunk.Text,
			CollectionName: doc.Metadata.Name,
		}
		localResults = append(localResults, result)
	}

	mu.Lock()
	*results = append(*results, localResults...)
	mu.Unlock()
}

func dotProduct(a, b []float64) float64 {
	var sum float64
	for i := 0; i < len(a); i += 4 {
		if i+3 < len(a) {
			sum += a[i]*b[i] + a[i+1]*b[i+1] + a[i+2]*b[i+2] + a[i+3]*b[i+3]
		} else {
			for j := i; j < len(a); j++ {
				sum += a[j] * b[j]
			}
		}
	}
	return sum
}

func loadDocument(collectionName, embeddingID string) (Document, error) {
	objectName := fmt.Sprintf("%s/%s_doc.json", collectionName, embeddingID)
	obj, err := minioClient.GetObject(context.Background(), bucketName, objectName, minio.GetObjectOptions{})
	if err != nil {
		return Document{}, err
	}
	defer obj.Close()

	var doc Document
	err = json.NewDecoder(obj).Decode(&doc)
	if err != nil {
		return Document{}, err
	}

	return doc, nil
}

func storeDocument(c *fiber.Ctx) error {
	var req StoreRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	operationID := uuid.New().String()

	go func() {
		lock := getCollectionLock(req.CollectionName)
		lock.Lock()
		defer lock.Unlock()

		var wg sync.WaitGroup
		for _, doc := range req.Documents {
			wg.Add(1)
			go func(doc Document) {
				defer wg.Done()
				embeddingID, err := storeDocumentEfficient(req.CollectionName, doc)
				if err != nil {
					golog.Errorf("Error storing document: %v", err)
					return
				}
				golog.Infof("Storage operation %s completed. Document stored with ID %s", operationID, embeddingID)
			}(doc)
		}
		wg.Wait()
	}()

	return c.JSON(fiber.Map{
		"message":      fmt.Sprintf("Storage operation started for collection %s", req.CollectionName),
		"operation_id": operationID,
	})
}

func storeDocumentEfficient(collectionName string, doc Document) (string, error) {
	embeddingID := uuid.New().String()
	ctx := context.Background()

	// Store document text and metadata
	docObjectName := fmt.Sprintf("%s/%s_doc.json", collectionName, embeddingID)
	docJSON, err := json.Marshal(doc)
	if err != nil {
		return "", err
	}

	_, err = minioClient.PutObject(ctx, bucketName, docObjectName,
		bytes.NewReader(docJSON), int64(len(docJSON)),
		minio.PutObjectOptions{ContentType: "application/json"})

	if err != nil {
		return "", err
	}

	return embeddingID, nil
}

func searchDocuments(c *fiber.Ctx) error {
	var req SearchRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	lock := getCollectionLock(req.CollectionName)
	lock.Lock()
	defer lock.Unlock()

	results, err := searchCollection(req.CollectionName, req.QueryEmbedding.Vector, req.TopK)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(results)
}

func multiSearchDocuments(c *fiber.Ctx) error {
	var req MultiSearchRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	var allResults []SearchResult
	var wg sync.WaitGroup
	var mu sync.Mutex

	for _, collectionName := range req.Collections {
		wg.Add(1)
		go func(collection string) {
			defer wg.Done()
			results, err := searchCollection(collection, req.QueryEmbedding.Vector, req.TopK)
			if err != nil {
				golog.Errorf("Error searching collection %s: %v", collection, err)
				return
			}

			mu.Lock()
			allResults = append(allResults, results...)
			mu.Unlock()
		}(collectionName)
	}

	wg.Wait()

	if len(allResults) > req.TopK {
		sort.Slice(allResults, func(i, j int) bool {
			return allResults[i].Similarity > allResults[j].Similarity
		})
		allResults = allResults[:req.TopK]
	}

	return c.JSON(allResults)
}

func deleteCollection(c *fiber.Ctx) error {
	var req DeleteCollectionRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	go func() {
		ctx := context.Background()
		objectsCh := minioClient.ListObjects(ctx, bucketName, minio.ListObjectsOptions{
			Prefix:    req.CollectionName + "/",
			Recursive: true,
		})

		var wg sync.WaitGroup
		errChan := make(chan error, 100)

		for object := range objectsCh {
			if object.Err != nil {
				errChan <- object.Err
				continue
			}

			wg.Add(1)
			go func(objectKey string) {
				defer wg.Done()
				err := minioClient.RemoveObject(ctx, bucketName, objectKey, minio.RemoveObjectOptions{})
				if err != nil {
					errChan <- fmt.Errorf("failed to delete object %s: %v", objectKey, err)
				}
			}(object.Key)
		}

		wg.Wait()
		close(errChan)

		var errors []string
		for err := range errChan {
			errors = append(errors, err.Error())
		}

		if len(errors) > 0 {
			golog.Error("Errors during deletion of collection %s: %v", req.CollectionName, errors)
		} else {
			golog.Info("Collection %s deleted successfully", req.CollectionName)
		}
	}()

	return c.JSON(fiber.Map{
		"status":  "success",
		"message": fmt.Sprintf("Deletion of collection %s started", req.CollectionName),
	})
}
