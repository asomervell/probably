package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/asomervell/probably/internal/config"
	"github.com/asomervell/probably/internal/embedding"
	"github.com/asomervell/probably/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// EmbeddingBackfillCmd handles embedding generation for existing entities and transactions
type EmbeddingBackfillCmd struct {
	pool             *pgxpool.Pool
	embeddingService *embedding.Service
	entityStore      *models.EntityStore
	txnStore         *models.TransactionStore
}

// RunEmbeddingBackfill backfills embeddings for entities that don't have them
func RunEmbeddingBackfill(cfg *config.Config, batchSize int, maxItems int, target string) error {
	ctx := context.Background()

	// Connect to database
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer pool.Close()

	// Create embedding service
	embService, err := embedding.NewServiceFromConfig(ctx, cfg)
	if err != nil {
		return fmt.Errorf("failed to create embedding service: %w", err)
	}

	cmd := &EmbeddingBackfillCmd{
		pool:             pool,
		embeddingService: embService,
		entityStore:      models.NewEntityStore(pool),
		txnStore:         models.NewTransactionStore(pool),
	}

	switch target {
	case "entities":
		return cmd.backfillEntities(ctx, batchSize, maxItems)
	case "transactions":
		return cmd.backfillTransactions(ctx, batchSize, maxItems)
	case "all":
		if err := cmd.backfillEntities(ctx, batchSize, maxItems); err != nil {
			return err
		}
		return cmd.backfillTransactions(ctx, batchSize, maxItems)
	default:
		return fmt.Errorf("unknown target: %s (use 'entities', 'transactions', or 'all')", target)
	}
}

func (cmd *EmbeddingBackfillCmd) backfillEntities(ctx context.Context, batchSize, maxItems int) error {
	// Get count of entities without embeddings
	withEmb, withoutEmb, err := cmd.entityStore.CountEntitiesWithEmbedding(ctx)
	if err != nil {
		return fmt.Errorf("failed to count entities: %w", err)
	}

	log.Printf("Entities with embeddings: %d, without: %d", withEmb, withoutEmb)

	if withoutEmb == 0 {
		log.Println("All entities have embeddings")
		return nil
	}

	processed := 0
	for {
		if maxItems > 0 && processed >= maxItems {
			break
		}

		// Get batch of entities without embeddings
		entities, err := cmd.entityStore.GetEntitiesWithoutEmbedding(ctx, batchSize)
		if err != nil {
			return fmt.Errorf("failed to get entities: %w", err)
		}

		if len(entities) == 0 {
			break
		}

		// Generate embeddings in batch
		texts := make([]string, len(entities))
		for i, entity := range entities {
			texts[i] = embedding.BuildEntityText(&embedding.EntityEmbeddingInput{
				Name:        entity.Name,
				Type:        string(entity.Type),
				Subtype:     entity.Subtype,
				Description: entity.Description,
				Website:     entity.Website,
			})
		}

		embeddings, err := cmd.embeddingService.EmbedTexts(ctx, texts)
		if err != nil {
			log.Printf("Failed to generate embeddings for batch: %v", err)
			// Wait and retry
			time.Sleep(5 * time.Second)
			continue
		}

		// Store embeddings
		for i, entity := range entities {
			if i < len(embeddings) {
				err := cmd.entityStore.UpdateEmbedding(ctx, entity.ID, embeddings[i], cmd.embeddingService.Model())
				if err != nil {
					log.Printf("Failed to store embedding for entity %s: %v", entity.ID, err)
					continue
				}
				processed++
			}
		}

		log.Printf("Processed %d entities (total: %d)", len(entities), processed)

		// Small delay to avoid rate limiting
		time.Sleep(100 * time.Millisecond)
	}

	log.Printf("Entity embedding backfill complete. Processed %d entities.", processed)
	return nil
}

func (cmd *EmbeddingBackfillCmd) backfillTransactions(ctx context.Context, batchSize, maxItems int) error {
	// Get all ledgers first
	rows, err := cmd.pool.Query(ctx, "SELECT id FROM ledgers")
	if err != nil {
		return fmt.Errorf("failed to get ledgers: %w", err)
	}
	var ledgerIDs []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ledgerIDs = append(ledgerIDs, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return fmt.Errorf("ledger iteration: %w", err)
	}

	totalProcessed := 0
	for _, ledgerID := range ledgerIDs {
		if maxItems > 0 && totalProcessed >= maxItems {
			break
		}

		// Get count for this ledger
		withEmb, withoutEmb, err := cmd.txnStore.CountTransactionsWithEmbedding(ctx, ledgerID)
		if err != nil {
			log.Printf("Failed to count transactions for ledger %s: %v", ledgerID, err)
			continue
		}

		if withoutEmb == 0 {
			continue
		}

		log.Printf("Ledger %s: transactions with embeddings: %d, without: %d", ledgerID, withEmb, withoutEmb)

		processed := 0
		for {
			if maxItems > 0 && totalProcessed >= maxItems {
				break
			}

			// Get batch of transactions without embeddings
			txns, err := cmd.txnStore.GetTransactionsWithoutEmbedding(ctx, ledgerID, batchSize)
			if err != nil {
				log.Printf("Failed to get transactions: %v", err)
				break
			}

			if len(txns) == 0 {
				break
			}

			// Generate embeddings in batch
			texts := make([]string, len(txns))
			for i, txn := range txns {
				texts[i] = embedding.BuildTransactionText(&embedding.TransactionEmbeddingInput{
					Description:  txn.Description,
					DisplayTitle: txn.DisplayTitle,
					PatternType:  txn.PatternType,
				})
			}

			embeddings, err := cmd.embeddingService.EmbedTexts(ctx, texts)
			if err != nil {
				log.Printf("Failed to generate embeddings for batch: %v", err)
				time.Sleep(5 * time.Second)
				continue
			}

			// Store embeddings
			for i, txn := range txns {
				if i < len(embeddings) {
					err := cmd.txnStore.UpdateEmbedding(ctx, txn.ID, embeddings[i], cmd.embeddingService.Model())
					if err != nil {
						log.Printf("Failed to store embedding for transaction %s: %v", txn.ID, err)
						continue
					}
					processed++
					totalProcessed++
				}
			}

			log.Printf("Ledger %s: processed %d transactions", ledgerID, processed)
			time.Sleep(100 * time.Millisecond)
		}
	}

	log.Printf("Transaction embedding backfill complete. Processed %d transactions.", totalProcessed)
	return nil
}

// EmbeddingSubcommand runs the embedding subcommand
func EmbeddingSubcommand() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: util embedding <command>")
		fmt.Println("Commands:")
		fmt.Println("  backfill [target]  - Generate embeddings (target: entities, transactions, or all)")
		fmt.Println("  stats              - Show embedding statistics")
		fmt.Println("  similar <name>     - Find entities similar to a given name")
		os.Exit(1)
	}

	cfg := config.Load()
	if err := cfg.RequireDatabaseURL(); err != nil {
		log.Fatalf("%v", err)
	}

	subcmd := os.Args[2]
	switch subcmd {
	case "backfill":
		target := "all"
		if len(os.Args) >= 4 {
			target = os.Args[3]
		}
		batchSize := 50
		maxItems := 0 // 0 = no limit
		if err := RunEmbeddingBackfill(cfg, batchSize, maxItems, target); err != nil {
			log.Fatalf("Backfill failed: %v", err)
		}

	case "stats":
		if err := showEmbeddingStats(cfg); err != nil {
			log.Fatalf("Stats failed: %v", err)
		}

	case "similar":
		if len(os.Args) < 4 {
			fmt.Println("Usage: util embedding similar <entity-name>")
			os.Exit(1)
		}
		entityName := os.Args[3]
		if err := findSimilarEntities(cfg, entityName); err != nil {
			log.Fatalf("Similar search failed: %v", err)
		}

	default:
		fmt.Printf("Unknown subcommand: %s\n", subcmd)
		os.Exit(1)
	}
}

func showEmbeddingStats(cfg *config.Config) error {
	ctx := context.Background()

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer pool.Close()

	entityStore := models.NewEntityStore(pool)
	withEmb, withoutEmb, err := entityStore.CountEntitiesWithEmbedding(ctx)
	if err != nil {
		return fmt.Errorf("failed to count entities: %w", err)
	}

	total := withEmb + withoutEmb
	coverage := float64(0)
	if total > 0 {
		coverage = float64(withEmb) / float64(total) * 100
	}

	fmt.Printf("Entity Embedding Statistics\n")
	fmt.Printf("===========================\n")
	fmt.Printf("Total entities:        %d\n", total)
	fmt.Printf("With embeddings:       %d (%.1f%%)\n", withEmb, coverage)
	fmt.Printf("Without embeddings:    %d\n", withoutEmb)
	fmt.Println()

	// Transaction stats by ledger
	rows, err := pool.Query(ctx, "SELECT id, name FROM ledgers")
	if err != nil {
		return fmt.Errorf("failed to get ledgers: %w", err)
	}
	defer rows.Close()

	fmt.Printf("Transaction Embedding Statistics (by ledger)\n")
	fmt.Printf("=============================================\n")

	txnStore := models.NewTransactionStore(pool)
	for rows.Next() {
		var ledgerID uuid.UUID
		var name string
		if err := rows.Scan(&ledgerID, &name); err != nil {
			continue
		}

		withEmb, withoutEmb, err := txnStore.CountTransactionsWithEmbedding(ctx, ledgerID)
		if err != nil {
			continue
		}

		total := withEmb + withoutEmb
		coverage := float64(0)
		if total > 0 {
			coverage = float64(withEmb) / float64(total) * 100
		}

		fmt.Printf("\n%s:\n", name)
		fmt.Printf("  Total:           %d\n", total)
		fmt.Printf("  With embeddings: %d (%.1f%%)\n", withEmb, coverage)
		fmt.Printf("  Without:         %d\n", withoutEmb)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("ledger iteration: %w", err)
	}

	return nil
}

func findSimilarEntities(cfg *config.Config, entityName string) error {
	ctx := context.Background()

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer pool.Close()

	entityStore := models.NewEntityStore(pool)

	// Find the entity by name
	entity, err := entityStore.GetByName(ctx, entityName)
	if err != nil {
		return fmt.Errorf("entity not found: %v", err)
	}

	if len(entity.Embedding) == 0 {
		return fmt.Errorf("entity %q has no embedding - run 'util embedding backfill entities' first", entityName)
	}

	// Find similar entities
	similar, err := entityStore.FindSimilarToEntity(ctx, entity.ID, 10, 0.5)
	if err != nil {
		return fmt.Errorf("similarity search failed: %w", err)
	}

	fmt.Printf("Entities similar to %q:\n", entity.Name)
	fmt.Printf("========================\n")
	for i, result := range similar {
		fmt.Printf("%d. %s (%.1f%% similar) - %s\n",
			i+1, result.Entity.Name, result.Similarity*100, result.Entity.Type)
	}

	if len(similar) == 0 {
		fmt.Println("No similar entities found (try lowering minSimilarity)")
	}

	return nil
}
