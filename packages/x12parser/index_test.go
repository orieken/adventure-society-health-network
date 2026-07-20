package x12parser

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseMapsSupportedRawX12Transactions(t *testing.T) {
	tests := []struct {
		name   string
		raw    string
		assert func(*testing.T, ParsedTransaction)
	}{
		{
			name: "834 enrollment",
			raw:  "ISA*00*          *00*          *ZZ*partner-greenstone*ZZ*Adventure Society*260708*1200*^*00501*000000834*0*T*:~GS*BE*partner-greenstone*Adventure Society*20260708*1200*000000834*X*005010X220A1~ST*834*000000834~NM1*IL*1*Raw Ranger*****MI*adv-raw-834~K3*Rank:Silver~K3*Guild:Parser Guild~K3*Region:Greenstone~SE*7*000000834~GE*1*000000834~IEA*1*000000834~",
			assert: func(t *testing.T, parsed ParsedTransaction) {
				require.NotNil(t, parsed.Enrollment)
				assert.Equal(t, "834", parsed.Type)
				assert.Equal(t, "partner-greenstone", parsed.Sender.ID)
				assert.Equal(t, "Raw Ranger", parsed.Enrollment.Name)
				assert.Equal(t, "Silver", parsed.Enrollment.Rank)
			},
		},
		{
			name: "820 premium",
			raw:  "ISA*00*          *00*          *ZZ*partner-greenstone*ZZ*Adventure Society*260708*1200*^*00501*000000820*0*T*:~GS*RA*partner-greenstone*Adventure Society*20260708*1200*000000820*X*005010X218~ST*820*000000820~BPR*C*25.00~NM1*IL*1*Premium Ranger*****MI*adv-raw-820~SE*4*000000820~GE*1*000000820~IEA*1*000000820~",
			assert: func(t *testing.T, parsed ParsedTransaction) {
				require.NotNil(t, parsed.PremiumPayment)
				assert.Equal(t, "820", parsed.Type)
				assert.Equal(t, "adv-raw-820", parsed.PremiumPayment.AdventurerID)
				assert.Equal(t, "2500", parsed.PremiumPayment.AmountCents)
			},
		},
		{
			name: "270 eligibility",
			raw:  "ISA*00*          *00*          *ZZ*provider-vitesse-temple*ZZ*Adventure Society*260708*1200*^*00501*000000270*0*T*:~GS*HS*provider-vitesse-temple*Adventure Society*20260708*1200*000000270*X*005010X279A1~ST*270*000000270~NM1*1P*2*provider-vitesse-temple*****XX*provider-vitesse-temple~NM1*IL*1*Eligibility Ranger*****MI*adv-raw-270~EQ*35~SE*5*000000270~GE*1*000000270~IEA*1*000000270~",
			assert: func(t *testing.T, parsed ParsedTransaction) {
				require.NotNil(t, parsed.EligibilityInquiry)
				assert.Equal(t, "270", parsed.Type)
				assert.Equal(t, "provider-vitesse-temple", parsed.EligibilityInquiry.ProviderID)
				assert.Equal(t, "dental", parsed.EligibilityInquiry.ServiceType)
			},
		},
		{
			name: "276 claim status",
			raw:  "ISA*00*          *00*          *ZZ*provider-vitesse-temple*ZZ*Adventure Society*260708*1200*^*00501*000000276*0*T*:~GS*HR*provider-vitesse-temple*Adventure Society*20260708*1200*000000276*X*005010X212~ST*276*000000276~REF*1K*claim-raw-276~SE*3*000000276~GE*1*000000276~IEA*1*000000276~",
			assert: func(t *testing.T, parsed ParsedTransaction) {
				require.NotNil(t, parsed.ClaimStatusRequest)
				assert.Equal(t, "276", parsed.Type)
				assert.Equal(t, "claim-raw-276", parsed.ClaimStatusRequest.ClaimID)
			},
		},
		{
			name: "278 prior authorization",
			raw:  "ISA*00*          *00*          *ZZ*provider-vitesse-temple*ZZ*Adventure Society*260708*1200*^*00501*000000278*0*T*:~GS*HI*provider-vitesse-temple*Adventure Society*20260708*1200*000000278*X*005010X217~ST*278*000000278~NM1*1P*2*provider-vitesse-temple*****XX*provider-vitesse-temple~NM1*IL*1*Auth Ranger*****MI*adv-raw-278~UM*AR*I*2***resurrection~HI*ABK:S062X9A~SE*6*000000278~GE*1*000000278~IEA*1*000000278~",
			assert: func(t *testing.T, parsed ParsedTransaction) {
				require.NotNil(t, parsed.PriorAuthorization)
				assert.Equal(t, "278", parsed.Type)
				assert.Equal(t, "resurrection", parsed.PriorAuthorization.ServiceType)
				assert.Equal(t, "Diamond", parsed.PriorAuthorization.IncidentSeverity)
			},
		},
		{
			name: "837 claim",
			raw:  "ISA*00*          *00*          *ZZ*provider-vitesse-temple*ZZ*Adventure Society*260708*1200*^*00501*000000837*0*T*:~GS*HC*provider-vitesse-temple*Adventure Society*20260708*1200*000000837*X*005010X837P~ST*837*000000837~NM1*85*2*provider-vitesse-temple*****XX*provider-vitesse-temple~NM1*IL*1*Claim Ranger*****MI*adv-raw-837~CLM*claim-raw-837*1250.00***11:B:1*Y*A*Y*I~HI*ABK:S062X9A*ABF:T509~SV1*HC:ASHN1*950.00*UN*1***1~PWK*B4*EL****ATTACH-837~SE*8*000000837~GE*1*000000837~IEA*1*000000837~",
			assert: func(t *testing.T, parsed ParsedTransaction) {
				require.NotNil(t, parsed.Claim)
				assert.Equal(t, "837", parsed.Type)
				assert.Equal(t, "125000", parsed.Claim.AmountCents)
				assert.Equal(t, []ClaimDiagnosis{{Qualifier: "ABK", Code: "S062X9A", Primary: true}, {Qualifier: "ABF", Code: "T509"}}, parsed.Claim.Diagnoses)
				require.Len(t, parsed.Claim.ServiceLines, 1)
				assert.Equal(t, "ASHN1", parsed.Claim.ServiceLines[0].ProcedureCode)
				assert.Equal(t, "ATTACH-837", parsed.Claim.AttachmentControls[0].AttachmentControlNumber)
			},
		},
		{
			name: "275 attachment",
			raw:  "ISA*00*          *00*          *ZZ*provider-vitesse-temple*ZZ*Adventure Society*260708*1200*^*00501*000000275*0*T*:~GS*HC*provider-vitesse-temple*Adventure Society*20260708*1200*000000275*X*006020X314~ST*275*000000275~BGN*11*trace-277~NM1*1P*2*provider-vitesse-temple*****XX*provider-vitesse-temple~REF*1K*claim-raw-275~REF*6R*ATTACH-275~REF*F8*packet-raw-1-OF-2~DTP*472*D8*20260718~PWK*B4*EL***AC*ATTACH-275~CAT*B4*TXT~OOI*DOC*ATTACH-275~BDS*ASC**Content-Type: text/plain~LQ*AT*OZ~K3*Document-Reference: https://docs.example.test/raw.pdf~SE*14*000000275~GE*1*000000275~IEA*1*000000275~",
			assert: func(t *testing.T, parsed ParsedTransaction) {
				require.NotNil(t, parsed.Attachment)
				assert.Equal(t, "275", parsed.Type)
				assert.Equal(t, "solicited", parsed.Attachment.AttachmentPurpose)
				assert.Equal(t, "claim-raw-275", parsed.Attachment.ClaimID)
				assert.Equal(t, "packet-raw", parsed.Attachment.PacketID)
				assert.Equal(t, 1, parsed.Attachment.PacketSequence)
				assert.Equal(t, 2, parsed.Attachment.PacketCount)
				assert.Equal(t, "https://docs.example.test/raw.pdf", parsed.Attachment.DocumentReferenceURL)
			},
		},
		{
			name: "835 payment",
			raw:  "ISA*00*          *00*          *ZZ*Adventure Society*ZZ*provider-vitesse-temple*260708*1200*^*00501*000000835*0*T*:~GS*HP*Adventure Society*provider-vitesse-temple*20260708*1200*000000835*X*005010X221A1~ST*835*000000835~BPR*I*850.00~CLP*claim-raw-835*1*1250.00*850.00*150.00~SE*4*000000835~GE*1*000000835~IEA*1*000000835~",
			assert: func(t *testing.T, parsed ParsedTransaction) {
				require.NotNil(t, parsed.Payment)
				assert.Equal(t, "835", parsed.Type)
				assert.Equal(t, "claim-raw-835", parsed.Payment.ClaimID)
				assert.Equal(t, "85000", parsed.Payment.PaymentAmountCents)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed, err := Parse([]byte(tt.raw))
			require.NoError(t, err)
			tt.assert(t, parsed)
		})
	}
}

func TestParseRejectsUnsupportedAndInvalidPayloads(t *testing.T) {
	unsupported, err := Parse([]byte("ISA*00*          *00*          *ZZ*provider-vitesse-temple*ZZ*Adventure Society*260708*1200*^*00501*000000001*0*T*:~GS*HC*provider-vitesse-temple*Adventure Society*20260708*1200*000000001*X*005010X999A1~ST*999*000000001~SE*2*000000001~"))
	assert.Equal(t, "999", unsupported.Type)
	assert.EqualError(t, err, "raw X12 transaction type 999 not implemented")

	_, err = Parse([]byte("ST*837*0001~NM1*IL*1*adv-raw-1*****MI*adv-raw-1~"))
	assert.EqualError(t, err, "missing CLM claim segment")

	_, err = Parse([]byte("not really x12"))
	assert.EqualError(t, err, "missing ST transaction set")
}
