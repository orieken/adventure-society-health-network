package asyncjobs

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"ashn/packages/ashnlog"
	"ashn/packages/domain"
	edimock "ashn/packages/edi-mock"
)

const (
	JobAuthReview        = "auth_review"
	JobClaimAdjudication = "claim_adjudication"
	JobClaimFinalization = "claim_finalization"

	StatusPending    = "pending"
	StatusProcessing = "processing"
	StatusCompleted  = "completed"
	StatusFailed     = "failed"
)

type Job struct {
	ID        string
	Type      string
	EntityID  string
	Status    string
	Attempts  int
	RunAfter  time.Time
	LastError string
	CreatedAt time.Time
	UpdatedAt time.Time
}

func Enqueue(db *sql.DB, jobType, entityID string, delay time.Duration) error {
	if db == nil {
		return nil
	}
	now := time.Now().UTC()
	_, err := db.Exec(
		`INSERT INTO transaction_jobs (id, job_type, entity_id, status, attempts, run_after, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, 0, $5, $6, $6)`,
		domain.NewID(), jobType, entityID, StatusPending, now.Add(delay), now,
	)
	return err
}

func List(db *sql.DB, limit int) ([]domain.TransactionJob, error) {
	if db == nil {
		return []domain.TransactionJob{}, nil
	}
	if limit <= 0 {
		limit = 25
	}
	if limit > 100 {
		limit = 100
	}
	rows, err := db.Query(
		`SELECT id, job_type, entity_id, status, attempts, run_after, COALESCE(last_error, ''), created_at, updated_at
		 FROM transaction_jobs
		 ORDER BY created_at DESC
		 LIMIT $1`,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	jobs := []domain.TransactionJob{}
	for rows.Next() {
		var job domain.TransactionJob
		if err := rows.Scan(&job.ID, &job.Type, &job.EntityID, &job.Status, &job.Attempts, &job.RunAfter, &job.LastError, &job.CreatedAt, &job.UpdatedAt); err != nil {
			return nil, err
		}
		job.DeadLetter = job.Status == StatusFailed
		jobs = append(jobs, job)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return jobs, nil
}

func Replay(db *sql.DB, id string) (domain.TransactionJob, error) {
	if db == nil {
		return domain.TransactionJob{}, errors.New("database is required")
	}
	var job domain.TransactionJob
	err := db.QueryRow(
		`UPDATE transaction_jobs
		 SET status = $1, attempts = 0, run_after = now(), last_error = NULL, updated_at = now()
		 WHERE id = $2 AND status = $3
		 RETURNING id, job_type, entity_id, status, attempts, run_after, COALESCE(last_error, ''), created_at, updated_at`,
		StatusPending, id, StatusFailed,
	).Scan(&job.ID, &job.Type, &job.EntityID, &job.Status, &job.Attempts, &job.RunAfter, &job.LastError, &job.CreatedAt, &job.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.TransactionJob{}, fmt.Errorf("dead-letter job not found")
	}
	if err != nil {
		return domain.TransactionJob{}, err
	}
	return job, nil
}

func ProcessDue(db *sql.DB, limit int) (int, error) {
	if db == nil {
		return 0, errors.New("database is required")
	}
	if limit <= 0 {
		limit = 5
	}

	processed := 0
	for processed < limit {
		job, ok, err := claimNextJob(db)
		if err != nil {
			return processed, err
		}
		if !ok {
			return processed, nil
		}
		if err := processJob(db, job); err != nil {
			markFailed(db, job, err)
			ashnlog.Error("async_job_failed", err, "jobId", job.ID, "jobType", job.Type, "entityId", job.EntityID)
		} else {
			markCompleted(db, job)
			ashnlog.Info("async_job_completed", "jobId", job.ID, "jobType", job.Type, "entityId", job.EntityID)
		}
		processed++
	}
	return processed, nil
}

func claimNextJob(db *sql.DB) (Job, bool, error) {
	tx, err := db.Begin()
	if err != nil {
		return Job{}, false, err
	}
	defer tx.Rollback()

	var job Job
	err = tx.QueryRow(
		`SELECT id, job_type, entity_id, status, attempts, run_after, COALESCE(last_error, ''), created_at, updated_at
		 FROM transaction_jobs
		 WHERE status = $1 AND run_after <= now()
		 ORDER BY run_after, created_at
		 FOR UPDATE SKIP LOCKED
		 LIMIT 1`,
		StatusPending,
	).Scan(&job.ID, &job.Type, &job.EntityID, &job.Status, &job.Attempts, &job.RunAfter, &job.LastError, &job.CreatedAt, &job.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Job{}, false, nil
	}
	if err != nil {
		return Job{}, false, err
	}
	if _, err := tx.Exec(`UPDATE transaction_jobs SET status = $1, attempts = attempts + 1, updated_at = now() WHERE id = $2`, StatusProcessing, job.ID); err != nil {
		return Job{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return Job{}, false, err
	}
	return job, true, nil
}

func processJob(db *sql.DB, job Job) error {
	switch job.Type {
	case JobAuthReview:
		return processAuthReview(db, job.EntityID)
	case JobClaimAdjudication:
		return processClaimAdjudication(db, job.EntityID)
	case JobClaimFinalization:
		return processClaimFinalization(db, job.EntityID)
	default:
		return fmt.Errorf("unsupported job type %q", job.Type)
	}
}

func processAuthReview(db *sql.DB, transactionID string) error {
	var serviceType string
	var severity domain.IncidentSeverity
	var currentStatus string
	err := db.QueryRow(`SELECT service_type, incident_severity, status FROM auth_requests WHERE transaction_id = $1`, transactionID).Scan(&serviceType, &severity, &currentStatus)
	if err != nil {
		return err
	}
	if currentStatus != string(domain.TxStatusPending) {
		return nil
	}

	decision := domain.TxStatusDenied
	if severity == domain.SeverityDiamond && strings.Contains(strings.ToLower(serviceType), "resurrection") {
		decision = domain.TxStatusApproved
	}

	if _, err := db.Exec(`UPDATE auth_requests SET status = $1 WHERE transaction_id = $2`, string(decision), transactionID); err != nil {
		return err
	}
	if _, err := db.Exec(`UPDATE claims SET authorization_status = $1, authorization_reason = $2 WHERE authorization_transaction_id = $3`, string(decision), autoReviewReason(decision), transactionID); err != nil {
		return err
	}
	_, err = db.Exec(`UPDATE transactions SET status = $1 WHERE id = $2 AND type = $3`, string(decision), transactionID, string(domain.Tx278))
	return err
}

func autoReviewReason(decision domain.TransactionStatus) string {
	if decision == domain.TxStatusApproved {
		return "Auto-approved by severity and service-type rule."
	}
	return "Auto-denied by severity and service-type rule."
}

func processClaimAdjudication(db *sql.DB, claimID string) error {
	result, err := db.Exec(`UPDATE claims SET status = $1 WHERE id = $2 AND status = $3`, string(domain.ClaimPending), claimID, string(domain.ClaimSubmitted))
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return nil
	}
	return Enqueue(db, JobClaimFinalization, claimID, 2*time.Second)
}

func processClaimFinalization(db *sql.DB, claimID string) error {
	var claim domain.Claim
	var context adjudicationContext
	err := db.QueryRow(
		`SELECT c.id, c.adventurer_id, c.provider_id, c.incident_severity, COALESCE(c.transaction_id, ''), COALESCE(c.authorization_transaction_id, ''), COALESCE(c.authorization_status, ''), COALESCE(c.authorization_reason, ''), c.amount_cents, c.status,
		        COALESCE(a.rank, ''), COALESCE(a.coverage_status, ''), COALESCE(p.tier_rank, '')
		 FROM claims c
		 LEFT JOIN adventurers a ON a.id = c.adventurer_id
		 LEFT JOIN providers p ON p.id = c.provider_id
		 WHERE c.id = $1`,
		claimID,
	).Scan(&claim.ID, &claim.AdventurerID, &claim.ProviderID, &claim.IncidentSeverity, &claim.TransactionID, &claim.AuthorizationTransactionID, &claim.AuthorizationStatus, &claim.AuthorizationReason, &claim.AmountCents, &claim.Status, &context.AdventurerRank, &context.CoverageStatus, &context.ProviderTier)
	if err != nil {
		return err
	}
	if claim.Status != domain.ClaimPending && claim.Status != domain.ClaimSubmitted {
		return nil
	}

	adjudicateClaimWithContext(&claim, context)

	statusTx := edimock.Generate277(claim.ID, claim.Status)
	statusTx.RelatedID = claim.TransactionID
	statusTx.Payload = domain.Payload(map[string]any{
		"x12": "277 Claim Status Response", "claimId": claim.ID, "claimStatus": claim.Status,
		"adjudication": map[string]any{
			"engine":                     "async-worker",
			"allowedAmountCents":         claim.AllowedAmountCents,
			"paidAmountCents":            claim.PaidAmountCents,
			"patientResponsibilityCents": claim.PatientResponsibilityCents,
			"adjustmentAmountCents":      claim.AdjustmentAmountCents,
			"adjustmentReason":           claim.AdjustmentReason,
			"denialReason":               claim.DenialReason,
			"coverageStatus":             context.CoverageStatus,
			"providerTier":               context.ProviderTier,
			"adventurerRank":             context.AdventurerRank,
		},
		"relatedId": claim.TransactionID,
	})

	if _, err := db.Exec(`UPDATE claims SET status = $1, allowed_amount_cents = $2, paid_amount_cents = $3, patient_responsibility_cents = $4, adjustment_amount_cents = $5, adjustment_reason = NULLIF($6, ''), denial_reason = NULLIF($7, '') WHERE id = $8`,
		string(claim.Status), claim.AllowedAmountCents, claim.PaidAmountCents, claim.PatientResponsibilityCents, claim.AdjustmentAmountCents, claim.AdjustmentReason, claim.DenialReason, claim.ID); err != nil {
		return err
	}
	_, err = db.Exec(
		`INSERT INTO transactions (id, type, status, sender_id, receiver_id, payload, raw_x12, related_id, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, NULLIF($8, ''), $9)
		 ON CONFLICT (id) DO NOTHING`,
		statusTx.ID, statusTx.Type, statusTx.Status, statusTx.SenderID, statusTx.ReceiverID, []byte(statusTx.Payload), statusTx.RawX12, statusTx.RelatedID, statusTx.CreatedAt,
	)
	return err
}

func adjudicateClaim(claim *domain.Claim) {
	adjudicateClaimWithContext(claim, adjudicationContext{})
}

type adjudicationContext struct {
	AdventurerRank domain.Rank
	CoverageStatus domain.CoverageStatus
	ProviderTier   domain.Rank
}

func adjudicateClaimWithContext(claim *domain.Claim, context adjudicationContext) {
	claim.Status = domain.ClaimApproved
	allowedPercent := int64(80)
	paidPercent := int64(85)
	adjustmentReason := "ASHN contractual allowance"

	if claim.IncidentSeverity == domain.SeverityNormal {
		allowedPercent = 90
		paidPercent = 90
	}

	allowedPercent += providerAllowedBoost(context.ProviderTier)
	paidPercent += providerPaidBoost(context.ProviderTier) + adventurerPaidBoost(context.AdventurerRank)
	if context.CoverageStatus == domain.CoveragePending {
		paidPercent -= 15
		adjustmentReason = "ASHN contractual allowance with pending benefits review"
	}
	if allowedPercent > 98 {
		allowedPercent = 98
	}
	if paidPercent > 98 {
		paidPercent = 98
	}
	if paidPercent < 0 {
		paidPercent = 0
	}

	claim.AllowedAmountCents = percentage(claim.AmountCents, allowedPercent)
	claim.PaidAmountCents = percentage(claim.AllowedAmountCents, paidPercent)
	claim.PatientResponsibilityCents = claim.AllowedAmountCents - claim.PaidAmountCents
	claim.AdjustmentAmountCents = claim.AmountCents - claim.AllowedAmountCents
	claim.AdjustmentReason = adjustmentReason
	claim.DenialReason = ""

	hasApprovedAuthorization := strings.EqualFold(claim.AuthorizationStatus, string(domain.TxStatusApproved))
	hasInactiveCoverage := context.CoverageStatus == domain.CoverageInactive || context.CoverageStatus == domain.CoverageSuspended
	if hasInactiveCoverage {
		denyClaim(claim, "Coverage not active", "Coverage inactive or suspended at service date")
		return
	}
	if (claim.IncidentSeverity == domain.SeverityDiamond || claim.AmountCents > 200000) && !hasApprovedAuthorization {
		denyClaim(claim, "Non-covered catastrophic encounter", "Prior authorization or benefit exception required")
		return
	}

	if hasApprovedAuthorization && claim.IncidentSeverity == domain.SeverityDiamond {
		claim.AdjustmentReason = "ASHN contractual allowance with approved prior authorization"
	}
}

func denyClaim(claim *domain.Claim, adjustmentReason, denialReason string) {
	claim.Status = domain.ClaimDenied
	claim.AllowedAmountCents = 0
	claim.PaidAmountCents = 0
	claim.PatientResponsibilityCents = 0
	claim.AdjustmentAmountCents = claim.AmountCents
	claim.AdjustmentReason = adjustmentReason
	claim.DenialReason = denialReason
}

func providerAllowedBoost(rank domain.Rank) int64 {
	switch rank {
	case domain.RankDiamond:
		return 5
	case domain.RankGold:
		return 3
	default:
		return 0
	}
}

func providerPaidBoost(rank domain.Rank) int64 {
	switch rank {
	case domain.RankDiamond:
		return 7
	case domain.RankGold:
		return 5
	case domain.RankSilver:
		return 2
	default:
		return 0
	}
}

func adventurerPaidBoost(rank domain.Rank) int64 {
	switch rank {
	case domain.RankDiamond:
		return 5
	case domain.RankGold:
		return 3
	case domain.RankSilver:
		return 1
	default:
		return 0
	}
}

func percentage(value int64, percent int64) int64 {
	return (value * percent) / 100
}

func markCompleted(db *sql.DB, job Job) {
	_, _ = db.Exec(`UPDATE transaction_jobs SET status = $1, updated_at = now() WHERE id = $2`, StatusCompleted, job.ID)
}

func markFailed(db *sql.DB, job Job, jobErr error) {
	status := StatusPending
	runAfter := time.Now().UTC().Add(time.Duration(job.Attempts+1) * time.Second)
	if job.Attempts >= 2 {
		status = StatusFailed
	}
	errorPayload, _ := json.Marshal(map[string]string{"error": jobErr.Error()})
	_, _ = db.Exec(
		`UPDATE transaction_jobs SET status = $1, run_after = $2, last_error = $3, updated_at = now() WHERE id = $4`,
		status, runAfter, string(errorPayload), job.ID,
	)
}
