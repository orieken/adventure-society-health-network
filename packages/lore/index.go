package lore

import (
	"fmt"

	"ashn/packages/domain"
)

func RankDescription(rank domain.Rank) string {
	switch rank {
	case domain.RankIron:
		return "Roadside Clinic — basic wound care, fatigue assessment"
	case domain.RankBronze:
		return "Town Healer's Guild — general practice, curse identification, triage"
	case domain.RankSilver:
		return "Regional Clinic — surgery, restoration magic, specialist referrals"
	case domain.RankGold:
		return "City Hospital — full inpatient, soul anchor procedures, resurrection"
	case domain.RankDiamond:
		return "Temple of the Healer — essence ability restoration, divine intervention, rare conditions"
	default:
		return "Unranked care site — records pending Society review"
	}
}

func SeverityDescription(severity domain.IncidentSeverity) string {
	switch severity {
	case domain.SeverityNormal:
		return "outpatient 837P — minor injury, surface wound, mild curse"
	case domain.SeverityAwakened:
		return "inpatient 837I — broken bones, moderate poison, essence disruption"
	case domain.SeverityDiamond:
		return "prior auth 278 + inpatient 837I — near-death, soul damage, dimensional affliction"
	default:
		return "unknown incident severity"
	}
}

func ThemeTransaction(txType domain.TransactionType, parties ...string) string {
	sender, receiver := "Unknown sender", "Unknown receiver"
	if len(parties) > 0 && parties[0] != "" {
		sender = parties[0]
	}
	if len(parties) > 1 && parties[1] != "" {
		receiver = parties[1]
	}
	switch txType {
	case domain.Tx834:
		return fmt.Sprintf("Society registration accepted for %s by %s", sender, receiver)
	case domain.Tx820:
		return fmt.Sprintf("Guild dues payment recorded for %s by %s", sender, receiver)
	case domain.Tx270:
		return fmt.Sprintf("Eligibility verification requested for %s at %s", sender, receiver)
	case domain.Tx271:
		return fmt.Sprintf("Eligibility response issued by %s for %s", receiver, sender)
	case domain.Tx275:
		return fmt.Sprintf("Patient information attachment sent by %s to %s", sender, receiver)
	case domain.Tx278:
		return fmt.Sprintf("Prior authorization requested for %s through %s", sender, receiver)
	case domain.Tx837:
		return fmt.Sprintf("Incident claim submitted by %s for %s", receiver, sender)
	case domain.Tx837D:
		return fmt.Sprintf("Dental claim submitted by %s for %s", receiver, sender)
	case domain.Tx835:
		return fmt.Sprintf("Adventure Society remittance issued to %s for %s", receiver, sender)
	case domain.Tx824:
		return fmt.Sprintf("Application advice rejected %s for %s", sender, receiver)
	case domain.Tx276:
		return fmt.Sprintf("Claim status inquiry sent by %s to %s", sender, receiver)
	case domain.Tx277:
		return fmt.Sprintf("Claim status response issued by %s to %s", receiver, sender)
	case domain.Tx269:
		return fmt.Sprintf("Coordination of benefits checked between %s and %s", sender, receiver)
	default:
		return fmt.Sprintf("ASHN transaction exchanged between %s and %s", sender, receiver)
	}
}
