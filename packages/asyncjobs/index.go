package asyncjobs

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

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
			log.Printf("[ASHN] async job failed id=%s type=%s entity=%s err=%v", job.ID, job.Type, job.EntityID, err)
		} else {
			markCompleted(db, job)
			log.Printf("[ASHN] async job completed id=%s type=%s entity=%s", job.ID, job.Type, job.EntityID)
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
	err := db.QueryRow(`SELECT service_type, incident_severity FROM auth_requests WHERE transaction_id = $1`, transactionID).Scan(&serviceType, &severity)
	if err != nil {
		return err
	}

	decision := domain.TxStatusDenied
	if severity == domain.SeverityDiamond && strings.Contains(strings.ToLower(serviceType), "resurrection") {
		decision = domain.TxStatusApproved
	}

	if _, err := db.Exec(`UPDATE auth_requests SET status = $1 WHERE transaction_id = $2`, string(decision), transactionID); err != nil {
		return err
	}
	_, err = db.Exec(`UPDATE transactions SET status = $1 WHERE id = $2 AND type = $3`, string(decision), transactionID, string(domain.Tx278))
	return err
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
	err := db.QueryRow(
		`SELECT id, adventurer_id, provider_id, incident_severity, COALESCE(transaction_id, ''), amount_cents, status
		 FROM claims WHERE id = $1`,
		claimID,
	).Scan(&claim.ID, &claim.AdventurerID, &claim.ProviderID, &claim.IncidentSeverity, &claim.TransactionID, &claim.AmountCents, &claim.Status)
	if err != nil {
		return err
	}
	if claim.Status != domain.ClaimPending && claim.Status != domain.ClaimSubmitted {
		return nil
	}

	adjudicateClaim(&claim)

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
	claim.Status = domain.ClaimApproved
	claim.AllowedAmountCents = percentage(claim.AmountCents, 80)
	claim.PaidAmountCents = percentage(claim.AllowedAmountCents, 85)
	claim.PatientResponsibilityCents = claim.AllowedAmountCents - claim.PaidAmountCents
	claim.AdjustmentAmountCents = claim.AmountCents - claim.AllowedAmountCents
	claim.AdjustmentReason = "ASHN contractual allowance"
	claim.DenialReason = ""

	if claim.IncidentSeverity == domain.SeverityNormal {
		claim.AllowedAmountCents = percentage(claim.AmountCents, 90)
		claim.PaidAmountCents = percentage(claim.AllowedAmountCents, 90)
		claim.PatientResponsibilityCents = claim.AllowedAmountCents - claim.PaidAmountCents
		claim.AdjustmentAmountCents = claim.AmountCents - claim.AllowedAmountCents
	}

	if claim.IncidentSeverity == domain.SeverityDiamond || claim.AmountCents > 200000 {
		claim.Status = domain.ClaimDenied
		claim.AllowedAmountCents = 0
		claim.PaidAmountCents = 0
		claim.PatientResponsibilityCents = 0
		claim.AdjustmentAmountCents = claim.AmountCents
		claim.AdjustmentReason = "Non-covered catastrophic encounter"
		claim.DenialReason = "Prior authorization or benefit exception required"
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
