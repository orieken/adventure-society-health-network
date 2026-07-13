package asyncjobs

import (
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"testing"
	"time"

	"ashn/packages/domain"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProcessDueRequiresDatabase(t *testing.T) {
	processed, err := ProcessDue(nil, 0)

	assert.Equal(t, 0, processed)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "database is required")
}

func TestEnqueueNoopsWithoutDatabase(t *testing.T) {
	assert.NoError(t, Enqueue(nil, JobAuthReview, "tx-1", 0))
}

func TestEnqueuePersistsPendingJob(t *testing.T) {
	db, mock, cleanup := newMockDB(t)
	defer cleanup()
	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO transaction_jobs (id, job_type, entity_id, status, attempts, run_after, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, 0, $5, $6, $6)`)).
		WithArgs(sqlmock.AnyArg(), JobAuthReview, "tx-1", StatusPending, sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	require.NoError(t, Enqueue(db, JobAuthReview, "tx-1", time.Second))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestProcessDueCompletesClaimedJob(t *testing.T) {
	db, mock, cleanup := newMockDB(t)
	defer cleanup()
	now := time.Now().UTC()

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT id, job_type, entity_id, status, attempts, run_after, COALESCE(last_error, ''), created_at, updated_at
		 FROM transaction_jobs
		 WHERE status = $1 AND run_after <= now()
		 ORDER BY run_after, created_at
		 FOR UPDATE SKIP LOCKED
		 LIMIT 1`)).
		WithArgs(StatusPending).
		WillReturnRows(sqlmock.NewRows([]string{"id", "job_type", "entity_id", "status", "attempts", "run_after", "last_error", "created_at", "updated_at"}).
			AddRow("job-1", "unsupported", "entity-1", StatusPending, 0, now, "", now, now))
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE transaction_jobs SET status = $1, attempts = attempts + 1, updated_at = now() WHERE id = $2`)).
		WithArgs(StatusProcessing, "job-1").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE transaction_jobs SET status = $1, run_after = $2, last_error = $3, updated_at = now() WHERE id = $4`)).
		WithArgs(StatusPending, sqlmock.AnyArg(), jsonErrorArg{contains: "unsupported job type"}, "job-1").
		WillReturnResult(sqlmock.NewResult(0, 1))

	processed, err := ProcessDue(db, 1)

	require.NoError(t, err)
	assert.Equal(t, 1, processed)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestProcessDueReturnsWhenNoJobsAreReady(t *testing.T) {
	db, mock, cleanup := newMockDB(t)
	defer cleanup()
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT id, job_type, entity_id, status, attempts, run_after, COALESCE(last_error, ''), created_at, updated_at
		 FROM transaction_jobs
		 WHERE status = $1 AND run_after <= now()
		 ORDER BY run_after, created_at
		 FOR UPDATE SKIP LOCKED
		 LIMIT 1`)).
		WithArgs(StatusPending).
		WillReturnRows(sqlmock.NewRows([]string{"id", "job_type", "entity_id", "status", "attempts", "run_after", "last_error", "created_at", "updated_at"}))
	mock.ExpectRollback()

	processed, err := ProcessDue(db, 5)

	require.NoError(t, err)
	assert.Equal(t, 0, processed)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestProcessJobRejectsUnsupportedJobType(t *testing.T) {
	err := processJob(nil, Job{Type: "unknown", EntityID: "entity-1"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported job type")
}

func TestProcessJobDispatchesSupportedTypes(t *testing.T) {
	authDB, authMock, authCleanup := newMockDB(t)
	defer authCleanup()
	authMock.ExpectQuery(regexp.QuoteMeta(`SELECT service_type, incident_severity, status FROM auth_requests WHERE transaction_id = $1`)).
		WithArgs("tx-278").
		WillReturnRows(sqlmock.NewRows([]string{"service_type", "incident_severity", "status"}).AddRow("resurrection", domain.SeverityDiamond, domain.TxStatusPending))
	authMock.ExpectExec(regexp.QuoteMeta(`UPDATE auth_requests SET status = $1 WHERE transaction_id = $2`)).
		WithArgs(string(domain.TxStatusApproved), "tx-278").
		WillReturnResult(sqlmock.NewResult(0, 1))
	authMock.ExpectExec(regexp.QuoteMeta(`UPDATE claims SET authorization_status = $1, authorization_reason = $2 WHERE authorization_transaction_id = $3`)).
		WithArgs(string(domain.TxStatusApproved), "Auto-approved by severity and service-type rule.", "tx-278").
		WillReturnResult(sqlmock.NewResult(0, 1))
	authMock.ExpectExec(regexp.QuoteMeta(`UPDATE transactions SET status = $1 WHERE id = $2 AND type = $3`)).
		WithArgs(string(domain.TxStatusApproved), "tx-278", string(domain.Tx278)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	require.NoError(t, processJob(authDB, Job{Type: JobAuthReview, EntityID: "tx-278"}))
	require.NoError(t, authMock.ExpectationsWereMet())

	claimDB, claimMock, claimCleanup := newMockDB(t)
	defer claimCleanup()
	claimMock.ExpectExec(regexp.QuoteMeta(`UPDATE claims SET status = $1 WHERE id = $2 AND status = $3`)).
		WithArgs(string(domain.ClaimPending), "claim-1", string(domain.ClaimSubmitted)).
		WillReturnResult(sqlmock.NewResult(0, 0))
	require.NoError(t, processJob(claimDB, Job{Type: JobClaimAdjudication, EntityID: "claim-1"}))
	require.NoError(t, claimMock.ExpectationsWereMet())
}

func TestProcessAuthReviewApprovesDiamondResurrection(t *testing.T) {
	db, mock, cleanup := newMockDB(t)
	defer cleanup()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT service_type, incident_severity, status FROM auth_requests WHERE transaction_id = $1`)).
		WithArgs("tx-278").
		WillReturnRows(sqlmock.NewRows([]string{"service_type", "incident_severity", "status"}).AddRow("resurrection", domain.SeverityDiamond, domain.TxStatusPending))
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE auth_requests SET status = $1 WHERE transaction_id = $2`)).
		WithArgs(string(domain.TxStatusApproved), "tx-278").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE claims SET authorization_status = $1, authorization_reason = $2 WHERE authorization_transaction_id = $3`)).
		WithArgs(string(domain.TxStatusApproved), "Auto-approved by severity and service-type rule.", "tx-278").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE transactions SET status = $1 WHERE id = $2 AND type = $3`)).
		WithArgs(string(domain.TxStatusApproved), "tx-278", string(domain.Tx278)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	require.NoError(t, processAuthReview(db, "tx-278"))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestProcessAuthReviewDeniesNonResurrection(t *testing.T) {
	db, mock, cleanup := newMockDB(t)
	defer cleanup()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT service_type, incident_severity, status FROM auth_requests WHERE transaction_id = $1`)).
		WithArgs("tx-278").
		WillReturnRows(sqlmock.NewRows([]string{"service_type", "incident_severity", "status"}).AddRow("campfire rest", domain.SeverityDiamond, domain.TxStatusPending))
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE auth_requests SET status = $1 WHERE transaction_id = $2`)).
		WithArgs(string(domain.TxStatusDenied), "tx-278").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE claims SET authorization_status = $1, authorization_reason = $2 WHERE authorization_transaction_id = $3`)).
		WithArgs(string(domain.TxStatusDenied), "Auto-denied by severity and service-type rule.", "tx-278").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE transactions SET status = $1 WHERE id = $2 AND type = $3`)).
		WithArgs(string(domain.TxStatusDenied), "tx-278", string(domain.Tx278)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	require.NoError(t, processAuthReview(db, "tx-278"))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestProcessAuthReviewSkipsManualDecision(t *testing.T) {
	db, mock, cleanup := newMockDB(t)
	defer cleanup()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT service_type, incident_severity, status FROM auth_requests WHERE transaction_id = $1`)).
		WithArgs("tx-278").
		WillReturnRows(sqlmock.NewRows([]string{"service_type", "incident_severity", "status"}).AddRow("resurrection", domain.SeverityDiamond, domain.TxStatusDenied))

	require.NoError(t, processAuthReview(db, "tx-278"))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestProcessClaimAdjudicationQueuesFinalizationWhenClaimIsSubmitted(t *testing.T) {
	db, mock, cleanup := newMockDB(t)
	defer cleanup()
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE claims SET status = $1 WHERE id = $2 AND status = $3`)).
		WithArgs(string(domain.ClaimPending), "claim-1", string(domain.ClaimSubmitted)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO transaction_jobs (id, job_type, entity_id, status, attempts, run_after, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, 0, $5, $6, $6)`)).
		WithArgs(sqlmock.AnyArg(), JobClaimFinalization, "claim-1", StatusPending, sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	require.NoError(t, processClaimAdjudication(db, "claim-1"))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestProcessClaimAdjudicationNoopsWhenClaimAlreadyMoved(t *testing.T) {
	db, mock, cleanup := newMockDB(t)
	defer cleanup()
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE claims SET status = $1 WHERE id = $2 AND status = $3`)).
		WithArgs(string(domain.ClaimPending), "claim-1", string(domain.ClaimSubmitted)).
		WillReturnResult(sqlmock.NewResult(0, 0))

	require.NoError(t, processClaimAdjudication(db, "claim-1"))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestProcessClaimFinalizationUpdatesClaimAndRecords277(t *testing.T) {
	db, mock, cleanup := newMockDB(t)
	defer cleanup()
	mock.ExpectQuery(claimFinalizationQueryPattern()).
		WithArgs("claim-1").
		WillReturnRows(claimFinalizationRows().
			AddRow("claim-1", "adv-1", "provider-1", domain.SeverityAwakened, "tx-837", "", "", "", int64(100000), `[{"lineNumber":1,"procedureCode":"ASHN1","description":"Stabilization","units":1,"amountCents":100000}]`, domain.ClaimPending, "", "", "", false, int64(0)))
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE claims SET status = $1, allowed_amount_cents = $2, paid_amount_cents = $3, patient_responsibility_cents = $4, adjustment_amount_cents = $5, adjustment_reason = NULLIF($6, ''), denial_reason = NULLIF($7, ''), service_lines = $8::jsonb WHERE id = $9`)).
		WithArgs(string(domain.ClaimApproved), int64(80000), int64(68000), int64(12000), int64(20000), "ASHN contractual allowance", "", jsonServiceLinesArg{contains: `"paidAmountCents":68000`}, "claim-1").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO transactions (id, type, status, sender_id, receiver_id, payload, raw_x12, related_id, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, NULLIF($8, ''), $9)
		 ON CONFLICT (id) DO NOTHING`)).
		WithArgs(sqlmock.AnyArg(), domain.Tx277, domain.TxStatusAccepted, "Adventure Society", "provider", sqlmock.AnyArg(), sqlmock.AnyArg(), "tx-837", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	require.NoError(t, processClaimFinalization(db, "claim-1"))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestProcessClaimFinalizationSkipsCompletedClaims(t *testing.T) {
	db, mock, cleanup := newMockDB(t)
	defer cleanup()
	mock.ExpectQuery(claimFinalizationQueryPattern()).
		WithArgs("claim-1").
		WillReturnRows(claimFinalizationRows().
			AddRow("claim-1", "adv-1", "provider-1", domain.SeverityNormal, "tx-837", "", "", "", int64(100000), `[]`, domain.ClaimPaid, "", "", "", false, int64(0)))

	require.NoError(t, processClaimFinalization(db, "claim-1"))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestAdjudicateClaimApprovesStandardClaim(t *testing.T) {
	claim := domain.Claim{IncidentSeverity: domain.SeverityAwakened, AmountCents: 100000}

	adjudicateClaim(&claim)

	assert.Equal(t, domain.ClaimApproved, claim.Status)
	assert.Equal(t, int64(80000), claim.AllowedAmountCents)
	assert.Equal(t, int64(68000), claim.PaidAmountCents)
	assert.Equal(t, int64(12000), claim.PatientResponsibilityCents)
	assert.Equal(t, int64(20000), claim.AdjustmentAmountCents)
	assert.Equal(t, "ASHN contractual allowance", claim.AdjustmentReason)
	assert.Empty(t, claim.DenialReason)
}

func TestAdjudicateClaimNormalSeverityUsesRicherAllowance(t *testing.T) {
	claim := domain.Claim{IncidentSeverity: domain.SeverityNormal, AmountCents: 100000}

	adjudicateClaim(&claim)

	assert.Equal(t, domain.ClaimApproved, claim.Status)
	assert.Equal(t, int64(90000), claim.AllowedAmountCents)
	assert.Equal(t, int64(81000), claim.PaidAmountCents)
	assert.Equal(t, int64(9000), claim.PatientResponsibilityCents)
	assert.Equal(t, int64(10000), claim.AdjustmentAmountCents)
}

func TestAdjudicateClaimRollsUpServiceLines(t *testing.T) {
	claim := domain.Claim{
		IncidentSeverity: domain.SeverityAwakened,
		AmountCents:      125000,
		ServiceLines: []domain.ClaimServiceLine{
			{LineNumber: 1, ProcedureCode: "ASHN1", Description: "Resurrection stabilization", Units: 1, AmountCents: 95000},
			{LineNumber: 2, ProcedureCode: "ASHN2", Description: "Dragonfire trauma supplies", Units: 1, AmountCents: 30000},
		},
	}

	adjudicateClaim(&claim)

	assert.Equal(t, domain.ClaimApproved, claim.Status)
	assert.Equal(t, int64(100000), claim.AllowedAmountCents)
	assert.Equal(t, int64(85000), claim.PaidAmountCents)
	assert.Equal(t, int64(15000), claim.PatientResponsibilityCents)
	assert.Equal(t, int64(25000), claim.AdjustmentAmountCents)
	require.Len(t, claim.ServiceLines, 2)
	assert.Equal(t, int64(76000), claim.ServiceLines[0].AllowedAmountCents)
	assert.Equal(t, int64(64600), claim.ServiceLines[0].PaidAmountCents)
	assert.Equal(t, int64(24000), claim.ServiceLines[1].AllowedAmountCents)
	assert.Equal(t, int64(20400), claim.ServiceLines[1].PaidAmountCents)
}

func TestAdjudicateClaimDeniesCatastrophicClaims(t *testing.T) {
	for _, claim := range []domain.Claim{
		{IncidentSeverity: domain.SeverityDiamond, AmountCents: 100000},
		{IncidentSeverity: domain.SeverityAwakened, AmountCents: 250000},
	} {
		adjudicateClaim(&claim)

		assert.Equal(t, domain.ClaimDenied, claim.Status)
		assert.Equal(t, int64(0), claim.AllowedAmountCents)
		assert.Equal(t, int64(0), claim.PaidAmountCents)
		assert.Equal(t, claim.AmountCents, claim.AdjustmentAmountCents)
		assert.Equal(t, "Non-covered catastrophic encounter", claim.AdjustmentReason)
		assert.Equal(t, "Prior authorization or benefit exception required", claim.DenialReason)
	}
}

func TestAdjudicateClaimHonorsApprovedPriorAuthorization(t *testing.T) {
	claim := domain.Claim{
		IncidentSeverity:           domain.SeverityDiamond,
		AmountCents:                250000,
		AuthorizationTransactionID: "tx-278-approved",
		AuthorizationStatus:        string(domain.TxStatusApproved),
	}

	adjudicateClaim(&claim)

	assert.Equal(t, domain.ClaimApproved, claim.Status)
	assert.Equal(t, int64(200000), claim.AllowedAmountCents)
	assert.Equal(t, int64(170000), claim.PaidAmountCents)
	assert.Equal(t, int64(30000), claim.PatientResponsibilityCents)
	assert.Equal(t, int64(50000), claim.AdjustmentAmountCents)
	assert.Contains(t, claim.AdjustmentReason, "approved prior authorization")
	assert.Empty(t, claim.DenialReason)
}

func TestAdjudicateClaimAppliesProviderTierAndAdventurerRank(t *testing.T) {
	claim := domain.Claim{IncidentSeverity: domain.SeverityAwakened, AmountCents: 100000}

	adjudicateClaimWithContext(&claim, adjudicationContext{
		AdventurerRank: domain.RankGold,
		CoverageStatus: domain.CoverageActive,
		ProviderTier:   domain.RankDiamond,
	})

	assert.Equal(t, domain.ClaimApproved, claim.Status)
	assert.Equal(t, int64(85000), claim.AllowedAmountCents)
	assert.Equal(t, int64(80750), claim.PaidAmountCents)
	assert.Equal(t, int64(4250), claim.PatientResponsibilityCents)
	assert.Equal(t, int64(15000), claim.AdjustmentAmountCents)
	assert.Equal(t, "ASHN contractual allowance", claim.AdjustmentReason)
}

func TestAdjudicateClaimUsesCoverageStatus(t *testing.T) {
	pending := domain.Claim{IncidentSeverity: domain.SeverityNormal, AmountCents: 100000}
	adjudicateClaimWithContext(&pending, adjudicationContext{CoverageStatus: domain.CoveragePending})
	assert.Equal(t, domain.ClaimApproved, pending.Status)
	assert.Equal(t, int64(90000), pending.AllowedAmountCents)
	assert.Equal(t, int64(67500), pending.PaidAmountCents)
	assert.Equal(t, "ASHN contractual allowance with pending benefits review", pending.AdjustmentReason)

	inactive := domain.Claim{IncidentSeverity: domain.SeverityNormal, AmountCents: 100000}
	adjudicateClaimWithContext(&inactive, adjudicationContext{CoverageStatus: domain.CoverageSuspended})
	assert.Equal(t, domain.ClaimDenied, inactive.Status)
	assert.Equal(t, "Coverage not active", inactive.AdjustmentReason)
	assert.Equal(t, "Coverage inactive or suspended at service date", inactive.DenialReason)
}

func TestAdjudicateClaimDeniedVariants(t *testing.T) {
	tests := []struct {
		name             string
		claim            domain.Claim
		context          adjudicationContext
		adjustmentReason string
		denialReason     string
	}{
		{
			name:             "diamond severity without authorization",
			claim:            domain.Claim{IncidentSeverity: domain.SeverityDiamond, AmountCents: 100000},
			adjustmentReason: "Non-covered catastrophic encounter",
			denialReason:     "Prior authorization or benefit exception required",
		},
		{
			name:             "high billed amount without authorization",
			claim:            domain.Claim{IncidentSeverity: domain.SeverityAwakened, AmountCents: 250000},
			adjustmentReason: "Non-covered catastrophic encounter",
			denialReason:     "Prior authorization or benefit exception required",
		},
		{
			name:             "inactive coverage",
			claim:            domain.Claim{IncidentSeverity: domain.SeverityNormal, AmountCents: 100000},
			context:          adjudicationContext{CoverageStatus: domain.CoverageInactive, ProviderTier: domain.RankDiamond, AdventurerRank: domain.RankDiamond},
			adjustmentReason: "Coverage not active",
			denialReason:     "Coverage inactive or suspended at service date",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adjudicateClaimWithContext(&tt.claim, tt.context)

			assert.Equal(t, domain.ClaimDenied, tt.claim.Status)
			assert.Equal(t, int64(0), tt.claim.AllowedAmountCents)
			assert.Equal(t, int64(0), tt.claim.PaidAmountCents)
			assert.Equal(t, int64(0), tt.claim.PatientResponsibilityCents)
			assert.Equal(t, tt.claim.AmountCents, tt.claim.AdjustmentAmountCents)
			assert.Equal(t, tt.adjustmentReason, tt.claim.AdjustmentReason)
			assert.Equal(t, tt.denialReason, tt.claim.DenialReason)
		})
	}
}

func TestAdjudicateClaimPartialPaymentVariants(t *testing.T) {
	tests := []struct {
		name                  string
		claim                 domain.Claim
		context               adjudicationContext
		allowedAmount         int64
		paidAmount            int64
		patientResponsibility int64
		adjustmentAmount      int64
	}{
		{
			name:                  "standard awakened partial payment",
			claim:                 domain.Claim{IncidentSeverity: domain.SeverityAwakened, AmountCents: 100000},
			allowedAmount:         80000,
			paidAmount:            68000,
			patientResponsibility: 12000,
			adjustmentAmount:      20000,
		},
		{
			name:                  "pending coverage reduces paid amount",
			claim:                 domain.Claim{IncidentSeverity: domain.SeverityNormal, AmountCents: 100000},
			context:               adjudicationContext{CoverageStatus: domain.CoveragePending},
			allowedAmount:         90000,
			paidAmount:            67500,
			patientResponsibility: 22500,
			adjustmentAmount:      10000,
		},
		{
			name:                  "silver provider and silver adventurer improve payment",
			claim:                 domain.Claim{IncidentSeverity: domain.SeverityAwakened, AmountCents: 100000},
			context:               adjudicationContext{ProviderTier: domain.RankSilver, AdventurerRank: domain.RankSilver, CoverageStatus: domain.CoverageActive},
			allowedAmount:         80000,
			paidAmount:            70400,
			patientResponsibility: 9600,
			adjustmentAmount:      20000,
		},
		{
			name:                  "current premium improves paid amount",
			claim:                 domain.Claim{IncidentSeverity: domain.SeverityAwakened, AmountCents: 100000},
			context:               adjudicationContext{PremiumCurrent: true, PremiumPaidAmountCents: 25000},
			allowedAmount:         80000,
			paidAmount:            70400,
			patientResponsibility: 9600,
			adjustmentAmount:      20000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adjudicateClaimWithContext(&tt.claim, tt.context)

			assert.Equal(t, domain.ClaimApproved, tt.claim.Status)
			assert.Equal(t, tt.allowedAmount, tt.claim.AllowedAmountCents)
			assert.Equal(t, tt.paidAmount, tt.claim.PaidAmountCents)
			assert.Equal(t, tt.patientResponsibility, tt.claim.PatientResponsibilityCents)
			assert.Equal(t, tt.adjustmentAmount, tt.claim.AdjustmentAmountCents)
			assert.Empty(t, tt.claim.DenialReason)
		})
	}
}

func TestPercentageUsesIntegerMath(t *testing.T) {
	assert.Equal(t, int64(33), percentage(100, 33))
	assert.Equal(t, int64(0), percentage(99, 0))
}

func TestMarkCompletedAndFailedUpdateJobs(t *testing.T) {
	db, mock, cleanup := newMockDB(t)
	defer cleanup()
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE transaction_jobs SET status = $1, updated_at = now() WHERE id = $2`)).
		WithArgs(StatusCompleted, "job-1").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE transaction_jobs SET status = $1, run_after = $2, last_error = $3, updated_at = now() WHERE id = $4`)).
		WithArgs(StatusFailed, sqlmock.AnyArg(), jsonErrorArg{contains: "boom"}, "job-2").
		WillReturnResult(sqlmock.NewResult(0, 1))

	markCompleted(db, Job{ID: "job-1"})
	markFailed(db, Job{ID: "job-2", Attempts: 2}, errors.New("boom"))

	require.NoError(t, mock.ExpectationsWereMet())
}

func newMockDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock, func()) {
	t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	return db, mock, func() {
		_ = db.Close()
	}
}

func claimFinalizationQueryPattern() string {
	return "SELECT c\\.id, c\\.adventurer_id, c\\.provider_id, c\\.incident_severity"
}

func claimFinalizationRows() *sqlmock.Rows {
	return sqlmock.NewRows([]string{
		"id", "adventurer_id", "provider_id", "incident_severity", "transaction_id", "authorization_transaction_id", "authorization_status", "authorization_reason", "amount_cents", "service_lines", "status",
		"adventurer_rank", "coverage_status", "provider_tier", "premium_current", "premium_paid_amount_cents",
	})
}

type jsonErrorArg struct {
	contains string
}

func (arg jsonErrorArg) Match(value driver.Value) bool {
	text, ok := value.(string)
	if !ok {
		return false
	}
	var payload map[string]string
	if err := json.Unmarshal([]byte(text), &payload); err != nil {
		return false
	}
	return strings.Contains(payload["error"], arg.contains)
}

type jsonServiceLinesArg struct {
	contains string
}

func (arg jsonServiceLinesArg) Match(value driver.Value) bool {
	text, ok := value.(string)
	return ok && strings.Contains(text, arg.contains)
}
