package api

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const scanSummaryBackfillBatchSize = 100

// backfillVulnerabilityFixSummaries makes the fix filter work for scans that
// predate the with_fix/without_fix summary fields. It reads only the result
// array for documents that have never been backfilled and writes compact
// counters, so list endpoints can keep excluding large vulnerability arrays.
func (s *Server) backfillVulnerabilityFixSummaries(ctx context.Context) error {
	cursor, err := s.scans.Find(
		ctx,
		bson.M{
			"type":                        bson.M{"$in": []string{"sca", "host", "container"}},
			"summary.fix_status_computed": bson.M{"$ne": true},
		},
		options.Find().SetProjection(bson.M{"_id": 1, "vulnerabilities": 1}),
	)
	if err != nil {
		return fmt.Errorf("find scan summaries to backfill: %w", err)
	}
	defer closeCursor(ctx, cursor)

	models := make([]mongo.WriteModel, 0, scanSummaryBackfillBatchSize)
	flush := func() error {
		if len(models) == 0 {
			return nil
		}
		if _, err := s.scans.BulkWrite(
			ctx,
			models,
			options.BulkWrite().SetOrdered(false),
		); err != nil {
			return fmt.Errorf("backfill scan fix summaries: %w", err)
		}
		models = models[:0]
		return nil
	}

	for cursor.Next(ctx) {
		var scan struct {
			ID              bson.ObjectID   `bson:"_id"`
			Vulnerabilities []Vulnerability `bson:"vulnerabilities"`
		}
		if err := cursor.Decode(&scan); err != nil {
			return fmt.Errorf("decode scan summary backfill row: %w", err)
		}

		summary := buildSummary(nil, scan.Vulnerabilities)
		models = append(models, mongo.NewUpdateOneModel().
			SetFilter(bson.M{"_id": scan.ID}).
			SetUpdate(bson.M{"$set": bson.M{
				"summary.with_fix":            summary.WithFix,
				"summary.without_fix":         summary.WithoutFix,
				"summary.fix_status_computed": true,
			}}))
		if len(models) == scanSummaryBackfillBatchSize {
			if err := flush(); err != nil {
				return err
			}
		}
	}
	if err := cursor.Err(); err != nil {
		return fmt.Errorf("iterate scan summaries to backfill: %w", err)
	}

	return flush()
}
