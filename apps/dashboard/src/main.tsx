import React, { FormEvent, useEffect, useMemo, useRef, useState } from "react";
import { createRoot } from "react-dom/client";
import "./styles.css";

type Provider = {
  id: string;
  name: string;
  providerType: string;
  tierRank: string;
  region: string;
};

type TradingPartner = {
  id: string;
  name: string;
  senderId: string;
  receiverId: string;
  allowedTransactionTypes: string[];
  routeTarget: string;
  status: string;
  validationProfile?: {
    attachmentTypes?: string[];
    reportTypeCodes?: string[];
    contentTypes?: string[];
    allowedFileExtensions?: string[];
    maxEmbeddedContentBytes?: number;
    maxAttachmentsPerPacket?: number;
    unsolicitedAttachmentWindowDays?: number;
    diagnosisQualifiers?: string[];
    diagnosisCodes?: string[];
    procedureCodePrefixes?: string[];
    procedureCodes?: string[];
    dentalCdtRanges?: string[];
    dentalRequiredAttachmentCodes?: string[];
    dentalRequiresToothNumber?: boolean;
    dentalAllowedSurfaces?: string[];
    dentalAllowedQuadrants?: string[];
    dentalPredeterminationRules?: string[];
    serviceTypes?: string[];
    incidentSeverities?: string[];
  };
};

type PartnerFormState = {
  id: string;
  name: string;
  senderId: string;
  receiverId: string;
  allowedTransactionTypes: string;
  routeTarget: string;
  status: string;
};

type SavedFilter = {
  id: string;
  name: string;
  tab: DashboardTab;
  searchTerm: string;
  transactionType: string;
  transactionStatus: string;
  claimStatus: string;
  provider: string;
  auditStatus: string;
  auditType: string;
};

type Adventurer = {
  id: string;
  name: string;
  rank: string;
  guild: string;
  region: string;
  coverageStatus: string;
};

type Claim = {
  id: string;
  adventurerId: string;
  providerId: string;
  incidentSeverity: string;
  transactionId: string;
  authorizationTransactionId?: string;
  authorizationStatus?: string;
  authorizationReason?: string;
  amountCents: number;
  allowedAmountCents?: number;
  paidAmountCents?: number;
  patientResponsibilityCents?: number;
  adjustmentAmountCents?: number;
  adjustmentReason?: string;
  denialReason?: string;
  status: string;
  serviceLines?: ClaimServiceLine[];
  diagnoses?: ClaimDiagnosis[];
};

type ClaimDiagnosis = {
  qualifier: string;
  code: string;
  description?: string;
  primary?: boolean;
};

type ClaimServiceLine = {
  lineNumber: number;
  procedureCode: string;
  description: string;
  units: number;
  amountCents: number;
  cdtCode?: string;
  toothNumber?: string;
  surface?: string;
  quadrant?: string;
  orthodontic?: boolean;
  allowedAmountCents?: number;
  paidAmountCents?: number;
  patientResponsibilityCents?: number;
  adjustmentAmountCents?: number;
  adjustmentReason?: string;
  denialReason?: string;
};

type AdjudicationExplanation = {
  engine?: string;
  allowedAmountCents?: number;
  paidAmountCents?: number;
  patientResponsibilityCents?: number;
  adjustmentAmountCents?: number;
  adjustmentReason?: string;
  denialReason?: string;
  coverageStatus?: string;
  providerTier?: string;
  adventurerRank?: string;
  premiumCurrent?: boolean;
  premiumPaidAmountCents?: number;
};

type PremiumPayment = {
  id: string;
  adventurerId: string;
  transactionId: string;
  amountCents: number;
  status: string;
  createdAt: string;
  reconciled: boolean;
  currentForBenefits: boolean;
};

type DocumentationChecklistItem = {
  code: string;
  label: string;
  attachmentType: string;
  reportTypeCode: string;
  contentType: string;
  required: boolean;
};

type AttachmentDraft = {
  packetId: string;
  packetSequence?: number;
  packetCount?: number;
  attachmentType: string;
  attachmentControlNumber: string;
  reportTypeCode: string;
  transmissionCode: string;
  contentType: string;
  description: string;
  documentReferenceId: string;
  documentReferenceUrl: string;
  content: string;
};

type PageInfo = {
  limit: number;
  offset: number;
  count: number;
  hasMore: boolean;
};

type Envelope<T = unknown> = {
  data?: T;
  lore?: string;
  transaction?: Transaction;
  transactions?: Transaction[];
  page?: PageInfo;
  error?: string;
};

type Transaction = {
  id: string;
  type: string;
  status: string;
  senderId: string;
  receiverId: string;
  payload: unknown;
  rawX12?: string;
  relatedId?: string;
  createdAt: string;
};

type DocumentReference = {
  transactionId: string;
  claimId?: string;
  authorizationTransactionId?: string;
  attachmentType?: string;
  attachmentControlNumber?: string;
  reportTypeCode?: string;
  contentType?: string;
  description?: string;
  documentReferenceId?: string;
  documentReferenceUrl?: string;
  embeddedContentAvailable: boolean;
  retrievalMode: string;
  retrievalStatus: string;
  retrievalInstructions: string;
};

type TimelineGroup = {
  id: string;
  title: string;
  subtitle: string;
  transactions: Transaction[];
  latestAt: number;
};

type AttachmentPacketSummary = {
  packetId: string;
  observedCount: number;
  declaredCount?: number;
  acceptedCount: number;
  rejectedCount: number;
  reviewStatuses: string[];
  labels: string[];
};

type TransactionRelationshipMap = {
  parent?: Transaction;
  current: Transaction;
  children: Transaction[];
};

type AcknowledgmentDrilldown = {
  id: string;
  label: string;
  typeFilter: string;
  count: number;
  detail: string;
  examples: Transaction[];
};

type DashboardTab = "workflow" | "metrics" | "timeline" | "ledger" | "xml" | "partners";
type PayloadTab = "json" | "xml" | "x12";

type InboundMessage = {
  id: string;
  partnerId?: string;
  contentType: string;
  transactionType?: string;
  rawPayload: string;
  status: string;
  error?: string;
  downstreamStatus?: number;
  createdAt: string;
};

type TransactionJob = {
  id: string;
  type: string;
  entityId: string;
  status: string;
  attempts: number;
  runAfter: string;
  lastError?: string;
  deadLetter: boolean;
  createdAt: string;
  updatedAt: string;
};

type ReadinessCheck = {
  name: string;
  status: string;
  detail: string;
  count?: number;
};

type ReadinessReport = {
  status: string;
  generatedAt: string;
  version: string;
  commit?: string;
  services: Record<string, string>;
  checks: ReadinessCheck[];
  summary: Record<string, number>;
  links: Record<string, string>;
};

type MetricsSummary = {
  generatedAt: string;
  window: string;
  transactions: {
    totalLoaded: number;
    byType: Record<string, number>;
    byStatus: Record<string, number>;
  };
  claims: {
    totalLoaded: number;
    byStatus: Record<string, number>;
    byProvider: Record<string, number>;
  };
  intake: {
    rejectionTotal: number;
    byPartner: Record<string, number>;
    byType: Record<string, number>;
    byReason: Record<string, number>;
  };
  asyncJobs: {
    totalLoaded: number;
    byStatus: Record<string, number>;
    deadLetters: number;
  };
  financials: {
    billedCents: number;
    allowedCents: number;
    paidCents: number;
    patientResponsibilityCents: number;
    adjustmentCents: number;
  };
  operationalStatus: string;
  highlights: string[];
};

type DemoScenario = {
  id: string;
  title: string;
  outcome: string;
  audience: string;
  duration: string;
  story: string;
  highlights: string[];
  steps: Array<{
    label: string;
    action: string;
    expected: string;
  }>;
  exports: string[];
};

type RejectionFixture = {
  id: string;
  label: string;
  reason: string;
  payload: string;
};

type ScenarioRunState = {
  running: boolean;
  completedSteps: number;
  mode?: "auto" | "playback";
  runId?: string;
  startedAt?: string;
  completedAt?: string;
  currentStep?: string;
  error?: string;
  evidence?: ScenarioStepEvidence[];
};

type ScenarioPlaybackContext = {
  adventurer?: Adventurer;
  authorization?: Transaction;
  claim?: Claim;
  packet?: Envelope<Record<string, string>>;
};

type ScenarioStepEvidence = {
  label: string;
  action: string;
  expected: string;
  completedAt: string;
  transactionIds: string[];
  transactionTypes: string[];
  relatedIds: string[];
  claimId?: string;
  adventurerId?: string;
  lore?: string;
  error?: string;
};

type ScenarioRunRecord = {
  id: string;
  scenarioId: string;
  scenarioTitle: string;
  completedAt: string;
  completedSteps: number;
  totalSteps: number;
  status: string;
  transactionIds: string[];
  claimIds: string[];
  adventurerIds: string[];
  bundle: unknown;
};

type IntakeRejectionSummary = {
  messages: InboundMessage[];
  byPartner: Array<{ label: string; count: number }>;
  byType: Array<{ label: string; count: number }>;
  byReason: Array<{ label: string; count: number }>;
};

type IntakeRejectionCount = {
  label: string;
  count: number;
  query?: string;
  type?: string;
  partnerId?: string;
};

type IntakeRejectionTrend = {
  date: string;
  count: number;
};

type IntakeRejectionMetrics = {
  total: number;
  byPartner: IntakeRejectionCount[];
  byType: IntakeRejectionCount[];
  byReason: IntakeRejectionCount[];
  trend: IntakeRejectionTrend[];
  latest: InboundMessage[];
};

const apiUrl = import.meta.env.VITE_ASHN_API_URL ?? "http://localhost:8080";
const apiKey = String(import.meta.env.VITE_ASHN_API_KEY ?? "");
const adventurerPageSize = 10;
const claimPageSize = 10;
const transactionPageSize = 25;
const auditPageSize = 10;
const dashboardRefreshMs = 3000;
const transactionTypes = ["All", "834", "820", "270", "271", "275", "278", "837", "837D", "835", "824", "TA1", "276", "277", "269", "999", "277CA"];
const x12CapabilityRows = [
  { type: "834", direction: "Inbound", parser: "Raw X12 + XML/JSON", generated: "Yes", acknowledgments: "999", learning: "Enrollment member identity, coverage effective date, guild rank, and region." },
  { type: "820", direction: "Inbound", parser: "Raw X12 + XML/JSON", generated: "Yes", acknowledgments: "999", learning: "Premium payment amount, reconciliation status, and benefit-current adjudication signal." },
  { type: "270/271", direction: "Round trip", parser: "270 raw/XML/JSON", generated: "271", acknowledgments: "999", learning: "Eligibility request and response, including dental benefits, limits, waiting periods, and maximums." },
  { type: "278", direction: "Inbound + review", parser: "Raw X12 + XML/JSON", generated: "Yes", acknowledgments: "999", learning: "Prior authorization and dental predetermination lifecycle with manual approval or denial." },
  { type: "275", direction: "Inbound attachment", parser: "Raw X12 + XML/JSON", generated: "Yes", acknowledgments: "999 + 824", learning: "Claim/auth documentation packets, BGN purpose, trace correlation, MIME/Base64/REF validation, and review outcomes." },
  { type: "837/837D", direction: "Inbound claim", parser: "Raw X12 + XML/JSON", generated: "Yes", acknowledgments: "999 + 277CA", learning: "Professional and dental claim intake, partner guide validation, service lines, CDT/tooth detail, and async adjudication." },
  { type: "835", direction: "Outbound remittance", parser: "Raw X12 + XML/JSON", generated: "Yes", acknowledgments: "999", learning: "Payment, allowed amount, patient responsibility, adjustments, and service-line remittance detail." },
  { type: "276/277", direction: "Round trip", parser: "276 raw/XML/JSON", generated: "277", acknowledgments: "999", learning: "Claim status inquiry, documentation requests, adjudication explainers, and request/response correlation." },
  { type: "TA1/999/824/277CA", direction: "Acknowledgment", parser: "Display + ledger", generated: "Yes", acknowledgments: "N/A", learning: "Interchange, syntax, application, and claim-acceptance outcomes separated from business decisions." }
];
const transactionStatuses = ["All", "Created", "Dispatched", "Accepted", "Pending", "Approved", "Denied", "Paid", "Failed"];
const claimStatuses = ["All", "Submitted", "Pending", "Pending Documentation", "Approved", "Denied", "Paid"];
const auditStatuses = ["All", "accepted", "rejected"];
const documentationChecklist: DocumentationChecklistItem[] = [
  { code: "MED-NEC", label: "Medical necessity letter", attachmentType: "OZ", reportTypeCode: "B4", contentType: "text/plain", required: true },
  { code: "ENC-NOTE", label: "Encounter notes", attachmentType: "OZ", reportTypeCode: "B4", contentType: "text/plain", required: true },
  { code: "ITEM-BILL", label: "Itemized bill narrative", attachmentType: "OZ", reportTypeCode: "B4", contentType: "text/plain", required: false }
];
const payloadTabs: { id: PayloadTab; label: string }[] = [
  { id: "json", label: "JSON" },
  { id: "xml", label: "XML" },
  { id: "x12", label: "X12" }
];
const sampleRaw834 = [
  "ISA*00*          *00*          *ZZ*partner-greenstone*ZZ*Adventure Society*260708*1200*^*00501*000000834*0*T*:~",
  "GS*BE*partner-greenstone*Adventure Society*20260708*1200*000000834*X*005010X220A1~",
  "ST*834*000000834~",
  "BGN*00*000000834*20260708~",
  "INS*Y*18*030*XN*A***FT~",
  "NM1*IL*1*Raw Enrollee****MI*adv-raw-834~",
  "K3*Rank: Iron~",
  "K3*Guild: Grim Foundations~",
  "K3*Region: Greenstone~",
  "HD*030**HLT~",
  "SE*8*000000834~",
  "GE*1*000000834~",
  "IEA*1*000000834~"
].join("\n");
const sampleRaw820 = [
  "ISA*00*          *00*          *ZZ*partner-greenstone*ZZ*Adventure Society*260708*1200*^*00501*000000820*0*T*:~",
  "GS*RA*partner-greenstone*Adventure Society*20260708*1200*000000820*X*005010X218~",
  "ST*820*000000820~",
  "BPR*C*50.00*C*ACH************20260708~",
  "TRN*1*000000820*partner-greenstone~",
  "N1*PR*Adventure Society~",
  "NM1*IL*1*adv-e2e-dashboard****MI*adv-e2e-dashboard~",
  "RMR*IK*adv-e2e-dashboard**50.00~",
  "SE*6*000000820~",
  "GE*1*000000820~",
  "IEA*1*000000820~"
].join("\n");
const sampleRawX12 = [
  "ISA*00*          *00*          *ZZ*provider-vitesse-temple*ZZ*Adventure Society*260708*1200*^*00501*000000777*0*T*:~",
  "GS*HC*provider-vitesse-temple*Adventure Society*20260708*1200*000000777*X*005010X837P~",
  "ST*837*000000777~",
  "BHT*0019*00*000000777*20260708*1200*CH~",
  "HL*1**20*1~",
  "NM1*41*2*provider-vitesse-temple*****46*provider-vitesse-temple~",
  "NM1*85*2*provider-vitesse-temple*****XX*provider-vitesse-temple~",
  "HL*2*1*22*0~",
  "NM1*IL*1*adv-e2e-dashboard****MI*adv-e2e-dashboard~",
  "CLM*claim-raw-demo*1250.00***11:B:1*Y*A*Y*I~",
  "HI*ABK:S062X9A~",
  "SV1*HC:ASHN1*1250.00*UN*1***1~",
  "SE*12*000000777~",
  "GE*1*000000777~",
  "IEA*1*000000777~"
].join("\n");
const sampleRaw837D = [
  "ISA*00*          *00*          *ZZ*provider-crown-dental*ZZ*Adventure Society*260708*1200*^*00501*0000837D0*0*T*:~",
  "GS*HC*provider-crown-dental*Adventure Society*20260708*1200*0000837D0*X*005010X224A2~",
  "ST*837D*0000837D0~",
  "BHT*0019*00*0000837D0*20260708*1200*CH~",
  "NM1*85*2*provider-crown-dental*****XX*provider-crown-dental~",
  "NM1*IL*1*Dental Ranger****MI*adv-e2e-dashboard~",
  "CLM*claim-raw-837d*850.00***11:B:1*Y*A*Y*I~",
  "HI*ABK:K021~",
  "SV3*AD:D7240*850.00*UN*1***1~",
  "TOO*JP*14~",
  "REF*D9*SURFACE-MO~",
  "REF*D9*QUADRANT-UR~",
  "CRC*ZZ*Y*ORTHO~",
  "SE*13*0000837D0~",
  "GE*1*0000837D0~",
  "IEA*1*0000837D0~"
].join("\n");
const sampleRaw270 = [
  "ISA*00*          *00*          *ZZ*provider-vitesse-temple*ZZ*Adventure Society*260708*1200*^*00501*000000270*0*T*:~",
  "GS*HS*provider-vitesse-temple*Adventure Society*20260708*1200*000000270*X*005010X279A1~",
  "ST*270*000000270~",
  "BHT*0022*13*000000270*20260708*1200~",
  "HL*1**20*1~",
  "NM1*1P*2*provider-vitesse-temple*****XX*provider-vitesse-temple~",
  "HL*2*1*22*0~",
  "NM1*IL*1*Filter Fixture Ranger****MI*adv-e2e-dashboard~",
  "EQ*30~",
  "SE*9*000000270~",
  "GE*1*000000270~",
  "IEA*1*000000270~"
].join("\n");
const sampleRaw269 = [
  "ISA*00*          *00*          *ZZ*provider-vitesse-temple*ZZ*Adventure Society*260708*1200*^*00501*000000269*0*T*:~",
  "GS*HS*provider-vitesse-temple*Adventure Society*20260708*1200*000000269*X*005010X269A1~",
  "ST*269*000000269~",
  "BHT*0022*13*000000269*20260708*1200~",
  "TRN*1*tx-raw-269*provider-vitesse-temple~",
  "HL*1**20*1~",
  "NM1*PR*2*Adventure Society*****PI*Adventure Society~",
  "NM1*PR*2*guild-secondary-plan*****PI*guild-secondary-plan~",
  "HL*2*1*21*1~",
  "NM1*1P*2*provider-vitesse-temple*****XX*provider-vitesse-temple~",
  "HL*3*2*22*0~",
  "NM1*IL*1*Coordination Ranger*****MI*adv-raw-269~",
  "REF*6P*Adventure Society~",
  "REF*2U*guild-secondary-plan~",
  "EQ*35~",
  "SE*15*000000269~",
  "GE*1*000000269~",
  "IEA*1*000000269~"
].join("\n");
const sampleRaw276 = [
  "ISA*00*          *00*          *ZZ*provider-vitesse-temple*ZZ*Adventure Society*260708*1200*^*00501*000000276*0*T*:~",
  "GS*HR*provider-vitesse-temple*Adventure Society*20260708*1200*000000276*X*005010X212~",
  "ST*276*000000276~",
  "BHT*0010*13*000000276*20260708*1200~",
  "HL*1**20*1~",
  "NM1*1P*2*provider-vitesse-temple*****XX*provider-vitesse-temple~",
  "HL*2*1*22*0~",
  "NM1*IL*1*Filter Fixture Ranger****MI*adv-e2e-dashboard~",
  "TRN*1*claim-e2e-dashboard*provider-vitesse-temple~",
  "REF*1K*claim-e2e-dashboard~",
  "SE*10*000000276~",
  "GE*1*000000276~",
  "IEA*1*000000276~"
].join("\n");
const sampleRaw278 = [
  "ISA*00*          *00*          *ZZ*provider-vitesse-temple*ZZ*Adventure Society*260708*1200*^*00501*000000278*0*T*:~",
  "GS*HI*provider-vitesse-temple*Adventure Society*20260708*1200*000000278*X*005010X217~",
  "ST*278*000000278~",
  "BHT*0007*13*000000278*20260708*1200~",
  "TRN*1*tx-e2e-raw-278*provider-vitesse-temple~",
  "HL*1**20*1~",
  "NM1*1P*2*provider-vitesse-temple*****XX*provider-vitesse-temple~",
  "HL*2*1*22*0~",
  "NM1*IL*1*Filter Fixture Ranger****MI*adv-e2e-dashboard~",
  "UM*AR*I*2***resurrection~",
  "HI*ABK:S062X9A~",
  "DTP*472*D8*20260708~",
  "SE*12*000000278~",
  "GE*1*000000278~",
  "IEA*1*000000278~"
].join("\n");
const sampleRaw835 = [
  "ISA*00*          *00*          *ZZ*Adventure Society*ZZ*provider-vitesse-temple*260708*1200*^*00501*000000835*0*T*:~",
  "GS*HP*Adventure Society*provider-vitesse-temple*20260708*1200*000000835*X*005010X221A1~",
  "ST*835*000000835~",
  "BPR*I*1000.00*C*CHK************20260708~",
  "TRN*1*000000835*Adventure Society~",
  "CLP*claim-e2e-dashboard*1*1250.00*1000.00*50.00*MC*000000835~",
  "CAS*CO*45*250.00~",
  "SE*5*000000835~",
  "GE*1*000000835~",
  "IEA*1*000000835~"
].join("\n");
const sampleRaw275 = [
  "ISA*00*          *00*          *ZZ*provider-vitesse-temple*ZZ*Adventure Society*260708*1200*^*00501*000000275*0*T*:~",
  "GS*HC*provider-vitesse-temple*Adventure Society*20260708*1200*000000275*X*006020X314~",
  "ST*275*000000275~",
  "BGN*11*tx-doc-request~",
  "NM1*1P*2*provider-vitesse-temple*****XX*provider-vitesse-temple~",
  "REF*1K*claim-e2e-dashboard~",
  "REF*6R*ATTACH-275~",
  "REF*F8*packet-raw-1-OF-2~",
  "DTP*472*D8*20260708~",
  "PWK*B4*EL***AC*ATTACH-275~",
  "CAT*B4*TXT~",
  "OOI*DOC*ATTACH-275~",
  "BDS*ASC**Content-Type: text/plain~",
  "LQ*AT*OZ~",
  "K3*Document-Reference: https://docs.example.test/raw-275.pdf~",
  "BIN*39*Raw 275 documentation packet accepted.~",
  "SE*16*000000275~",
  "GE*1*000000275~",
  "IEA*1*000000275~"
].join("\n");
const rawSampleGuides = [
  { type: "834", result: "Enrollment accepted", focus: "Creates an active member from INS/NM1/HD plus lore K3 fields." },
  { type: "820", result: "Premium accepted", focus: "Posts a payment that can make later adjudication benefit-current." },
  { type: "270", result: "Eligibility checked", focus: "Maps provider/member/service request into a 271-style eligibility answer." },
  { type: "269", result: "Coordination accepted", focus: "Shows primary and secondary payer coordination for dental service type 35." },
  { type: "276", result: "Claim status checked", focus: "Links inquiry trace and claim identifiers to a generated 277 response." },
  { type: "278", result: "Authorization queued", focus: "Starts async prior authorization review with service and diagnosis clues." },
  { type: "837", result: "Professional claim submitted", focus: "Submits diagnosis and ASHN procedure service lines for adjudication." },
  { type: "837D", result: "Dental claim submitted", focus: "Carries CDT, tooth, surface, quadrant, and orthodontic hints." },
  { type: "835", result: "Payment accepted", focus: "Applies remittance payment, adjustment, and patient responsibility detail." },
  { type: "275", result: "Attachment accepted", focus: "Demonstrates solicited BGN trace, CAT/BDS metadata, and document reference." }
];
const raw275RejectionFixtures: RejectionFixture[] = [
  {
    id: "invalid-bgn01",
    label: "275 Invalid BGN01",
    reason: "Invalid purpose indicator should reject before business review.",
    payload: raw275Fixture({
      control: "000275901",
      bgn01: "99",
      trace: "trace-bad-bgn",
      claimId: "claim-e2e-dashboard",
      attachmentControl: "ATTACH-BAD-BGN",
      cat02: "TXT",
      bds01: "ASC",
      content: "Invalid purpose fixture."
    })
  },
  {
    id: "invalid-cat02",
    label: "275 Invalid CAT02",
    reason: "Attachment format outside partner profile should emit 824 advice.",
    payload: raw275Fixture({
      control: "000275902",
      bgn01: "02",
      trace: "trace-bad-cat",
      claimId: "claim-e2e-dashboard",
      attachmentControl: "ATTACH-BAD-CAT",
      cat02: "BIN",
      bds01: "ASC",
      content: "Invalid format fixture."
    })
  },
  {
    id: "oversized-packet",
    label: "275 Oversized Packet",
    reason: "More LX loops than the partner allows should reject the packet.",
    payload: raw275Fixture({
      control: "000275903",
      bgn01: "02",
      trace: "trace-oversized",
      claimId: "claim-e2e-dashboard",
      attachmentControl: "ATTACH-TOO-MANY",
      cat02: "TXT",
      bds01: "ASC",
      content: "Oversized packet fixture.",
      attachmentCount: 4
    })
  },
  {
    id: "corrupt-base64",
    label: "275 Corrupt Base64",
    reason: "B64 content that cannot decode should reject with a clear payload error.",
    payload: raw275Fixture({
      control: "000275904",
      bgn01: "02",
      trace: "trace-bad-b64",
      claimId: "claim-e2e-dashboard",
      attachmentControl: "ATTACH-BAD-B64",
      cat02: "TXT",
      bds01: "B64",
      content: "not-valid-base64"
    })
  },
  {
    id: "missing-trace",
    label: "275 Missing Trace",
    reason: "Solicited attachments need the documentation request trace.",
    payload: raw275Fixture({
      control: "000275905",
      bgn01: "11",
      trace: "",
      claimId: "claim-e2e-dashboard",
      attachmentControl: "ATTACH-MISSING-TRACE",
      cat02: "TXT",
      bds01: "ASC",
      content: "Missing trace fixture."
    })
  },
  {
    id: "late-attachment",
    label: "275 Late Attachment",
    reason: "Unsolicited attachments outside the partner timing window should reject.",
    payload: raw275Fixture({
      control: "000275906",
      bgn01: "02",
      trace: "trace-late-attachment",
      claimId: "claim-e2e-dashboard",
      attachmentControl: "ATTACH-LATE",
      cat02: "TXT",
      bds01: "ASC",
      serviceDate: "20200101",
      content: "Late attachment fixture."
    })
  }
];
const savedFiltersStorageKey = "ashn.savedFilters.v1";
const scenarioRunsStorageKey = "ashn.scenarioRuns.v1";
const initialPartnerForm: PartnerFormState = {
  id: "",
  name: "",
  senderId: "",
  receiverId: "Adventure Society",
  allowedTransactionTypes: "270,275,276,278,837,837D",
  routeTarget: "payer-core",
  status: "active"
};
const dashboardTabs: { id: DashboardTab; label: string; detail: string }[] = [
  { id: "workflow", label: "Workflow", detail: "Run the demo flow" },
  { id: "metrics", label: "Metrics", detail: "Guild operations board" },
  { id: "timeline", label: "Timeline", detail: "Follow transaction chains" },
  { id: "ledger", label: "Ledger", detail: "Browse DB records" },
  { id: "xml", label: "XML Intake", detail: "Inspect inbound audits" },
  { id: "partners", label: "Partners", detail: "Review routing profiles" }
];
const filterTabs: DashboardTab[] = ["timeline", "ledger", "xml"];
const demoScenarios: DemoScenario[] = [
  {
    id: "premium-current-claim",
    title: "Premium-Current Claim Adjudication",
    outcome: "Shows how an accepted 820 premium changes async claim adjudication and appears in the related 277 explanation.",
    audience: "Stakeholder / payer operations demo",
    duration: "4–6 minutes",
    story: "An adventurer enrolls, pays guild dues, receives care, and the payer explains why the paid amount improved.",
    highlights: ["834 enrollment", "820 premium", "837 claim", "277 async adjudication", "835 payment"],
    steps: [
      { label: "Enroll", action: "Send 834 Enrollment", expected: "Adventurer appears with active coverage." },
      { label: "Premium", action: "Click 820 Pay Premium", expected: "Ledger records accepted premium dues." },
      { label: "Claim", action: "Submit 837 Claim and wait for tx-worker", expected: "Claim reaches Approved with premium-current adjudication context." },
      { label: "Explain", action: "Open the claim detail drawer", expected: "Adjudication Explanation shows premium current, paid amount, and patient responsibility." }
    ],
    exports: ["Ledger CSV", "277 JSON/XML/X12", "835 JSON/XML/X12"]
  },
  {
    id: "275-deficiency-resubmission",
    title: "275 Deficiency + Resubmission",
    outcome: "Demonstrates a payer requesting documentation, rejecting one document, and accepting a corrected resubmission.",
    audience: "Claims documentation / EDI education demo",
    duration: "6–8 minutes",
    story: "A high-value encounter needs supporting scrolls; one document is deficient and only that document is resubmitted.",
    highlights: ["277 documentation request", "275 packet", "per-document review", "deficiency follow-up", "targeted resubmission"],
    steps: [
      { label: "Create claim", action: "Enroll a scenario member and submit an 837 claim", expected: "Claim detail is ready for documentation work." },
      { label: "Request docs", action: "Click Request 275 Docs", expected: "Claim moves to Pending Documentation and emits a 277 request." },
      { label: "Submit packet", action: "Click Submit 275 Packet", expected: "Multiple 275 transactions share a packet ID." },
      { label: "Reject one", action: "Reject Encounter notes, then Request + Resubmit", expected: "A follow-up request and single replacement 275 are added." }
    ],
    exports: ["277 request JSON/XML/X12", "275 packet JSON/XML/X12", "Ledger CSV"]
  },
  {
    id: "dental-predetermination-to-remittance",
    title: "Dental Predetermination to Remittance",
    outcome: "Walks the dental-specific 270, 278, 275, 837D, and 835 path with CDT, tooth, attachment, and remittance detail.",
    audience: "Dental benefits / payer operations demo",
    duration: "6–8 minutes",
    story: "A temple checks dental benefits, requests predetermination, submits evidence, files an 837D claim, and receives CDT line-level remittance.",
    highlights: ["270 dental eligibility", "278 predetermination", "275 dental evidence packet", "837D dental claim", "835 CDT remittance"],
    steps: [
      { label: "Enroll", action: "Send 834 Enrollment", expected: "Dental scenario member has active coverage." },
      { label: "Benefits", action: "Check 270 → 271 Dental Eligibility", expected: "Annual max, remaining max, coverage percentages, waiting period, and frequency limits are visible." },
      { label: "Predetermine", action: "Request 278 Dental Predetermination", expected: "Manual review prompts request x-rays, perio chart, narrative, and treatment plan." },
      { label: "Evidence", action: "Submit Auth 275 Packet and accept required docs", expected: "Required dental evidence documents are linked and reviewed as Accepted." },
      { label: "Claim", action: "Submit 837D Dental Claim", expected: "CDT, tooth, surface, and quadrant detail appears in the claim transaction." },
      { label: "Remit", action: "Pay claim with 835", expected: "Dental remittance shows CDT service line, allowed amount, paid amount, patient responsibility, and references." }
    ],
    exports: ["270/271 JSON/XML/X12", "278 + 275 evidence JSON/XML/X12", "837D + 835 JSON/XML/X12"]
  },
  {
    id: "partner-rejection-ops",
    title: "Obsidian Companion-Guide Rejection",
    outcome: "Shows a strict professional-claims partner rejecting catastrophic diagnosis/procedure data, then accepting the corrected 837.",
    audience: "Integration / operations demo",
    duration: "3–5 minutes",
    story: "Obsidian Claims sends a professional claim outside its companion guide; operations inspect the rejection, correct the claim, and prove the audit trail.",
    highlights: ["Obsidian profile", "S062X9A rejected", "ASHN3 rejected", "corrected 837 accepted", "audit trend"],
    steps: [
      { label: "Reject", action: "Submit Obsidian 837 with S062X9A + ASHN3", expected: "Intake audit records a rejected companion-guide violation." },
      { label: "Correct", action: "Submit Obsidian 837 with T509 + ASHN2", expected: "Corrected professional claim is accepted and forwarded." },
      { label: "Trend", action: "Load rejection metrics for Obsidian diagnosis rules", expected: "Ops can see the companion-guide reason grouped in trends." },
      { label: "Drilldown", action: "Filter XML Intake to rejected 837s", expected: "Failed Obsidian payload is inspectable and replayable." }
    ],
    exports: ["Rejected audit JSON/XML", "Accepted 837 JSON/XML/X12", "999 JSON/XML/X12", "Rejection drilldown filters"]
  },
  {
    id: "275-rejection-fixtures",
    title: "275 Rejection Fixture Tour",
    outcome: "Submits companion-guide-inspired 275 failures and shows the audit/824 trail operators can inspect.",
    audience: "EDI analyst / QA demo",
    duration: "4–6 minutes",
    story: "A partner tests bad attachment payloads so analysts can see exactly how ASHN maps each failure into audit evidence.",
    highlights: ["invalid BGN01", "invalid CAT02", "oversized LX packet", "corrupt B64", "missing trace", "late attachment"],
    steps: [
      { label: "Submit fixtures", action: "Run all six 275 rejection samples", expected: "Each payload is rejected and recorded in intake audit." },
      { label: "Open metrics", action: "Load XML rejection metrics for 275", expected: "Dashboard groups the failures by reason." },
      { label: "Drilldown", action: "Switch XML Intake filters to rejected 275s", expected: "Operators can inspect and replay the failed payloads." }
    ],
    exports: ["275 raw X12 rejection samples", "824 JSON/XML/X12", "Inbound audit JSON/XML"]
  }
];

function providerLabel(providerId: string, providers: Provider[]) {
  if (providerId === "All") return "All";
  return providers.find((provider) => provider.id === providerId)?.name ?? providerId;
}

function partnerFromForm(form: PartnerFormState): TradingPartner {
  return {
    id: form.id.trim(),
    name: form.name.trim(),
    senderId: form.senderId.trim(),
    receiverId: form.receiverId.trim(),
    allowedTransactionTypes: form.allowedTransactionTypes.split(",").map((type) => type.trim()).filter(Boolean),
    routeTarget: form.routeTarget.trim() || "payer-core",
    status: form.status.trim() || "active"
  };
}

function buildQuery(values: Record<string, string | number>) {
  const params = new URLSearchParams();
  Object.entries(values).forEach(([key, value]) => {
    if (value === "" || value === "All") return;
    params.set(key, String(value));
  });
  return params.toString();
}

function pageSummary(page: PageInfo) {
  if (page.count === 0) return "Showing 0";
  return `Showing ${page.offset + 1}-${page.offset + page.count}`;
}

function loadSavedFilters(): SavedFilter[] {
  try {
    const raw = window.localStorage.getItem(savedFiltersStorageKey);
    if (!raw) return [];
    const parsed = JSON.parse(raw) as SavedFilter[];
    return Array.isArray(parsed) ? parsed : [];
  } catch {
    return [];
  }
}

function loadScenarioRuns(): ScenarioRunRecord[] {
  if (typeof window === "undefined") return [];
  try {
    const raw = window.localStorage.getItem(scenarioRunsStorageKey);
    return raw ? JSON.parse(raw) as ScenarioRunRecord[] : [];
  } catch {
    return [];
  }
}

function storeScenarioRuns(runs: ScenarioRunRecord[]) {
  window.localStorage.setItem(scenarioRunsStorageKey, JSON.stringify(runs));
}

function storeSavedFilters(filters: SavedFilter[]) {
  window.localStorage.setItem(savedFiltersStorageKey, JSON.stringify(filters));
}

function App() {
  const [health, setHealth] = useState<Envelope<Record<string, string>> | null>(null);
  const [readiness, setReadiness] = useState<ReadinessReport | null>(null);
  const [metrics, setMetrics] = useState<MetricsSummary | null>(null);
  const [providers, setProviders] = useState<Provider[]>([]);
  const [tradingPartners, setTradingPartners] = useState<TradingPartner[]>([]);
  const [selectedProviderId, setSelectedProviderId] = useState("provider-vitesse-temple");
  const [adventurer, setAdventurer] = useState<Adventurer | null>(null);
  const [claim, setClaim] = useState<Claim | null>(null);
  const [recentAdventurers, setRecentAdventurers] = useState<Adventurer[]>([]);
  const [recentClaims, setRecentClaims] = useState<Claim[]>([]);
  const [recentPremiumPayments, setRecentPremiumPayments] = useState<PremiumPayment[]>([]);
  const [recentTransactions, setRecentTransactions] = useState<Transaction[]>([]);
  const [inboundMessages, setInboundMessages] = useState<InboundMessage[]>([]);
  const [intakeRejectionMetrics, setIntakeRejectionMetrics] = useState<IntakeRejectionMetrics | null>(null);
  const [transactionJobs, setTransactionJobs] = useState<TransactionJob[]>([]);
  const [adventurerPage, setAdventurerPage] = useState<PageInfo>({ limit: adventurerPageSize, offset: 0, count: 0, hasMore: false });
  const [claimPage, setClaimPage] = useState<PageInfo>({ limit: claimPageSize, offset: 0, count: 0, hasMore: false });
  const [transactionPage, setTransactionPage] = useState<PageInfo>({ limit: transactionPageSize, offset: 0, count: 0, hasMore: false });
  const [auditPage, setAuditPage] = useState<PageInfo>({ limit: auditPageSize, offset: 0, count: 0, hasMore: false });
  const [adventurerOffset, setAdventurerOffset] = useState(0);
  const [claimOffset, setClaimOffset] = useState(0);
  const [transactionOffset, setTransactionOffset] = useState(0);
  const [auditOffset, setAuditOffset] = useState(0);
  const [selectedClaim, setSelectedClaim] = useState<Claim | null>(null);
  const [selectedTransaction, setSelectedTransaction] = useState<Transaction | null>(null);
  const [selectedInboundMessage, setSelectedInboundMessage] = useState<InboundMessage | null>(null);
  const [selectedPremiumPayment, setSelectedPremiumPayment] = useState<PremiumPayment | null>(null);
  const [authorizationTransaction, setAuthorizationTransaction] = useState<Transaction | null>(null);
  const [events, setEvents] = useState<Envelope[]>([]);
  const [busy, setBusy] = useState(false);
  const [searchTerm, setSearchTerm] = useState("");
  const [transactionTypeFilter, setTransactionTypeFilter] = useState("All");
  const [transactionStatusFilter, setTransactionStatusFilter] = useState("All");
  const [claimStatusFilter, setClaimStatusFilter] = useState("All");
  const [providerFilter, setProviderFilter] = useState("All");
  const [auditStatusFilter, setAuditStatusFilter] = useState("All");
  const [auditTypeFilter, setAuditTypeFilter] = useState("All");
  const [activeTab, setActiveTab] = useState<DashboardTab>("workflow");
  const [partnerForm, setPartnerForm] = useState<PartnerFormState>(initialPartnerForm);
  const [savedFilters, setSavedFilters] = useState<SavedFilter[]>(loadSavedFilters);
  const [savedFilterName, setSavedFilterName] = useState("");
  const [selectedSavedFilterId, setSelectedSavedFilterId] = useState("");
  const [payloadTab, setPayloadTab] = useState<PayloadTab>("json");
  const [rawX12Draft, setRawX12Draft] = useState(sampleRawX12);
  const [scenarioRuns, setScenarioRuns] = useState<Record<string, ScenarioRunState>>({});
  const [recentScenarioRuns, setRecentScenarioRuns] = useState<ScenarioRunRecord[]>(loadScenarioRuns);
  const scenarioRunEvidenceRef = useRef<Record<string, ScenarioStepEvidence[]>>({});
  const scenarioPlaybackContextRef = useRef<Record<string, ScenarioPlaybackContext>>({});

  const selectedProvider = useMemo(
    () => providers.find((provider) => provider.id === selectedProviderId),
    [providers, selectedProviderId]
  );

  const selectedPayloadView = useMemo(
    () => (selectedTransaction ? transactionPayloadView(selectedTransaction, payloadTab) : null),
    [payloadTab, selectedTransaction]
  );

  const selectedRelationshipMap = useMemo(
    () => (selectedTransaction ? buildTransactionRelationshipMap(selectedTransaction, recentTransactions) : null),
    [recentTransactions, selectedTransaction]
  );

  const selectedClaimAttachmentTransactions = useMemo(
    () => (selectedClaim ? claimAttachmentTransactions(selectedClaim, recentTransactions) : []),
    [recentTransactions, selectedClaim]
  );

  const selectedClaimAdjudication = useMemo(
    () => (selectedClaim ? latestClaimAdjudication(selectedClaim, recentTransactions) : null),
    [recentTransactions, selectedClaim]
  );

  const authorizationAttachmentTransactions = useMemo(
    () => (authorizationTransaction ? authorizationDocumentationTransactions(authorizationTransaction, recentTransactions) : []),
    [authorizationTransaction, recentTransactions]
  );

  const intakeRejectionSummary = useMemo(
    () => buildIntakeRejectionSummary(inboundMessages),
    [inboundMessages]
  );

  const providerFilters = useMemo(
    () => ["All", ...providers.map((provider) => provider.id)],
    [providers]
  );

  const timelineGroups = useMemo(
    () => buildTimelineGroups(recentTransactions),
    [recentTransactions]
  );

  const acknowledgmentDrilldowns = useMemo(
    () => buildAcknowledgmentDrilldowns(recentTransactions),
    [recentTransactions]
  );

  useEffect(() => {
    void refresh();
  }, [adventurerOffset, auditOffset, auditStatusFilter, auditTypeFilter, claimOffset, claimStatusFilter, providerFilter, searchTerm, transactionOffset, transactionStatusFilter, transactionTypeFilter]);

  useEffect(() => {
    const interval = window.setInterval(() => {
      void refresh();
    }, dashboardRefreshMs);
    return () => window.clearInterval(interval);
  }, [adventurerOffset, auditOffset, auditStatusFilter, auditTypeFilter, claimOffset, claimStatusFilter, providerFilter, searchTerm, transactionOffset, transactionStatusFilter, transactionTypeFilter]);

  useEffect(() => {
    storeSavedFilters(savedFilters);
  }, [savedFilters]);

  useEffect(() => {
    storeScenarioRuns(recentScenarioRuns);
  }, [recentScenarioRuns]);

  async function refresh(pushProviderEvent = false) {
    const adventurerQuery = buildQuery({ limit: adventurerPageSize, offset: adventurerOffset, q: searchTerm });
    const claimQuery = buildQuery({
      limit: claimPageSize,
      offset: claimOffset,
      q: searchTerm,
      status: claimStatusFilter,
      providerId: providerFilter
    });
    const transactionQuery = buildQuery({
      limit: transactionPageSize,
      offset: transactionOffset,
      q: searchTerm,
      type: transactionTypeFilter,
      status: transactionStatusFilter
    });
    const auditQuery = buildQuery({
      limit: auditPageSize,
      offset: auditOffset,
      q: searchTerm,
      status: auditStatusFilter,
      type: auditTypeFilter
    });
    const rejectionQuery = buildQuery({
      q: searchTerm,
      type: auditTypeFilter
    });
    const [healthResult, readinessResult, metricsResult, providersResult, partnersResult, adventurersResult, claimsResult, premiumPaymentsResult, transactionsResult, auditResult, rejectionResult, jobsResult] = await Promise.allSettled([
      request<Record<string, string>>("/v1/health"),
      request<ReadinessReport>("/v1/system/readiness"),
      request<MetricsSummary>("/v1/metrics/summary"),
      request<Provider[]>("/v1/providers"),
      request<TradingPartner[]>("/v1/x12/trading-partners"),
      request<Adventurer[]>(`/v1/adventurers?${adventurerQuery}`),
      request<Claim[]>(`/v1/claims?${claimQuery}`),
      request<PremiumPayment[]>("/v1/premium-payments?limit=5"),
      request<Transaction[]>(`/v1/transactions?${transactionQuery}`),
      request<InboundMessage[]>(`/v1/x12/messages?${auditQuery}`),
      request<IntakeRejectionMetrics>(`/v1/x12/messages/rejections?${rejectionQuery}`),
      request<TransactionJob[]>("/v1/jobs?limit=8")
    ]);
    const healthEnvelope = settledValue(healthResult);
    const readinessEnvelope = settledValue(readinessResult);
    const metricsEnvelope = settledValue(metricsResult);
    const providersEnvelope = settledValue(providersResult);
    const partnersEnvelope = settledValue(partnersResult);
    const adventurersEnvelope = settledValue(adventurersResult);
    const claimsEnvelope = settledValue(claimsResult);
    const premiumPaymentsEnvelope = settledValue(premiumPaymentsResult);
    const transactionsEnvelope = settledValue(transactionsResult);
    const auditEnvelope = settledValue(auditResult);
    const rejectionEnvelope = settledValue(rejectionResult);
    const jobsEnvelope = settledValue(jobsResult);
    if (healthEnvelope) setHealth(healthEnvelope);
    if (readinessEnvelope?.data) setReadiness(readinessEnvelope.data);
    if (metricsEnvelope?.data) setMetrics(metricsEnvelope.data);
    if (providersEnvelope) setProviders(providersEnvelope.data ?? []);
    if (partnersEnvelope) setTradingPartners(partnersEnvelope.data ?? []);
    if (adventurersEnvelope) {
      setRecentAdventurers(adventurersEnvelope.data ?? []);
      setAdventurerPage(adventurersEnvelope.page ?? { limit: adventurerPageSize, offset: adventurerOffset, count: adventurersEnvelope.data?.length ?? 0, hasMore: false });
    }
    if (claimsEnvelope) {
      setRecentClaims(claimsEnvelope.data ?? []);
      setClaimPage(claimsEnvelope.page ?? { limit: claimPageSize, offset: claimOffset, count: claimsEnvelope.data?.length ?? 0, hasMore: false });
    }
    if (premiumPaymentsEnvelope) setRecentPremiumPayments(premiumPaymentsEnvelope.data ?? []);
    if (transactionsEnvelope) {
      setRecentTransactions(transactionsEnvelope.data ?? []);
      setTransactionPage(transactionsEnvelope.page ?? { limit: transactionPageSize, offset: transactionOffset, count: transactionsEnvelope.data?.length ?? 0, hasMore: false });
    }
    if (auditEnvelope) {
      setInboundMessages(auditEnvelope.data ?? []);
      setAuditPage(auditEnvelope.page ?? { limit: auditPageSize, offset: auditOffset, count: auditEnvelope.data?.length ?? 0, hasMore: false });
    }
    if (rejectionEnvelope) setIntakeRejectionMetrics(rejectionEnvelope.data ?? null);
    if (jobsEnvelope) setTransactionJobs(jobsEnvelope.data ?? []);
    if (pushProviderEvent && providersEnvelope?.lore) {
      pushEvent(providersEnvelope);
    }
  }

  function resetLedgerOffsets() {
    setAdventurerOffset(0);
    setClaimOffset(0);
    setTransactionOffset(0);
    setAuditOffset(0);
  }

  function applyRejectionDrilldown(item: IntakeRejectionCount) {
    setAuditStatusFilter("rejected");
    setAuditTypeFilter(item.type || "All");
    setSearchTerm(item.query || item.partnerId || item.label);
    setAuditOffset(0);
    setActiveTab("xml");
  }

  function applyAcknowledgmentDrilldown(item: AcknowledgmentDrilldown) {
    setTransactionTypeFilter(item.typeFilter);
    setTransactionStatusFilter("All");
    setSearchTerm("");
    setTransactionOffset(0);
    setActiveTab("ledger");
  }

  function currentFilterState(name: string): SavedFilter {
    return {
      id: selectedSavedFilterId || `filter-${Date.now()}`,
      name: name.trim(),
      tab: activeTab,
      searchTerm,
      transactionType: transactionTypeFilter,
      transactionStatus: transactionStatusFilter,
      claimStatus: claimStatusFilter,
      provider: providerFilter,
      auditStatus: auditStatusFilter,
      auditType: auditTypeFilter
    };
  }

  function saveCurrentFilter() {
    const name = savedFilterName.trim() || `${activeTab} filter`;
    const filter = currentFilterState(name);
    setSavedFilters((current) => {
      const existingIndex = current.findIndex((item) => item.id === filter.id);
      if (existingIndex === -1) return [filter, ...current].slice(0, 12);
      return current.map((item, index) => (index === existingIndex ? filter : item));
    });
    setSelectedSavedFilterId(filter.id);
    setSavedFilterName(filter.name);
  }

  function applySavedFilter(id: string) {
    const filter = savedFilters.find((item) => item.id === id);
    if (!filter) return;
    setSelectedSavedFilterId(id);
    setSavedFilterName(filter.name);
    setActiveTab(filterTabs.includes(filter.tab) ? filter.tab : "ledger");
    setSearchTerm(filter.searchTerm);
    setTransactionTypeFilter(filter.transactionType);
    setTransactionStatusFilter(filter.transactionStatus);
    setClaimStatusFilter(filter.claimStatus);
    setProviderFilter(filter.provider);
    setAuditStatusFilter(filter.auditStatus);
    setAuditTypeFilter(filter.auditType);
    resetLedgerOffsets();
  }

  function deleteSavedFilter() {
    if (!selectedSavedFilterId) return;
    setSavedFilters((current) => current.filter((item) => item.id !== selectedSavedFilterId));
    setSelectedSavedFilterId("");
    setSavedFilterName("");
  }

  function clearFilters() {
    setSearchTerm("");
    setTransactionTypeFilter("All");
    setTransactionStatusFilter("All");
    setClaimStatusFilter("All");
    setProviderFilter("All");
    setAuditStatusFilter("All");
    setAuditTypeFilter("All");
    setSelectedSavedFilterId("");
    setSavedFilterName("");
    resetLedgerOffsets();
  }

  function settledValue<T>(result: PromiseSettledResult<Envelope<T>>) {
    return result.status === "fulfilled" ? result.value : undefined;
  }

  async function request<T>(path: string, init?: RequestInit): Promise<Envelope<T>> {
    const isFormData = init?.body instanceof FormData;
    const response = await fetch(`${apiUrl}${path}`, {
      ...init,
      headers: requestHeaders(init?.headers, isFormData)
    });
    return (await response.json()) as Envelope<T>;
  }

  function requestHeaders(initHeaders?: HeadersInit, skipContentType = false) {
    const headers = new Headers(initHeaders);
    if (!skipContentType && !headers.has("Content-Type")) {
      headers.set("Content-Type", "application/json");
    }
    if (apiKey && !headers.has("X-ASHN-API-Key")) {
      headers.set("X-ASHN-API-Key", apiKey);
    }
    return headers;
  }

  function pushEvent(event: Envelope) {
    setEvents((current) => [event, ...current].slice(0, 8));
  }

  function copyText(value: string) {
    void navigator.clipboard?.writeText(value);
  }

  function downloadText(filename: string, value: string) {
    const blob = new Blob([value], { type: "text/plain;charset=utf-8" });
    const url = URL.createObjectURL(blob);
    const anchor = document.createElement("a");
    anchor.href = url;
    anchor.download = filename;
    document.body.appendChild(anchor);
    anchor.click();
    anchor.remove();
    URL.revokeObjectURL(url);
  }

  function exportLedgerCSV() {
    downloadText("ashn-ledger-transactions.csv", transactionsToCSV(recentTransactions));
  }

  function exportDemoScenario(scenario: DemoScenario) {
    downloadText(`ashn-demo-scenario-${scenario.id}.json`, JSON.stringify(demoScenarioExport(scenario), null, 2));
  }

  function exportScenarioEvidence(scenario: DemoScenario) {
    const runState = scenarioRuns[scenario.id];
    if (!runState || runState.completedSteps < scenario.steps.length) {
      return;
    }
    downloadText(`ashn-demo-evidence-${scenario.id}.json`, JSON.stringify(demoScenarioEvidenceBundle(scenario, runState), null, 2));
  }

  function exportScenarioRunRecord(run: ScenarioRunRecord) {
    downloadText(`ashn-demo-evidence-${run.scenarioId}-${run.id}.json`, JSON.stringify(run.bundle, null, 2));
  }

  function copyScenarioRunTransactions(run: ScenarioRunRecord) {
    copyText(run.transactionIds.join("\n"));
  }

  function rememberScenarioRun(scenario: DemoScenario, runState: ScenarioRunState) {
    const bundle = demoScenarioEvidenceBundle(scenario, runState);
    const evidence = bundle.evidence;
    const runRecord: ScenarioRunRecord = {
      id: runState.runId ?? `scenario-${scenario.id}-${Date.now()}`,
      scenarioId: scenario.id,
      scenarioTitle: scenario.title,
      completedAt: runState.completedAt ?? new Date().toISOString(),
      completedSteps: runState.completedSteps,
      totalSteps: scenario.steps.length,
      status: bundle.run.status,
      transactionIds: evidence.transactionIds,
      claimIds: evidence.claimIds,
      adventurerIds: evidence.adventurerIds,
      bundle
    };
    setRecentScenarioRuns((current) => [runRecord, ...current.filter((item) => item.id !== runRecord.id)].slice(0, 10));
  }

  function copyDemoScenario(scenario: DemoScenario) {
    copyText(scenario.steps.map((step, index) => `${index + 1}. ${step.action} → ${step.expected}`).join("\n"));
  }

  function updateScenarioRun(scenario: DemoScenario, patch: Partial<ScenarioRunState>) {
    setScenarioRuns((current) => ({
      ...current,
      [scenario.id]: {
        ...current[scenario.id],
        running: current[scenario.id]?.running ?? false,
        completedSteps: current[scenario.id]?.completedSteps ?? 0,
        ...patch
      }
    }));
  }

  async function scenarioStep<T>(scenario: DemoScenario, stepIndex: number, action: () => Promise<Envelope<T>>) {
    const step = scenario.steps[stepIndex];
    updateScenarioRun(scenario, { running: true, currentStep: step.label, error: undefined });
    const result = await action();
    pushEvent(result);
    const evidence = scenarioStepEvidence(step, result);
    scenarioRunEvidenceRef.current[scenario.id] = [
      ...(scenarioRunEvidenceRef.current[scenario.id] ?? []),
      evidence
    ];
    setScenarioRuns((current) => ({
      ...current,
      [scenario.id]: {
        ...current[scenario.id],
        running: current[scenario.id]?.running ?? true,
        completedSteps: stepIndex + 1,
        currentStep: step.label,
        evidence: scenarioRunEvidenceRef.current[scenario.id]
      }
    }));
    return result;
  }

  function initializeScenarioRun(scenario: DemoScenario, mode: ScenarioRunState["mode"]) {
    const runId = `scenario-${scenario.id}-${Date.now()}`;
    const startedAt = new Date().toISOString();
    updateScenarioRun(scenario, {
      running: false,
      completedSteps: 0,
      mode,
      runId,
      startedAt,
      completedAt: undefined,
      currentStep: mode === "playback" ? `Ready: ${scenario.steps[0]?.label ?? "Complete"}` : "Starting",
      error: undefined,
      evidence: []
    });
    scenarioRunEvidenceRef.current[scenario.id] = [];
    scenarioPlaybackContextRef.current[scenario.id] = {};
    return { runId, startedAt };
  }

  function completeScenarioRun(scenario: DemoScenario, runId: string, startedAt: string) {
    const completedAt = new Date().toISOString();
    updateScenarioRun(scenario, { running: false, completedSteps: scenario.steps.length, currentStep: "Complete", completedAt });
    const finalRunState = {
      runId,
      startedAt,
      running: false,
      completedSteps: scenario.steps.length,
      currentStep: "Complete",
      completedAt,
      evidence: scenarioRunEvidenceRef.current[scenario.id]
    } as ScenarioRunState;
    rememberScenarioRun(scenario, finalRunState);
  }

  async function runDemoScenario(scenario: DemoScenario) {
    setBusy(true);
    const { runId, startedAt } = initializeScenarioRun(scenario, "auto");
    try {
      for (let stepIndex = 0; stepIndex < scenario.steps.length; stepIndex += 1) {
        await runDemoScenarioStep(scenario, stepIndex);
      }
      completeScenarioRun(scenario, runId, startedAt);
      await refresh(true);
    } catch (error) {
      updateScenarioRun(scenario, { running: false, error: error instanceof Error ? error.message : "Scenario failed" });
    } finally {
      setBusy(false);
    }
  }

  async function startScenarioPlayback(scenario: DemoScenario) {
    initializeScenarioRun(scenario, "playback");
  }

  async function runNextScenarioPlaybackStep(scenario: DemoScenario) {
    const runState = scenarioRuns[scenario.id];
    if (!runState || runState.completedSteps >= scenario.steps.length) return;
    setBusy(true);
    try {
      await runDemoScenarioStep(scenario, runState.completedSteps);
      const nextCompletedSteps = runState.completedSteps + 1;
      if (nextCompletedSteps >= scenario.steps.length) {
        completeScenarioRun(scenario, runState.runId ?? `scenario-${scenario.id}-${Date.now()}`, runState.startedAt ?? new Date().toISOString());
        await refresh(true);
      } else {
        updateScenarioRun(scenario, {
          running: false,
          mode: "playback",
          completedSteps: nextCompletedSteps,
          currentStep: `Ready: ${scenario.steps[nextCompletedSteps]?.label ?? "Complete"}`
        });
      }
    } catch (error) {
      updateScenarioRun(scenario, { running: false, error: error instanceof Error ? error.message : "Scenario playback failed" });
    } finally {
      setBusy(false);
    }
  }

  async function finishScenarioPlayback(scenario: DemoScenario) {
    const runState = scenarioRuns[scenario.id];
    if (!runState) return;
    setBusy(true);
    try {
      for (let stepIndex = runState.completedSteps; stepIndex < scenario.steps.length; stepIndex += 1) {
        await runDemoScenarioStep(scenario, stepIndex);
      }
      completeScenarioRun(scenario, runState.runId ?? `scenario-${scenario.id}-${Date.now()}`, runState.startedAt ?? new Date().toISOString());
      await refresh(true);
    } catch (error) {
      updateScenarioRun(scenario, { running: false, error: error instanceof Error ? error.message : "Scenario playback failed" });
    } finally {
      setBusy(false);
    }
  }

  async function runDemoScenarioStep(scenario: DemoScenario, stepIndex: number) {
    if (scenario.id === "premium-current-claim") {
      await runPremiumCurrentScenarioStep(scenario, stepIndex);
    } else if (scenario.id === "275-deficiency-resubmission") {
      await runDeficiencyScenarioStep(scenario, stepIndex);
    } else if (scenario.id === "dental-predetermination-to-remittance") {
      await runDentalScenarioStep(scenario, stepIndex);
    } else if (scenario.id === "partner-rejection-ops") {
      await runPartnerRejectionScenarioStep(scenario, stepIndex);
    } else if (scenario.id === "275-rejection-fixtures") {
      await run275RejectionFixtureScenarioStep(scenario, stepIndex);
    }
  }

  async function runPremiumCurrentScenarioStep(scenario: DemoScenario, stepIndex: number) {
    const context = scenarioPlaybackContextRef.current[scenario.id] ?? {};
    scenarioPlaybackContextRef.current[scenario.id] = context;
    if (stepIndex === 0) {
      const enrolled = await scenarioStep(scenario, 0, () => request<Adventurer>("/v1/adventurers", {
        method: "POST",
        body: JSON.stringify({
          name: `Scenario Premium ${new Date().toISOString().slice(11, 19)}`,
          rank: "Iron",
          guild: "Scenario Runner Guild",
          region: "Greenstone"
        })
      }));
      context.adventurer = requireScenarioData(enrolled.data, "Enrollment did not return an adventurer.");
      setAdventurer(context.adventurer);
      return;
    }
    const adventurerRecord = context.adventurer ?? adventurer;
    if (!adventurerRecord) throw new Error("Run enrollment before this scenario step.");
    if (stepIndex === 1) {
      await scenarioStep(scenario, 1, () => request<Record<string, string | number>>("/v1/premium-payments", {
        method: "POST",
        body: JSON.stringify({ adventurerId: adventurerRecord.id, amountCents: 5000 })
      }));
    } else if (stepIndex === 2) {
      const claimResult = await scenarioStep(scenario, 2, () => request<Claim>("/v1/claims", {
        method: "POST",
        body: JSON.stringify({
          adventurerId: adventurerRecord.id,
          providerId: "provider-greenstone-roadside",
          incidentSeverity: "Awakened",
          amountCents: 100000,
          serviceLines: [
            { lineNumber: 1, procedureCode: "ASHN1", description: "Scenario stabilization", units: 1, amountCents: 100000 }
          ]
        })
      }));
      context.claim = requireScenarioData(claimResult.data, "Claim submission did not return a claim.");
      setClaim(context.claim);
    } else if (stepIndex === 3) {
      const claimRecord = context.claim ?? claim;
      if (!claimRecord) throw new Error("Run claim submission before this scenario step.");
      await scenarioStep(scenario, 3, () => request<Claim>(`/v1/claims/${claimRecord.id}`));
    }
  }

  async function runDeficiencyScenarioStep(scenario: DemoScenario, stepIndex: number) {
    const context = scenarioPlaybackContextRef.current[scenario.id] ?? {};
    scenarioPlaybackContextRef.current[scenario.id] = context;
    if (stepIndex === 0) {
      const enrolled = await scenarioStep(scenario, 0, () => request<Adventurer>("/v1/adventurers", {
        method: "POST",
        body: JSON.stringify({
          name: `Scenario Docs ${new Date().toISOString().slice(11, 19)}`,
          rank: "Gold",
          guild: "Documentation Runner Guild",
          region: "Vitesse"
        })
      }));
      context.adventurer = requireScenarioData(enrolled.data, "Enrollment did not return an adventurer.");
      setAdventurer(context.adventurer);

      const claimResult = await request<Claim>("/v1/claims", {
        method: "POST",
        body: JSON.stringify({
          adventurerId: context.adventurer.id,
          providerId: selectedProviderId,
          incidentSeverity: "Awakened",
          amountCents: 125000
        })
      });
      pushEvent(claimResult);
      context.claim = requireScenarioData(claimResult.data, "Claim submission did not return a claim.");
      setClaim(context.claim);
      setSelectedClaim(context.claim);
      return;
    }
    const claimRecord = context.claim ?? claim;
    if (!claimRecord) throw new Error("Run claim creation before this scenario step.");
    if (stepIndex === 1) {
      await scenarioStep(scenario, 1, () => request<Record<string, string>>(`/v1/claims/${claimRecord.id}/documentation-request`, {
        method: "POST",
        body: JSON.stringify({
          reason: "Scenario runner documentation request.",
          dueDate: new Date(Date.now() + 7 * 24 * 60 * 60 * 1000).toISOString().slice(0, 10),
          requiredDocuments: documentationChecklist
        })
      }));
    } else if (stepIndex === 2) {
      const packetId = `scenario-packet-${claimRecord.id.slice(0, 8)}`;
      context.packet = await scenarioStep(scenario, 2, () => request<Record<string, string>>(`/v1/claims/${claimRecord.id}/attachments`, {
        method: "POST",
        body: JSON.stringify({
          packetId,
          attachments: documentationChecklist.map((item, index) => buildAttachmentDraft(claimRecord, item, packetId, index + 1, documentationChecklist.length))
        })
      }));
    } else if (stepIndex === 3) {
      const rejectedTransaction = context.packet?.transactions?.[1] ?? context.packet?.transaction;
      if (rejectedTransaction) {
        await scenarioStep(scenario, 3, () => request<Record<string, string>>(`/v1/transactions/${rejectedTransaction.id}/attachment-review`, {
          method: "POST",
          body: JSON.stringify({
            status: "Rejected",
            reason: "Scenario deficiency: encounter notes need corrected documentation."
          })
        }));
        const checklistItem = documentationChecklist[1];
        const resubmissionPacketId = `scenario-packet-${claimRecord.id.slice(0, 8)}-resub`;
        await request<Record<string, string>>(`/v1/claims/${claimRecord.id}/documentation-request`, {
          method: "POST",
          body: JSON.stringify({
            reason: `Deficiency follow-up: ${checklistItem.label} needs corrected documentation.`,
            dueDate: new Date(Date.now() + 5 * 24 * 60 * 60 * 1000).toISOString().slice(0, 10),
            requiredDocuments: [checklistItem]
          })
        }).then(pushEvent);
        await request<Record<string, string>>(`/v1/claims/${claimRecord.id}/attachments`, {
          method: "POST",
          body: JSON.stringify({
            packetId: resubmissionPacketId,
            attachments: [buildAttachmentDraft(claimRecord, checklistItem, resubmissionPacketId, 1, 1, "resubmission")]
          })
        }).then(pushEvent);
      }
    }
  }

  async function runDentalScenarioStep(scenario: DemoScenario, stepIndex: number) {
    const context = scenarioPlaybackContextRef.current[scenario.id] ?? {};
    scenarioPlaybackContextRef.current[scenario.id] = context;
    if (stepIndex === 0) {
      const enrolled = await scenarioStep(scenario, 0, () => request<Adventurer>("/v1/adventurers", {
        method: "POST",
        body: JSON.stringify({
          name: `Scenario Dental ${new Date().toISOString().slice(11, 19)}`,
          rank: "Gold",
          guild: "Molar Moon Guild",
          region: "Vitesse"
        })
      }));
      context.adventurer = requireScenarioData(enrolled.data, "Enrollment did not return an adventurer.");
      setAdventurer(context.adventurer);
      return;
    }
    const adventurerRecord = context.adventurer ?? adventurer;
    if (!adventurerRecord) throw new Error("Run enrollment before this scenario step.");
    if (stepIndex === 1) {
      await scenarioStep(scenario, 1, () => request<Record<string, string | number>>("/v1/eligibility/query", {
        method: "POST",
        body: JSON.stringify({ adventurerId: adventurerRecord.id, providerId: "provider-vitesse-temple", serviceType: "dental" })
      }));
    } else if (stepIndex === 2) {
      const authResult = await scenarioStep(scenario, 2, () => request<Record<string, string>>("/v1/auth-requests", {
        method: "POST",
        body: JSON.stringify({
          adventurerId: adventurerRecord.id,
          providerId: "provider-vitesse-temple",
          serviceType: "dental-predetermination",
          incidentSeverity: "Normal",
          dentalService: { cdtCode: "D7240", toothNumber: "14", surface: "MO", quadrant: "UR", orthodontic: false }
        })
      }));
      context.authorization = authResult.transaction ?? undefined;
      setAuthorizationTransaction(context.authorization ?? null);
    } else if (stepIndex === 3) {
      const authorization = context.authorization ?? authorizationTransaction;
      if (!authorization) throw new Error("Run dental predetermination before this scenario step.");
      const checklist = checklistForAuthorization(authorization);
      const packetId = `dental-auth-${authorization.id.slice(0, 8)}`;
      const packet = await scenarioStep(scenario, 3, () => request<Record<string, string>>(`/v1/auth-requests/${authorization.id}/attachments`, {
        method: "POST",
        body: JSON.stringify({
          packetId,
          attachments: checklist.map((item, index) => buildAuthorizationAttachmentDraft(authorization, item, packetId, index + 1, checklist.length))
        })
      }));
      context.packet = packet;
      const requiredDocuments = checklist.filter((item) => item.required);
      const packetTransactions = packet.transactions ?? [];
      for (let index = 0; index < requiredDocuments.length; index += 1) {
        const transaction = packetTransactions[index];
        if (!transaction) continue;
        await request<Record<string, string>>(`/v1/transactions/${transaction.id}/attachment-review`, {
          method: "POST",
          body: JSON.stringify({ status: "Accepted", reason: `${requiredDocuments[index].label} supports dental predetermination.` })
        }).then(pushEvent);
      }
    } else if (stepIndex === 4) {
      const claimResult = await scenarioStep(scenario, 4, () => request<Claim>("/v1/claims", {
        method: "POST",
        body: JSON.stringify({
          adventurerId: adventurerRecord.id,
          providerId: "provider-vitesse-temple",
          incidentSeverity: "Normal",
          amountCents: 85000,
          serviceLines: [
            { lineNumber: 1, procedureCode: "D7240", cdtCode: "D7240", description: "Surgical extraction of impacted tooth", units: 1, amountCents: 85000, toothNumber: "14", surface: "MO", quadrant: "UR" }
          ]
        })
      }));
      context.claim = requireScenarioData(claimResult.data, "Dental claim submission did not return a claim.");
      setClaim(context.claim);
      setSelectedClaim(context.claim);
    } else if (stepIndex === 5) {
      const claimRecord = context.claim ?? claim;
      if (!claimRecord) throw new Error("Run dental claim before this scenario step.");
      const payment = await scenarioStep(scenario, 5, () => request<Claim>(`/v1/claims/${claimRecord.id}/payment`, {
        method: "POST",
        body: JSON.stringify({ paymentAmountCents: claimRecord.paidAmountCents ?? 85000 })
      }));
      context.claim = payment.data ?? claimRecord;
      setClaim(context.claim);
    }
  }

  async function runPartnerRejectionScenarioStep(scenario: DemoScenario, stepIndex: number) {
    const invalidClaimXML = `<?xml version="1.0" encoding="UTF-8"?>
<AshnX12Transaction type="837">
  <Sender id="provider-obsidian-claims"/>
  <Receiver id="Adventure Society"/>
  <Claim>
    <AdventurerId>scenario-reject-member</AdventurerId>
    <ProviderId>provider-obsidian-claims</ProviderId>
    <IncidentSeverity>Awakened</IncidentSeverity>
    <AmountCents>10000</AmountCents>
    <Diagnosis qualifier="ABK" primary="true"><Code>S062X9A</Code></Diagnosis>
    <ServiceLine lineNumber="1"><ProcedureCode>ASHN3</ProcedureCode><AmountCents>10000</AmountCents></ServiceLine>
  </Claim>
</AshnX12Transaction>`;
    const correctedClaimXML = `<?xml version="1.0" encoding="UTF-8"?>
<AshnX12Transaction type="837">
  <Sender id="provider-obsidian-claims"/>
  <Receiver id="Adventure Society"/>
  <Claim>
    <AdventurerId>scenario-reject-member</AdventurerId>
    <ProviderId>provider-obsidian-claims</ProviderId>
    <IncidentSeverity>Awakened</IncidentSeverity>
    <AmountCents>10000</AmountCents>
    <Diagnosis qualifier="ABK" primary="true"><Code>T509</Code></Diagnosis>
    <ServiceLine lineNumber="1"><ProcedureCode>ASHN2</ProcedureCode><AmountCents>10000</AmountCents></ServiceLine>
  </Claim>
</AshnX12Transaction>`;
    if (stepIndex === 0) {
      await scenarioStep(scenario, 0, () => request("/v1/x12/xml", {
        method: "POST",
        headers: { "Content-Type": "application/xml" },
        body: invalidClaimXML
      }));
    } else if (stepIndex === 1) {
      await scenarioStep(scenario, 1, () => request("/v1/x12/xml", {
        method: "POST",
        headers: { "Content-Type": "application/xml" },
        body: correctedClaimXML
      }));
    } else if (stepIndex === 2) {
      await scenarioStep(scenario, 2, () => request<IntakeRejectionMetrics>("/v1/x12/messages/rejections?type=837&q=obsidian"));
    } else if (stepIndex === 3) {
      await scenarioStep(scenario, 3, () => request<InboundMessage[]>("/v1/x12/messages?status=rejected&type=837&q=obsidian&limit=5"));
      setActiveTab("xml");
      setAuditStatusFilter("rejected");
      setAuditTypeFilter("837");
      setSearchTerm("obsidian");
    }
  }

  async function run275RejectionFixtureScenarioStep(scenario: DemoScenario, stepIndex: number) {
    if (stepIndex === 0) {
      await scenarioStep(scenario, 0, submit275RejectionFixtures);
    } else if (stepIndex === 1) {
      await scenarioStep(scenario, 1, () => request<IntakeRejectionMetrics>("/v1/x12/messages/rejections?type=275"));
    } else if (stepIndex === 2) {
      await scenarioStep(scenario, 2, () => request<InboundMessage[]>("/v1/x12/messages?status=rejected&type=275&limit=10"));
      setActiveTab("xml");
      setAuditStatusFilter("rejected");
      setAuditTypeFilter("275");
      setSearchTerm("");
    }
  }

  async function submit275RejectionFixtures(): Promise<Envelope<{ submitted: number; fixtures: Array<{ id: string; reason: string; error?: string }> }>> {
    const results = [];
    for (const fixture of raw275RejectionFixtures) {
      const result = await request(`/v1/x12/raw`, {
        method: "POST",
        headers: { "Content-Type": "application/edi-x12" },
        body: fixture.payload
      });
      results.push({ id: fixture.id, reason: fixture.reason, error: result.error });
    }
    return {
      data: { submitted: results.length, fixtures: results },
      lore: `Submitted ${results.length} intentionally invalid 275 fixtures for audit review.`
    };
  }

  async function downloadFromPath(path: string) {
    const headers = new Headers();
    if (apiKey) {
      headers.set("X-ASHN-API-Key", apiKey);
    }
    const response = await fetch(`${apiUrl}${path}`, { headers });
    const blob = await response.blob();
    const url = URL.createObjectURL(blob);
    const anchor = document.createElement("a");
    anchor.href = url;
    anchor.download = downloadFilename(response, path);
    document.body.appendChild(anchor);
    anchor.click();
    anchor.remove();
    URL.revokeObjectURL(url);
  }

  async function inspectDocumentReference(transaction: Transaction) {
    setBusy(true);
    try {
      const result = await request<DocumentReference>(`/v1/transactions/${transaction.id}/document-reference`);
      pushEvent(result);
    } finally {
      setBusy(false);
    }
  }

  async function submitRawX12(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setBusy(true);
    try {
      const result = await request(`/v1/x12/raw`, {
        method: "POST",
        headers: { "Content-Type": "application/edi-x12" },
        body: rawX12Draft
      });
      pushEvent(result);
      await refresh(true);
    } finally {
      setBusy(false);
    }
  }

  async function submitBatchFiles(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const form = new FormData(event.currentTarget);
    if (!form.getAll("files").some((value) => value instanceof File && value.size > 0)) {
      return;
    }
    setBusy(true);
    try {
      const result = await request("/v1/x12/batch", {
        method: "POST",
        body: form
      });
      pushEvent(result);
      event.currentTarget.reset();
      await refresh(true);
    } finally {
      setBusy(false);
    }
  }

  async function saveTradingPartner(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setBusy(true);
    const partner = partnerFromForm(partnerForm);
    const path = partner.id ? `/v1/x12/trading-partners/${encodeURIComponent(partner.id)}` : "/v1/x12/trading-partners";
    const result = await request<TradingPartner>(path, {
      method: partner.id ? "PUT" : "POST",
      body: JSON.stringify(partner)
    });
    pushEvent(result);
    setPartnerForm(initialPartnerForm);
    await refresh();
    setBusy(false);
  }

  async function deleteTradingPartner(partnerId: string) {
    setBusy(true);
    const result = await request<Record<string, string>>(`/v1/x12/trading-partners/${encodeURIComponent(partnerId)}`, { method: "DELETE" });
    pushEvent(result);
    if (partnerForm.id === partnerId) setPartnerForm(initialPartnerForm);
    await refresh();
    setBusy(false);
  }

  function editTradingPartner(partner: TradingPartner) {
    setPartnerForm({
      id: partner.id,
      name: partner.name,
      senderId: partner.senderId,
      receiverId: partner.receiverId,
      allowedTransactionTypes: partner.allowedTransactionTypes.join(","),
      routeTarget: partner.routeTarget,
      status: partner.status
    });
  }

  async function replayTransaction(transactionId: string) {
    setBusy(true);
    const result = await request<Transaction>(`/v1/transactions/${transactionId}/replay`, { method: "POST" });
    pushEvent(result);
    await refresh();
    setBusy(false);
  }

  async function reviewAttachment(status: "Accepted" | "Rejected") {
    if (!selectedTransaction) return;
    await reviewAttachmentTransaction(selectedTransaction.id, status);
  }

  async function reviewAttachmentTransaction(transactionId: string, status: "Accepted" | "Rejected") {
    setBusy(true);
    const result = await request<Record<string, string>>(`/v1/transactions/${transactionId}/attachment-review`, {
      method: "POST",
      body: JSON.stringify({
        status,
        reason: status === "Accepted" ? "Supporting documentation satisfies review." : "Supporting documentation is insufficient for business review."
      })
    });
    if (result.transaction) {
      const reviewedTransaction = result.transaction;
      setSelectedTransaction((current) => current?.id === reviewedTransaction.id ? reviewedTransaction : current);
    }
    pushEvent(result);
    await refresh();
    setBusy(false);
  }

  async function replayInboundMessage(messageId: string) {
    setBusy(true);
    const result = await request(`/v1/x12/messages/${messageId}/replay`, { method: "POST" });
    pushEvent(result);
    await refresh();
    setBusy(false);
  }

  async function replayJob(jobId: string) {
    setBusy(true);
    const result = await request<TransactionJob>(`/v1/jobs/${jobId}/replay`, { method: "POST" });
    pushEvent(result);
    await refresh();
    setBusy(false);
  }

  async function enroll(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setBusy(true);
    const form = new FormData(event.currentTarget);
    const result = await request<Adventurer>("/v1/adventurers", {
      method: "POST",
      body: JSON.stringify({
        name: form.get("name"),
        rank: form.get("rank"),
        guild: form.get("guild"),
        region: form.get("region")
      })
    });
    setAdventurer(result.data ?? null);
    pushEvent(result);
    await refresh();
    setBusy(false);
  }

  async function checkEligibility() {
    await checkEligibilityRequest();
  }

  async function checkDentalEligibility() {
    await checkEligibilityRequest("dental");
  }

  async function checkEligibilityRequest(serviceType?: string) {
    if (!adventurer) return;
    setBusy(true);
    const result = await request("/v1/eligibility", {
      method: "POST",
      body: JSON.stringify({ adventurerId: adventurer.id, providerId: selectedProviderId, serviceType })
    });
    pushEvent(result);
    await refresh();
    setBusy(false);
  }

  async function payPremium() {
    if (!adventurer) return;
    setBusy(true);
    const result = await request<Record<string, string | number>>("/v1/premium-payments", {
      method: "POST",
      body: JSON.stringify({ adventurerId: adventurer.id, amountCents: 5000 })
    });
    pushEvent(result);
    await refresh();
    setBusy(false);
  }

  async function requestAuth() {
    await requestAuthorization({
      serviceType: "resurrection",
      incidentSeverity: "Diamond"
    });
  }

  async function requestDentalPredetermination() {
    await requestAuthorization({
      serviceType: "dental-predetermination",
      incidentSeverity: "Normal",
      dentalService: {
        cdtCode: "D7240",
        toothNumber: "14",
        surface: "MO",
        quadrant: "UR",
        orthodontic: false
      }
    });
  }

  async function requestAuthorization(body: Record<string, unknown>) {
    if (!adventurer) return;
    setBusy(true);
    const result = await request<Record<string, string>>("/v1/auth-requests", {
      method: "POST",
      body: JSON.stringify({
        adventurerId: adventurer.id,
        providerId: selectedProviderId,
        ...body
      })
    });
    setAuthorizationTransaction(result.transaction ?? null);
    pushEvent(result);
    await refresh();
    setBusy(false);
  }

  async function decideAuthorization(decision: "Approved" | "Denied") {
    if (!authorizationTransaction) return;
    setBusy(true);
    const result = await request<Record<string, string>>(`/v1/auth-requests/${authorizationTransaction.id}/decision`, {
      method: "POST",
      body: JSON.stringify({
        decision,
        reason: decision === "Approved" ? "Manual review approved resurrection medical necessity." : "Manual review denied pending additional documentation."
      })
    });
    if (result.transaction) {
      setAuthorizationTransaction(result.transaction);
    }
    pushEvent(result);
    await refresh();
    setBusy(false);
  }

  async function attachAuthorizationDocumentation() {
    if (!authorizationTransaction) return;
    const checklist = checklistForAuthorization(authorizationTransaction);
    const item = checklist[0];
    setBusy(true);
    const result = await request<Record<string, string>>(`/v1/auth-requests/${authorizationTransaction.id}/attachments`, {
      method: "POST",
      body: JSON.stringify({
        attachmentType: item.attachmentType,
        attachmentControlNumber: `ATTACH-${item.code}-${authorizationTransaction.id.slice(0, 8).toUpperCase()}`,
        reportTypeCode: item.reportTypeCode,
        transmissionCode: "EL",
        contentType: item.contentType,
        description: `${item.label} for authorization`,
        content: `${item.label} supporting document for authorization ${authorizationTransaction.id}.`
      })
    });
    pushEvent(result);
    await refresh();
    setBusy(false);
  }

  async function submitAuthorizationDocumentationPacket() {
    if (!authorizationTransaction) return;
    setBusy(true);
    const packetId = `auth-packet-${authorizationTransaction.id.slice(0, 8)}`;
    const checklist = checklistForAuthorization(authorizationTransaction);
    const result = await request<Record<string, string>>(`/v1/auth-requests/${authorizationTransaction.id}/attachments`, {
      method: "POST",
      body: JSON.stringify({
        packetId,
        attachments: checklist.map((item, index) => buildAuthorizationAttachmentDraft(authorizationTransaction, item, packetId, index + 1, checklist.length))
      })
    });
    pushEvent(result);
    await refresh();
    setBusy(false);
  }

  async function submitClaim() {
    await submitClaimRequest({
      incidentSeverity: "Awakened",
      amountCents: 125000,
      diagnoses: [
        {
          qualifier: "ABK",
          code: "T509",
          description: "Awakened injury stabilization",
          primary: true
        },
        {
          qualifier: "ABF",
          code: "S610",
          description: "Minor wound encounter"
        }
      ],
      serviceLines: [
        {
          lineNumber: 1,
          procedureCode: "ASHN1",
          description: "Resurrection stabilization",
          units: 1,
          amountCents: 95000
        },
        {
          lineNumber: 2,
          procedureCode: "ASHN2",
          description: "Dragonfire trauma supplies",
          units: 1,
          amountCents: 30000
        }
      ]
    });
  }

  async function submitDentalClaim() {
    await submitClaimRequest({
      incidentSeverity: "Normal",
      amountCents: 85000,
      diagnoses: [
        {
          qualifier: "ABK",
          code: "K021",
          description: "Dental caries requiring surgical extraction",
          primary: true
        }
      ],
      serviceLines: [
        {
          lineNumber: 1,
          procedureCode: "D7240",
          cdtCode: "D7240",
          description: "Removal of impacted tooth",
          units: 1,
          amountCents: 85000,
          toothNumber: "14",
          surface: "MO",
          quadrant: "UR",
          orthodontic: false
        }
      ]
    });
  }

  async function submitClaimRequest(body: Record<string, unknown>) {
    if (!adventurer) return;
    setBusy(true);
    const result = await request<Claim>("/v1/claims", {
      method: "POST",
      body: JSON.stringify({
        adventurerId: adventurer.id,
        providerId: selectedProviderId,
        authorizationTransactionId: authorizationTransaction?.id,
        ...body
      })
    });
    setClaim(result.data ?? null);
    pushEvent(result);
    await refresh();
    setBusy(false);
  }

  async function payClaim() {
    if (!claim) return;
    setBusy(true);
    const result = await request<Claim>(`/v1/claims/${claim.id}/payment`, {
      method: "POST",
      body: JSON.stringify({ paymentAmountCents: 100000 })
    });
    setClaim(result.data ?? null);
    pushEvent(result);
    await refresh();
    setBusy(false);
  }

  async function requestClaimDocumentation() {
    if (!selectedClaim) return;
    setBusy(true);
    const dueDate = new Date(Date.now() + 7 * 24 * 60 * 60 * 1000).toISOString().slice(0, 10);
    const result = await request<Record<string, string>>(`/v1/claims/${selectedClaim.id}/documentation-request`, {
      method: "POST",
      body: JSON.stringify({
        reason: "Medical necessity, encounter detail, and itemization required before adjudication.",
        dueDate,
        requiredDocuments: documentationChecklist
      })
    });
    pushEvent(result);
    const refreshed = await request<Claim>(`/v1/claims/${selectedClaim.id}`);
    if (refreshed.data) {
      setSelectedClaim(refreshed.data);
    }
    await refresh();
    setBusy(false);
  }

  async function submitClaimDocumentationPacket() {
    if (!selectedClaim) return;
    setBusy(true);
    const packetId = `packet-${selectedClaim.id.slice(0, 8)}`;
    const result = await request<Record<string, string>>(`/v1/claims/${selectedClaim.id}/attachments`, {
      method: "POST",
      body: JSON.stringify({
        packetId,
        attachments: documentationChecklist.map((item, index) => buildAttachmentDraft(selectedClaim, item, packetId, index + 1, documentationChecklist.length))
      })
    });
    pushEvent(result);
    const refreshed = await request<Claim>(`/v1/claims/${selectedClaim.id}`);
    if (refreshed.data) {
      setSelectedClaim(refreshed.data);
    }
    await refresh();
    setBusy(false);
  }

  async function requestDeficiencyAndResubmit(transaction: Transaction) {
    if (!selectedClaim) return;
    const checklistItem = checklistItemForTransaction(transaction);
    setBusy(true);
    const dueDate = new Date(Date.now() + 3 * 24 * 60 * 60 * 1000).toISOString().slice(0, 10);
    const deficiency = await request<Record<string, string>>(`/v1/claims/${selectedClaim.id}/documentation-request`, {
      method: "POST",
      body: JSON.stringify({
        reason: `Deficiency follow-up: ${checklistItem.label} needs corrected documentation.`,
        dueDate,
        requiredDocuments: [checklistItem]
      })
    });
    pushEvent(deficiency);
    const packetId = `${payloadString(transaction, "packetId") ?? `packet-${selectedClaim.id.slice(0, 8)}`}-resub`;
    const resubmission = await request<Record<string, string>>(`/v1/claims/${selectedClaim.id}/attachments`, {
      method: "POST",
      body: JSON.stringify({
        packetId,
        attachments: [buildAttachmentDraft(selectedClaim, checklistItem, packetId, 1, 1, "resubmission")]
      })
    });
    pushEvent(resubmission);
    const refreshed = await request<Claim>(`/v1/claims/${selectedClaim.id}`);
    if (refreshed.data) {
      setSelectedClaim(refreshed.data);
    }
    await refresh();
    setBusy(false);
  }

  async function openClaimDetail(claimId: string) {
    setBusy(true);
    const result = await request<Claim>(`/v1/claims/${claimId}`);
    if (result.data) {
      setSelectedClaim(result.data);
      setSelectedTransaction(null);
      setSelectedInboundMessage(null);
      setSelectedPremiumPayment(null);
    }
    setBusy(false);
  }

  async function openTransactionDetail(transactionId: string) {
    setBusy(true);
    const result = await request<Transaction>(`/v1/transactions/${transactionId}`);
    const transaction = result.transaction ?? result.data;
    if (transaction) {
      setSelectedTransaction(transaction);
      setPayloadTab("json");
      setSelectedClaim(null);
      setSelectedInboundMessage(null);
      setSelectedPremiumPayment(null);
    }
    setBusy(false);
  }

  async function openPremiumPaymentDetail(paymentId: string) {
    setBusy(true);
    const result = await request<PremiumPayment>(`/v1/premium-payments/${paymentId}`);
    if (result.data) {
      setSelectedPremiumPayment(result.data);
      setSelectedClaim(null);
      setSelectedTransaction(null);
      setSelectedInboundMessage(null);
    }
    setBusy(false);
  }

  function openInboundMessageDetail(message: InboundMessage) {
    setSelectedInboundMessage(message);
    setSelectedClaim(null);
    setSelectedTransaction(null);
    setSelectedPremiumPayment(null);
  }

  function closeDetail() {
    setSelectedClaim(null);
    setSelectedTransaction(null);
    setSelectedInboundMessage(null);
    setSelectedPremiumPayment(null);
  }

  return (
    <main>
      <section className="hero">
        <div>
          <p className="eyebrow">Adventure Society Health Network</p>
          <h1>ASHN Transaction Dashboard</h1>
          <p>
            Enroll adventurers, verify eligibility, request resurrection authorization,
            and send claim/remittance transactions through the gateway.
          </p>
        </div>
        <div className="status-card">
          <span className="rune">◆</span>
          <div className="gateway-header">
            <div>
              <h2>Gateway Skill Tree</h2>
              <a className="gateway-url" href={apiUrl} target="_blank" rel="noreferrer">{apiUrl}</a>
            </div>
            <span className="gateway-badge">Live</span>
          </div>
          <div className="gateway-tree" aria-label="Gateway service health diagram">
            <div className="gateway-core">
              <span className="gateway-diamond ok" />
              <strong>API Gateway</strong>
              <small>Routing Core</small>
            </div>
            <div className="gateway-branches">
              {Object.entries(health?.data ?? {}).map(([service, status], index) => (
                <div key={service} className={`gateway-node node-${index}`}>
                  <span className={status === "ok" ? "gateway-diamond ok" : "gateway-diamond bad"} />
                  <strong>{service}</strong>
                  <small>{status}</small>
                </div>
              ))}
              {Object.keys(health?.data ?? {}).length === 0 && (
                <div className="gateway-node node-0">
                  <span className="gateway-diamond bad" />
                  <strong>Awaiting Signal</strong>
                  <small>loading</small>
                </div>
              )}
            </div>
          </div>
          <ReadinessPanel readiness={readiness} />
        </div>
      </section>

      <section className="stats-grid">
        <MetricCard label="Adventurers" value={adventurerPage.count} detail={pageSummary(adventurerPage)} />
        <MetricCard label="Claims" value={claimPage.count} detail={`${recentClaims.filter((item) => item.status === "Paid").length} paid on this page`} />
        <MetricCard label="Transactions" value={transactionPage.count} detail={`${recentTransactions.length} ledger entries loaded`} />
      </section>

      <nav className="tab-nav" aria-label="Dashboard sections">
        {dashboardTabs.map((tab) => (
          <button
            key={tab.id}
            className={activeTab === tab.id ? "tab-button active" : "tab-button"}
            onClick={() => setActiveTab(tab.id)}
          >
            <strong>{tab.label}</strong>
            <span>{tab.detail}</span>
          </button>
        ))}
      </nav>

      {activeTab === "metrics" && (
        <MetricsCockpit metrics={metrics} />
      )}

      {activeTab === "partners" && (
      <section className="panel trading-panel">
        <div className="ledger-title">
          <div>
            <h2>Trading Partners</h2>
            <p className="muted">Create, update, delete, and inspect sender/receiver routing profiles.</p>
          </div>
          <span className="muted">{tradingPartners.length} profiles</span>
        </div>
        <form className="partner-form" onSubmit={saveTradingPartner}>
          <label>
            Partner name
            <input value={partnerForm.name} onChange={(event) => setPartnerForm({ ...partnerForm, name: event.target.value })} placeholder="Crystal Tower Partner" />
          </label>
          <label>
            Sender ID
            <input value={partnerForm.senderId} onChange={(event) => setPartnerForm({ ...partnerForm, senderId: event.target.value })} placeholder="provider-crystal-tower" />
          </label>
          <label>
            Receiver ID
            <input value={partnerForm.receiverId} onChange={(event) => setPartnerForm({ ...partnerForm, receiverId: event.target.value })} />
          </label>
          <label>
            Allowed X12 types
            <input value={partnerForm.allowedTransactionTypes} onChange={(event) => setPartnerForm({ ...partnerForm, allowedTransactionTypes: event.target.value })} />
          </label>
          <label>
            Status
            <select value={partnerForm.status} onChange={(event) => setPartnerForm({ ...partnerForm, status: event.target.value })}>
              <option value="active">active</option>
              <option value="inactive">inactive</option>
            </select>
          </label>
          <div className="actions compact-actions">
            <button disabled={busy}>{partnerForm.id ? "Update Partner" : "Create Partner"}</button>
            <button className="secondary" type="button" onClick={() => setPartnerForm(initialPartnerForm)}>Clear</button>
          </div>
        </form>
        <div className="partner-grid">
          {tradingPartners.length === 0 ? (
            <p className="muted">No trading partner profiles are loaded.</p>
          ) : (
            tradingPartners.map((partner) => <TradingPartnerCard key={partner.id} partner={partner} busy={busy} onEdit={editTradingPartner} onDelete={deleteTradingPartner} />)
          )}
        </div>
      </section>
      )}

      {activeTab === "workflow" && (
      <>
      <section className="grid">
        <div className="panel">
          <h2>1. Enroll Adventurer</h2>
          <form onSubmit={enroll}>
            <label>
              Name
              <input name="name" defaultValue="Farros" />
            </label>
            <label>
              Rank
              <select name="rank" defaultValue="Iron">
                {["Iron", "Bronze", "Silver", "Gold", "Diamond"].map((rank) => (
                  <option key={rank}>{rank}</option>
                ))}
              </select>
            </label>
            <label>
              Guild
              <input name="guild" defaultValue="Grim Foundations" />
            </label>
            <label>
              Region
              <select name="region" defaultValue="Greenstone">
                {["Greenstone", "Yaresh", "Rimaros", "Vitesse"].map((region) => (
                  <option key={region}>{region}</option>
                ))}
              </select>
            </label>
            <button disabled={busy}>Send 834 Enrollment</button>
          </form>
          {adventurer && (
            <div className="result">
              <strong>{adventurer.name}</strong>
              <span>{adventurer.rank} · {adventurer.coverageStatus}</span>
              <code>{adventurer.id}</code>
              <button type="button" className="secondary" disabled={busy} onClick={payPremium}>820 Pay Premium</button>
            </div>
          )}
        </div>

        <div className="panel">
          <h2>2. Provider + Workflow</h2>
          <label>
            Treating provider
            <select value={selectedProviderId} onChange={(event) => setSelectedProviderId(event.target.value)}>
              {providers.map((provider) => (
                <option value={provider.id} key={provider.id}>{provider.name}</option>
              ))}
            </select>
          </label>
          {selectedProvider && (
            <div className="provider-card">
              <h3>{selectedProvider.name}</h3>
              <p>{selectedProvider.providerType} · {selectedProvider.tierRank} · {selectedProvider.region}</p>
            </div>
          )}
          <div className="actions">
            <button disabled={!adventurer || busy} onClick={checkEligibility}>270 → 271 Eligibility</button>
            <button disabled={!adventurer || busy} onClick={checkDentalEligibility}>270 → 271 Dental Eligibility</button>
            <button disabled={!adventurer || busy} onClick={requestAuth}>278 Resurrection Auth</button>
            <button disabled={!adventurer || busy} onClick={requestDentalPredetermination}>278 Dental Predetermination</button>
            <button disabled={!adventurer || busy} onClick={submitClaim}>837 Submit Claim</button>
            <button disabled={!adventurer || busy} onClick={submitDentalClaim}>837D Submit Dental Claim</button>
            <button disabled={!claim || busy} onClick={payClaim}>835 Pay Claim</button>
          </div>
          {authorizationTransaction && (
            <div className="auth-review-card">
              <div>
                <span className="eyebrow">Prior Auth Review</span>
                <strong>278 · {authorizationTransaction.status}</strong>
                <code>{authorizationTransaction.id}</code>
              </div>
              <p>{authorizationReviewSummary(authorizationTransaction)}</p>
              {payloadNestedString(authorizationTransaction, "dentalService", "cdtCode") && (
                <div className="chips">
                  <span>CDT {payloadNestedString(authorizationTransaction, "dentalService", "cdtCode")}</span>
                  {payloadNestedString(authorizationTransaction, "dentalService", "toothNumber") && <span>Tooth {payloadNestedString(authorizationTransaction, "dentalService", "toothNumber")}</span>}
                  {payloadNestedString(authorizationTransaction, "dentalService", "surface") && <span>Surface {payloadNestedString(authorizationTransaction, "dentalService", "surface")}</span>}
                  {payloadNestedString(authorizationTransaction, "dentalService", "quadrant") && <span>{payloadNestedString(authorizationTransaction, "dentalService", "quadrant")}</span>}
                </div>
              )}
              <div className="actions compact-actions">
                <button disabled={busy} onClick={attachAuthorizationDocumentation}>Send 275 Auth Docs</button>
                <button disabled={busy} onClick={submitAuthorizationDocumentationPacket}>Submit Auth 275 Packet</button>
                <button disabled={busy || authorizationTransaction.status !== "Pending"} onClick={() => decideAuthorization("Approved")}>Approve Auth</button>
                <button className="danger" disabled={busy || authorizationTransaction.status !== "Pending"} onClick={() => decideAuthorization("Denied")}>Deny Auth</button>
              </div>
              <AuthorizationDocumentationWorkbench
                authorizationTransaction={authorizationTransaction}
                checklist={checklistForAuthorization(authorizationTransaction)}
                attachmentTransactions={authorizationAttachmentTransactions}
                busy={busy}
                onReview={reviewAttachmentTransaction}
              />
            </div>
          )}
          {claim && (
            <div className="result">
              <strong>Claim {claim.status}</strong>
              <span>{claim.incidentSeverity} · ${(claim.amountCents / 100).toLocaleString()}</span>
              <code>{claim.id}</code>
            </div>
          )}
        </div>
      </section>

      <section className="panel ledger">
        <div className="ledger-title">
          <h2>Live Session Events</h2>
          <button onClick={() => refresh(true)} disabled={busy}>Refresh</button>
        </div>
        {events.length === 0 ? (
          <p className="muted">No transactions yet. The Society scribe is sharpening a quill.</p>
        ) : (
          events.map((event, index) => <LedgerEvent key={index} event={event} />)
        )}
      </section>

      <section className="panel ledger">
        <div className="ledger-title">
          <div>
            <h2>Async Worker Queue</h2>
            <p className="muted">Queued 278 reviews and claim adjudication jobs with retry/dead-letter status.</p>
          </div>
          <span className="muted">{transactionJobs.length} recent jobs</span>
        </div>
        {transactionJobs.length === 0 ? (
          <p className="muted">No queued jobs are visible yet. The worker campfire is quiet.</p>
        ) : (
          transactionJobs.map((job) => <JobRow key={job.id} job={job} busy={busy} onReplay={replayJob} />)
        )}
      </section>

      <section className="panel scenario-library">
        <div className="ledger-title">
          <div>
            <h2>Exportable Demo Scenarios</h2>
            <p className="muted">Download repeatable runbooks for stakeholder walkthroughs, training, and regression demos.</p>
          </div>
          <span className="muted">{demoScenarios.length} runbooks</span>
        </div>
        <div className="scenario-grid">
          {demoScenarios.map((scenario) => (
            <DemoScenarioCard
              key={scenario.id}
              scenario={scenario}
              runState={scenarioRuns[scenario.id]}
              busy={busy}
              onExport={exportDemoScenario}
              onExportEvidence={exportScenarioEvidence}
              onCopy={copyDemoScenario}
              onRun={runDemoScenario}
              onStartPlayback={startScenarioPlayback}
              onRunNextStep={runNextScenarioPlaybackStep}
              onFinishPlayback={finishScenarioPlayback}
            />
          ))}
        </div>
        <RecentScenarioRuns
          runs={recentScenarioRuns}
          onExport={exportScenarioRunRecord}
          onCopyTransactions={copyScenarioRunTransactions}
          onRerun={(scenarioId) => {
            const scenario = demoScenarios.find((item) => item.id === scenarioId);
            if (scenario) void runDemoScenario(scenario);
          }}
          busy={busy}
        />
      </section>
      </>
      )}

      {filterTabs.includes(activeTab) && (
      <section className="panel filters-panel">
        <div className="ledger-title">
          <h2>Search & Filters</h2>
          <button className="secondary" onClick={clearFilters}>Clear</button>
        </div>
        <div className="saved-filter-bar">
          <label>
            Saved filters
            <select value={selectedSavedFilterId} onChange={(event) => applySavedFilter(event.target.value)}>
              <option value="">Choose preset</option>
              {savedFilters.map((filter) => (
                <option value={filter.id} key={filter.id}>{filter.name}</option>
              ))}
            </select>
          </label>
          <label>
            Filter name
            <input value={savedFilterName} onChange={(event) => setSavedFilterName(event.target.value)} placeholder="High-value 837s" />
          </label>
          <div className="actions compact-actions">
            <button type="button" onClick={saveCurrentFilter}>Save Filter</button>
            <button className="danger" type="button" disabled={!selectedSavedFilterId} onClick={deleteSavedFilter}>Delete Saved</button>
          </div>
        </div>
        <div className="filters-grid">
          <label className="wide-filter">
            Search ledger
            <input
              value={searchTerm}
              onChange={(event) => {
                setSearchTerm(event.target.value);
                resetLedgerOffsets();
              }}
              placeholder="Search IDs, names, statuses, providers..."
            />
          </label>
          <label>
            Transaction type
            <select value={transactionTypeFilter} onChange={(event) => {
              setTransactionTypeFilter(event.target.value);
              setTransactionOffset(0);
            }}>
              {transactionTypes.map((type) => <option key={type}>{type}</option>)}
            </select>
          </label>
          <label>
            Transaction status
            <select value={transactionStatusFilter} onChange={(event) => {
              setTransactionStatusFilter(event.target.value);
              setTransactionOffset(0);
            }}>
              {transactionStatuses.map((status) => <option key={status}>{status}</option>)}
            </select>
          </label>
          <label>
            Claim status
            <select value={claimStatusFilter} onChange={(event) => {
              setClaimStatusFilter(event.target.value);
              setClaimOffset(0);
            }}>
              {claimStatuses.map((status) => <option key={status}>{status}</option>)}
            </select>
          </label>
          <label>
            Provider
            <select value={providerFilter} onChange={(event) => {
              setProviderFilter(event.target.value);
              setClaimOffset(0);
            }}>
              {providerFilters.map((providerId) => (
                <option value={providerId} key={providerId}>{providerLabel(providerId, providers)}</option>
              ))}
            </select>
          </label>
          <label>
            XML status
            <select value={auditStatusFilter} onChange={(event) => {
              setAuditStatusFilter(event.target.value);
              setAuditOffset(0);
            }}>
              {auditStatuses.map((status) => <option key={status}>{status}</option>)}
            </select>
          </label>
          <label>
            XML type
            <select value={auditTypeFilter} onChange={(event) => {
              setAuditTypeFilter(event.target.value);
              setAuditOffset(0);
            }}>
              {transactionTypes.map((type) => <option key={type}>{type}</option>)}
            </select>
          </label>
        </div>
      </section>
      )}

      {activeTab === "timeline" && (
      <section className="panel timeline-panel">
        <div className="ledger-title">
          <div>
            <h2>Transaction Timeline</h2>
            <p className="muted">Grouped from the loaded ledger page by claim, adventurer, or acknowledgment relationship.</p>
          </div>
          <span className="muted">{timelineGroups.length} chains</span>
        </div>
        {timelineGroups.length === 0 ? (
          <p className="muted">No timeline chains match the current transaction filters.</p>
        ) : (
          <div className="timeline-grid">
            {timelineGroups.map((group) => (
              <TimelineGroupCard key={group.id} group={group} onSelect={openTransactionDetail} />
            ))}
          </div>
        )}
      </section>
      )}

      {activeTab === "ledger" && (
      <>
      <AcknowledgmentDrilldownPanel drilldowns={acknowledgmentDrilldowns} onDrilldown={applyAcknowledgmentDrilldown} />
      <section className="history-grid">
        <div className="panel ledger">
          <div className="ledger-title">
            <div>
              <h2>Persisted Transactions</h2>
              <p className="muted">from Postgres</p>
            </div>
            <button className="secondary" disabled={recentTransactions.length === 0} onClick={exportLedgerCSV}>Export CSV</button>
          </div>
          {recentTransactions.length === 0 ? (
            <p className="muted">No transactions match the current filters.</p>
          ) : (
            recentTransactions.map((transaction) => (
              <TransactionRow key={transaction.id} transaction={transaction} onSelect={openTransactionDetail} />
            ))
          )}
          <Pager page={transactionPage} onPrevious={() => setTransactionOffset(Math.max(0, transactionPage.offset - transactionPage.limit))} onNext={() => setTransactionOffset(transactionPage.offset + transactionPage.limit)} />
        </div>

        <div className="panel ledger">
          <div className="ledger-title">
            <h2>Recent Claims</h2>
            <span className="muted">from Postgres</span>
          </div>
          {recentClaims.length === 0 ? (
            <p className="muted">No claims match the current filters.</p>
          ) : (
            recentClaims.map((item) => <ClaimRow key={item.id} claim={item} onSelect={openClaimDetail} />)
          )}
          <Pager page={claimPage} onPrevious={() => setClaimOffset(Math.max(0, claimPage.offset - claimPage.limit))} onNext={() => setClaimOffset(claimPage.offset + claimPage.limit)} />
        </div>

        <div className="panel ledger">
          <div className="ledger-title">
            <div>
              <h2>Premium Ledger</h2>
              <p className="muted">820 reconciliation</p>
            </div>
            <span className="muted">{recentPremiumPayments.length} recent</span>
          </div>
          {recentPremiumPayments.length === 0 ? (
            <p className="muted">No premium payments have been recorded yet.</p>
          ) : (
            recentPremiumPayments.map((payment) => <PremiumPaymentRow key={payment.id} payment={payment} onSelect={openPremiumPaymentDetail} />)
          )}
        </div>

        <div className="panel ledger">
          <div className="ledger-title">
            <h2>Recent Adventurers</h2>
            <span className="muted">from Postgres</span>
          </div>
          {recentAdventurers.length === 0 ? (
            <p className="muted">No adventurers match the current search.</p>
          ) : (
            recentAdventurers.map((item) => <AdventurerRow key={item.id} adventurer={item} />)
          )}
          <Pager page={adventurerPage} onPrevious={() => setAdventurerOffset(Math.max(0, adventurerPage.offset - adventurerPage.limit))} onNext={() => setAdventurerOffset(adventurerPage.offset + adventurerPage.limit)} />
        </div>
      </section>
      </>
      )}

      {activeTab === "xml" && (
      <section className="history-grid intake-grid">
        <form className="panel raw-x12-panel" onSubmit={submitRawX12}>
          <div className="ledger-title">
            <div>
              <h2>Raw X12 Intake</h2>
              <p className="muted">Paste delimiter-based `834`, `820`, `270`, `276`, `278`, `837`, `835`, or `275` text and map it into canonical ASHN workflow.</p>
            </div>
            <div className="actions compact-actions">
              <button type="button" className="secondary" onClick={() => setRawX12Draft(sampleRaw834)}>Load Sample 834</button>
              <button type="button" className="secondary" onClick={() => setRawX12Draft(sampleRaw820)}>Load Sample 820</button>
              <button type="button" className="secondary" onClick={() => setRawX12Draft(sampleRaw270)}>Load Sample 270</button>
              <button type="button" className="secondary" onClick={() => setRawX12Draft(sampleRaw269)}>Load Sample 269</button>
              <button type="button" className="secondary" onClick={() => setRawX12Draft(sampleRaw276)}>Load Sample 276</button>
              <button type="button" className="secondary" onClick={() => setRawX12Draft(sampleRaw278)}>Load Sample 278</button>
              <button type="button" className="secondary" onClick={() => setRawX12Draft(sampleRawX12)}>Load Sample 837</button>
              <button type="button" className="secondary" onClick={() => setRawX12Draft(sampleRaw837D)}>Load Sample 837D</button>
              <button type="button" className="secondary" onClick={() => setRawX12Draft(sampleRaw835)}>Load Sample 835</button>
              <button type="button" className="secondary" onClick={() => setRawX12Draft(sampleRaw275)}>Load Sample 275</button>
              {raw275RejectionFixtures.map((fixture) => (
                <button type="button" className="secondary" key={fixture.id} onClick={() => setRawX12Draft(fixture.payload)}>{fixture.label}</button>
              ))}
            </div>
            <RawSampleGuide />
          </div>
          <label>
            Raw X12
            <textarea value={rawX12Draft} onChange={(event) => setRawX12Draft(event.target.value)} rows={12} spellCheck={false} />
          </label>
          <div className="actions compact-actions">
            <button type="submit" disabled={busy || !rawX12Draft.trim()}>Submit Raw X12</button>
          </div>
        </form>
        <form className="panel batch-drop-panel" onSubmit={submitBatchFiles}>
          <div className="ledger-title">
            <div>
              <h2>Batch File Drop</h2>
              <p className="muted">Upload XML, JSON, EDI, or X12 demo files and route each one through the same audited intake path.</p>
            </div>
            <span className="muted">multipart</span>
          </div>
          <label>
            Intake files
            <input name="files" type="file" multiple accept=".xml,.json,.x12,.edi,.txt,application/xml,application/json,text/plain" />
          </label>
          <div className="batch-drop-hints">
            <span>Accepted files create normal audit records.</span>
            <span>Rejected files still emit 999/audit visibility.</span>
          </div>
          <div className="actions compact-actions">
            <button type="submit" disabled={busy}>Submit Batch</button>
          </div>
        </form>
        <X12CapabilityMatrix />
        <div className="panel ledger">
          <div className="ledger-title">
            <h2>XML / Raw Intake Audits</h2>
            <span className="muted">from edi-intake</span>
          </div>
          <IntakeRejectionPanel
            summary={intakeRejectionSummary}
            metrics={intakeRejectionMetrics}
            busy={busy}
            onShowRejected={() => {
              setAuditStatusFilter("rejected");
              setAuditOffset(0);
            }}
            onShowRejected837={() => {
              setAuditStatusFilter("rejected");
              setAuditTypeFilter("837");
              setAuditOffset(0);
            }}
            onDrilldown={applyRejectionDrilldown}
            onSelect={openInboundMessageDetail}
            onReplay={replayInboundMessage}
          />
          {inboundMessages.length === 0 ? (
            <p className="muted">No intake messages match the current filters.</p>
          ) : (
            inboundMessages.map((message) => (
              <InboundMessageRow key={message.id} message={message} busy={busy} onSelect={openInboundMessageDetail} onReplay={replayInboundMessage} />
            ))
          )}
          <Pager page={auditPage} onPrevious={() => setAuditOffset(Math.max(0, auditPage.offset - auditPage.limit))} onNext={() => setAuditOffset(auditPage.offset + auditPage.limit)} />
        </div>
      </section>
      )}

      {(selectedClaim || selectedTransaction || selectedInboundMessage || selectedPremiumPayment) && (
        <div className="drawer-backdrop" onClick={closeDetail}>
        <aside className="detail-drawer" onClick={(event) => event.stopPropagation()} aria-label="Selected record details">
          <div className="ledger-title">
            <div>
              <p className="eyebrow">Selected Record</p>
              <h2>{selectedTransaction ? "Transaction Detail" : selectedInboundMessage ? "Intake Detail" : selectedPremiumPayment ? "Premium Reconciliation" : "Claim Detail"}</h2>
            </div>
            <button className="secondary" onClick={closeDetail}>Close</button>
          </div>
          {selectedTransaction && (
            <div className="detail-grid">
              <div className="detail-actions">
                <button className="secondary" onClick={() => downloadFromPath(`/v1/transactions/${selectedTransaction.id}/export?format=json`)}>Export JSON</button>
                <button className="secondary" onClick={() => downloadFromPath(`/v1/transactions/${selectedTransaction.id}/export?format=xml`)}>Export XML</button>
                <button className="secondary" disabled={!selectedTransaction.rawX12} onClick={() => downloadFromPath(`/v1/transactions/${selectedTransaction.id}/export?format=x12`)}>Export X12</button>
                <button disabled={busy} onClick={() => replayTransaction(selectedTransaction.id)}>Replay Transaction</button>
                {selectedTransaction.type === "275" && (
                  <>
                    <button className="secondary" disabled={busy || !hasDocumentReference(selectedTransaction)} onClick={() => inspectDocumentReference(selectedTransaction)}>Inspect Vault Receipt</button>
                    <button className="secondary" disabled={!payloadString(selectedTransaction, "content")} onClick={() => downloadFromPath(`/v1/transactions/${selectedTransaction.id}/document-reference/content`)}>Download Embedded Doc</button>
                    <button disabled={busy} onClick={() => reviewAttachment("Accepted")}>Accept Attachment</button>
                    <button className="danger" disabled={busy} onClick={() => reviewAttachment("Rejected")}>Reject Attachment</button>
                  </>
                )}
              </div>
              <DetailItem label="Type" value={selectedTransaction.type} />
              <DetailItem label="Status" value={selectedTransaction.status} />
              {selectedTransaction.type === "275" && (
                <>
                  <DetailItem label="Attachment Review" value={payloadString(selectedTransaction, "attachmentReviewStatus") ?? "Received"} />
                  <DetailItem label="Review Reason" value={payloadString(selectedTransaction, "attachmentReviewReason") ?? "—"} />
                  <DetailItem label="Packet" value={attachmentPacketLabel(selectedTransaction) ?? "—"} />
                  <DetailItem label="Document Ref" value={payloadString(selectedTransaction, "documentReferenceId") ?? "—"} />
                  <DetailLink label="Document URL" value={payloadString(selectedTransaction, "documentReferenceUrl") ?? ""} />
                </>
              )}
              <DetailItem label="Sender" value={selectedTransaction.senderId} />
              <DetailItem label="Receiver" value={selectedTransaction.receiverId} />
              <DetailItem label="Created" value={new Date(selectedTransaction.createdAt).toLocaleString()} />
              <DetailItem label="ID" value={selectedTransaction.id} />
              <DetailItem label="Related" value={selectedTransaction.relatedId ?? "—"} />
              {selectedRelationshipMap && (
                <TransactionRelationshipGraph relationshipMap={selectedRelationshipMap} onSelect={openTransactionDetail} />
              )}
              <div className="payload-tabs" role="tablist" aria-label="Payload formats">
                {payloadTabs.map((tab) => (
                  <button
                    key={tab.id}
                    className={`payload-tab ${payloadTab === tab.id ? "active" : ""}`}
                    type="button"
                    role="tab"
                    aria-selected={payloadTab === tab.id}
                    onClick={() => setPayloadTab(tab.id)}
                  >
                    {tab.label}
                  </button>
                ))}
              </div>
              {selectedPayloadView && (
                <PayloadBlock
                  title={`${selectedPayloadView.label} Payload`}
                  value={selectedPayloadView.value}
                  onCopy={copyText}
                  downloadLabel={`Download .${selectedPayloadView.extension}`}
                  onDownload={() => downloadText(selectedPayloadView.filename, selectedPayloadView.value)}
                  canDownload={selectedPayloadView.canDownload}
                />
              )}
            </div>
          )}
          {selectedClaim && (
            <div className="detail-grid">
              <div className="detail-actions">
                <button disabled={busy || selectedClaim.status === "Pending Documentation"} onClick={requestClaimDocumentation}>Request 275 Docs</button>
                <button disabled={busy} onClick={submitClaimDocumentationPacket}>Submit 275 Packet</button>
              </div>
              <DocumentationWorkbench
                claim={selectedClaim}
                checklist={documentationChecklist}
                attachmentTransactions={selectedClaimAttachmentTransactions}
                busy={busy}
                onReview={reviewAttachmentTransaction}
                onResubmit={requestDeficiencyAndResubmit}
              />
              <AdjudicationExplanationPanel claim={selectedClaim} explanation={selectedClaimAdjudication} />
              <BenefitPlanSignals claim={selectedClaim} explanation={selectedClaimAdjudication} />
              <DetailItem label="Status" value={selectedClaim.status} />
              <DetailItem label="Severity" value={selectedClaim.incidentSeverity} />
              <DetailItem label="Billed" value={money(selectedClaim.amountCents)} />
              <DetailItem label="Allowed" value={money(selectedClaim.allowedAmountCents)} />
              <DetailItem label="Paid" value={money(selectedClaim.paidAmountCents)} />
              <DetailItem label="Patient Resp." value={money(selectedClaim.patientResponsibilityCents)} />
              <DetailItem label="Adjustment" value={money(selectedClaim.adjustmentAmountCents)} />
              <DetailItem label="Adjustment Reason" value={selectedClaim.adjustmentReason ?? "—"} />
              <DetailItem label="Denial Reason" value={selectedClaim.denialReason ?? "—"} />
              <DetailItem label="Prior Auth" value={selectedClaim.authorizationTransactionId ?? "—"} />
              <DetailItem label="Auth Status" value={selectedClaim.authorizationStatus ?? "—"} />
              <DetailItem label="Auth Reason" value={selectedClaim.authorizationReason ?? "—"} />
              <DetailItem label="Adventurer" value={selectedClaim.adventurerId} />
              <DetailItem label="Provider" value={selectedClaim.providerId} />
              <DetailItem label="Transaction" value={selectedClaim.transactionId} />
              <DetailItem label="Claim ID" value={selectedClaim.id} />
              <DiagnosisBreakdown diagnoses={selectedClaim.diagnoses ?? []} />
              <ServiceLineBreakdown serviceLines={selectedClaim.serviceLines ?? []} />
            </div>
          )}
          {selectedPremiumPayment && (
            <div className="detail-grid">
              <div className="detail-actions">
                <button className="secondary" onClick={() => downloadFromPath(`/v1/premium-payments/${selectedPremiumPayment.id}/export?format=json`)}>Export JSON</button>
                <button className="secondary" onClick={() => downloadFromPath(`/v1/premium-payments/${selectedPremiumPayment.id}/export?format=xml`)}>Export XML</button>
                <button className="secondary" onClick={() => downloadText(`ashn-premium-payment-${selectedPremiumPayment.id}.csv`, premiumPaymentToCSV(selectedPremiumPayment))}>Export CSV</button>
                <button onClick={() => openTransactionDetail(selectedPremiumPayment.transactionId)}>Open 820 Transaction</button>
              </div>
              <DetailItem label="Amount" value={money(selectedPremiumPayment.amountCents)} />
              <DetailItem label="Status" value={selectedPremiumPayment.status} />
              <DetailItem label="Reconciled" value={selectedPremiumPayment.reconciled ? "Yes" : "No"} />
              <DetailItem label="Benefit Current" value={selectedPremiumPayment.currentForBenefits ? "Yes" : "No"} />
              <DetailItem label="Adventurer" value={selectedPremiumPayment.adventurerId} />
              <DetailItem label="Transaction" value={selectedPremiumPayment.transactionId} />
              <DetailItem label="Created" value={new Date(selectedPremiumPayment.createdAt).toLocaleString()} />
              <DetailItem label="Premium Payment ID" value={selectedPremiumPayment.id} />
            </div>
          )}
          {selectedInboundMessage && (
            <div className="detail-grid">
              <div className="detail-actions">
                <button className="secondary" onClick={() => downloadFromPath(`/v1/x12/messages/${selectedInboundMessage.id}/export?format=xml`)}>Export XML</button>
                <button className="secondary" onClick={() => downloadFromPath(`/v1/x12/messages/${selectedInboundMessage.id}/export?format=json`)}>Export JSON</button>
                <button disabled={busy} onClick={() => replayInboundMessage(selectedInboundMessage.id)}>Replay Intake</button>
              </div>
              <DetailItem label="Status" value={selectedInboundMessage.status} />
              <DetailItem label="Type" value={selectedInboundMessage.transactionType ?? "—"} />
              <DetailItem label="Downstream" value={selectedInboundMessage.downstreamStatus ? String(selectedInboundMessage.downstreamStatus) : "—"} />
              <DetailItem label="Content Type" value={selectedInboundMessage.contentType} />
              <DetailItem label="Created" value={new Date(selectedInboundMessage.createdAt).toLocaleString()} />
              <DetailItem label="ID" value={selectedInboundMessage.id} />
              {selectedInboundMessage.error && <DetailItem label="Error" value={selectedInboundMessage.error} />}
              <PayloadBlock
                title="Raw Intake Payload"
                value={selectedInboundMessage.rawPayload}
                onCopy={copyText}
              />
            </div>
          )}
        </aside>
        </div>
      )}
    </main>
  );
}

function MetricCard({ label, value, detail }: { label: string; value: number; detail: string }) {
  return (
    <div className="metric-card">
      <span>{label}</span>
      <strong>{value}</strong>
      <p>{detail}</p>
    </div>
  );
}

function ReadinessPanel({ readiness }: { readiness: ReadinessReport | null }) {
  const status = readiness?.status ?? "loading";
  const checkCount = readiness?.checks.length ?? 0;
  const okCount = readiness?.summary.ok ?? 0;
  const generatedAt = readiness?.generatedAt ? new Date(readiness.generatedAt).toLocaleTimeString() : "awaiting signal";
  return (
    <section className="readiness-panel" aria-label="System readiness">
      <div className="readiness-header">
        <div>
          <span className="mini-label">System Readiness</span>
          <strong className={status === "ready" ? "ready" : "degraded"}>{status}</strong>
        </div>
        <span className="readiness-time">{generatedAt}</span>
      </div>
      <div className="readiness-summary">
        <span>{okCount}/{checkCount} checks ok</span>
        <span>v{readiness?.version ?? "0.1.0"}</span>
        {readiness?.commit && <span>{readiness.commit.slice(0, 7)}</span>}
      </div>
      <div className="readiness-checks">
        {(readiness?.checks ?? []).slice(0, 5).map((check) => (
          <div key={check.name} className="readiness-check">
            <span className={check.status === "ok" ? "gateway-diamond ok" : "gateway-diamond bad"} />
            <strong>{check.name}</strong>
            <small>{check.count !== undefined ? `${check.count} loaded` : check.detail}</small>
          </div>
        ))}
        {!readiness && (
          <div className="readiness-check">
            <span className="gateway-diamond bad" />
            <strong>Readiness Signal</strong>
            <small>loading</small>
          </div>
        )}
      </div>
    </section>
  );
}

function MetricsCockpit({ metrics }: { metrics: MetricsSummary | null }) {
  const generatedAt = metrics?.generatedAt ? new Date(metrics.generatedAt).toLocaleTimeString() : "awaiting signal";
  return (
    <section className="metrics-cockpit" aria-label="Guild Operations Metrics">
      <div className="panel metrics-hero">
        <div>
          <p className="eyebrow">Guild Operations Board</p>
          <h2>Metrics Cockpit</h2>
          <p className="muted">Transaction volume, claim money flow, intake rejection pressure, and worker queue health from the live network.</p>
        </div>
        <div className={`metrics-status ${metrics?.operationalStatus === "healthy" ? "ready" : "attention"}`}>
          <span>{metrics?.operationalStatus ?? "loading"}</span>
          <strong>{generatedAt}</strong>
          <small>{metrics?.window ?? "latest records"}</small>
        </div>
      </div>

      <div className="metrics-grid">
        <MetricCard label="Loaded Transactions" value={metrics?.transactions.totalLoaded ?? 0} detail="latest ledger sample" />
        <MetricCard label="Loaded Claims" value={metrics?.claims.totalLoaded ?? 0} detail={`${money(metrics?.financials.billedCents)} billed`} />
        <MetricCard label="Intake Rejections" value={metrics?.intake.rejectionTotal ?? 0} detail="all-time rejection signal" />
        <MetricCard label="Dead Letters" value={metrics?.asyncJobs.deadLetters ?? 0} detail={`${metrics?.asyncJobs.totalLoaded ?? 0} worker jobs loaded`} />
      </div>

      <div className="metrics-board">
        <MetricBars title="Transactions by Type" items={metrics?.transactions.byType} />
        <MetricBars title="Transactions by Status" items={metrics?.transactions.byStatus} />
        <MetricBars title="Claims by Status" items={metrics?.claims.byStatus} />
        <MetricBars title="Claims by Provider" items={metrics?.claims.byProvider} />
        <MetricBars title="Async Jobs by Status" items={metrics?.asyncJobs.byStatus} />
        <MetricBars title="Top Intake Reasons" items={metrics?.intake.byReason} />
      </div>

      <section className="panel financial-panel">
        <div className="ledger-title">
          <div>
            <h2>Claim Money Flow</h2>
            <p className="muted">Summed from the latest claim sample.</p>
          </div>
        </div>
        <div className="financial-grid">
          <DetailItem label="Billed" value={money(metrics?.financials.billedCents)} />
          <DetailItem label="Allowed" value={money(metrics?.financials.allowedCents)} />
          <DetailItem label="Paid" value={money(metrics?.financials.paidCents)} />
          <DetailItem label="Patient Resp." value={money(metrics?.financials.patientResponsibilityCents)} />
          <DetailItem label="Adjustments" value={money(metrics?.financials.adjustmentCents)} />
        </div>
      </section>

      <section className="panel metrics-highlights">
        <h2>Signal Highlights</h2>
        {(metrics?.highlights ?? ["Awaiting live metric signal."]).map((highlight) => (
          <p key={highlight}>{highlight}</p>
        ))}
      </section>
    </section>
  );
}

function MetricBars({ title, items }: { title: string; items?: Record<string, number> }) {
  const entries = Object.entries(items ?? {}).sort((left, right) => right[1] - left[1]).slice(0, 6);
  const max = Math.max(1, ...entries.map(([, value]) => value));
  return (
    <section className="panel metric-bars">
      <div className="ledger-title">
        <h2>{title}</h2>
        <span className="muted">{entries.reduce((sum, [, value]) => sum + value, 0)} events</span>
      </div>
      {entries.length === 0 ? (
        <p className="muted">No metric signal loaded yet.</p>
      ) : (
        entries.map(([label, value]) => (
          <div key={label} className="metric-bar-row">
            <div>
              <strong>{label}</strong>
              <span>{value}</span>
            </div>
            <div className="metric-bar-track">
              <span style={{ width: `${Math.max(8, (value / max) * 100)}%` }} />
            </div>
          </div>
        ))
      )}
    </section>
  );
}

function AcknowledgmentDrilldownPanel({
  drilldowns,
  onDrilldown
}: {
  drilldowns: AcknowledgmentDrilldown[];
  onDrilldown: (item: AcknowledgmentDrilldown) => void;
}) {
  const total = drilldowns.reduce((sum, item) => sum + item.count, 0);
  return (
    <section className="panel acknowledgment-drilldown" aria-label="acknowledgment outcome drilldown">
      <div className="ledger-title">
        <div>
          <h2>Acknowledgment Outcomes</h2>
          <p className="muted">Separates interchange, syntax, application-advice, and business-review evidence from the loaded ledger page.</p>
        </div>
        <span className="muted">{total} signals</span>
      </div>
      <div className="acknowledgment-grid">
        {drilldowns.map((item) => (
          <article key={item.id} className={`acknowledgment-card ${item.id}`}>
            <span>{item.label}</span>
            <strong>{item.count}</strong>
            <p>{item.detail}</p>
            {item.examples.length > 0 && (
              <small>{item.examples.slice(0, 2).map((transaction) => transaction.id).join(", ")}</small>
            )}
            <button type="button" className="secondary" disabled={item.count === 0} onClick={() => onDrilldown(item)}>
              Drill into {item.typeFilter}
            </button>
          </article>
        ))}
      </div>
    </section>
  );
}

function DemoScenarioCard({
  scenario,
  runState,
  busy,
  onExport,
  onExportEvidence,
  onCopy,
  onRun,
  onStartPlayback,
  onRunNextStep,
  onFinishPlayback
}: {
  scenario: DemoScenario;
  runState?: ScenarioRunState;
  busy: boolean;
  onExport: (scenario: DemoScenario) => void;
  onExportEvidence: (scenario: DemoScenario) => void;
  onCopy: (scenario: DemoScenario) => void;
  onRun: (scenario: DemoScenario) => void;
  onStartPlayback: (scenario: DemoScenario) => void;
  onRunNextStep: (scenario: DemoScenario) => void;
  onFinishPlayback: (scenario: DemoScenario) => void;
}) {
  const completedSteps = runState?.completedSteps ?? 0;
  const isComplete = completedSteps === scenario.steps.length && !runState?.running;
  const isPlayback = runState?.mode === "playback" && !isComplete;
  const nextStep = scenario.steps[completedSteps];
  const progressLabel = runState?.running
    ? `Running: ${runState.currentStep ?? "Starting"}`
    : isPlayback
      ? `Playback ready: ${nextStep?.label ?? "Complete"}`
    : isComplete
      ? "Complete"
      : "Ready";

  return (
    <article className="scenario-card">
      <div className="scenario-card-header">
        <div>
          <span className="eyebrow">{scenario.duration} · {scenario.audience}</span>
          <h3>{scenario.title}</h3>
        </div>
        <span>{completedSteps}/{scenario.steps.length} steps</span>
      </div>
      <div className="scenario-progress" aria-label={`${scenario.title} runner status`}>
        <span>{progressLabel}</span>
        <progress max={scenario.steps.length} value={completedSteps} />
      </div>
      {runState?.error && <p className="scenario-error">{runState.error}</p>}
      <p>{scenario.outcome}</p>
      <p className="muted">{scenario.story}</p>
      <div className="chips">
        {scenario.highlights.map((highlight) => <span key={highlight}>{highlight}</span>)}
      </div>
      <ol className="scenario-steps">
        {scenario.steps.map((step) => (
          <li key={step.label}>
            <strong>{step.label}</strong>
            <span>{step.action}</span>
            <small>{step.expected}</small>
          </li>
        ))}
      </ol>
      <div className="scenario-exports">
        <span>Exports: {scenario.exports.join(", ")}</span>
      </div>
      <div className="actions compact-actions">
        <button type="button" disabled={busy || runState?.running} onClick={() => onRun(scenario)}>Run Scenario</button>
        <button type="button" className="secondary" disabled={busy || runState?.running} onClick={() => onStartPlayback(scenario)}>Start Playback</button>
        <button type="button" disabled={!isPlayback || busy || runState?.running} onClick={() => onRunNextStep(scenario)}>Run Next Step</button>
        <button type="button" className="secondary" disabled={!isPlayback || busy || runState?.running} onClick={() => onFinishPlayback(scenario)}>Finish Playback</button>
        <button type="button" onClick={() => onExport(scenario)}>Export Scenario JSON</button>
        <button type="button" className="secondary" disabled={!isComplete} onClick={() => onExportEvidence(scenario)}>Export Evidence Bundle</button>
        <button type="button" className="secondary" onClick={() => onCopy(scenario)}>Copy Operator Steps</button>
      </div>
    </article>
  );
}

function RecentScenarioRuns({
  runs,
  busy,
  onExport,
  onCopyTransactions,
  onRerun
}: {
  runs: ScenarioRunRecord[];
  busy: boolean;
  onExport: (run: ScenarioRunRecord) => void;
  onCopyTransactions: (run: ScenarioRunRecord) => void;
  onRerun: (scenarioId: string) => void;
}) {
  return (
    <section className="recent-scenario-runs" aria-label="Recent scenario runs">
      <div className="relationship-heading">
        <div>
          <h3>Recent Scenario Runs</h3>
          <p className="muted">Completed runs stay in this browser for quick evidence export or reruns.</p>
        </div>
        <span>{runs.length} saved</span>
      </div>
      {runs.length === 0 ? (
        <p className="muted">No scenario runs yet. Run a scenario to capture evidence here.</p>
      ) : (
        <div className="scenario-run-list">
          {runs.map((run) => (
            <article key={run.id} className="scenario-run-card">
              <div>
                <strong>{run.scenarioTitle}</strong>
                <span>{new Date(run.completedAt).toLocaleString()} · {run.completedSteps}/{run.totalSteps} steps · {run.status}</span>
                <code>{run.id}</code>
              </div>
              <div className="chips">
                <span>{run.transactionIds.length} tx</span>
                <span>{run.claimIds.length} claims</span>
                <span>{run.adventurerIds.length} adventurers</span>
              </div>
              {run.transactionIds.length > 0 && (
                <p className="muted">Transactions: {run.transactionIds.slice(0, 4).join(", ")}{run.transactionIds.length > 4 ? "…" : ""}</p>
              )}
              <div className="actions compact-actions">
                <button type="button" className="secondary" onClick={() => onExport(run)}>Export Evidence</button>
                <button type="button" className="secondary" disabled={run.transactionIds.length === 0} onClick={() => onCopyTransactions(run)}>Copy Transaction IDs</button>
                <button type="button" disabled={busy} onClick={() => onRerun(run.scenarioId)}>Re-run</button>
              </div>
            </article>
          ))}
        </div>
      )}
    </section>
  );
}

function TradingPartnerCard({
  partner,
  busy,
  onEdit,
  onDelete
}: {
  partner: TradingPartner;
  busy: boolean;
  onEdit: (partner: TradingPartner) => void;
  onDelete: (partnerId: string) => void;
}) {
  const profile = partner.validationProfile;
  return (
    <article className="partner-card">
      <div>
        <strong>{partner.name}</strong>
        <span>{partner.status} · routes to {partner.routeTarget}</span>
      </div>
      <code>{partner.senderId} → {partner.receiverId}</code>
      <div className="chips">
        {partner.allowedTransactionTypes.map((type) => <span key={type}>{type}</span>)}
      </div>
      {profile && (
        <div className="partner-guide-grid" aria-label={`${partner.name} companion guide`}>
          <GuideRule title="275 Attachments" value={attachmentGuide(profile)} />
          <GuideRule title="278 Auth" value={authorizationGuide(profile)} />
          <GuideRule title="837 Claims" value={claimGuide(profile)} />
          <GuideRule title="Dental Rules" value={dentalGuide(profile)} />
        </div>
      )}
      <div className="actions compact-actions">
        <button className="secondary" disabled={busy} onClick={() => onEdit(partner)}>Edit</button>
        <button className="danger" disabled={busy} onClick={() => onDelete(partner.id)}>Delete</button>
      </div>
    </article>
  );
}

function X12CapabilityMatrix() {
  return (
    <section className="panel x12-capability-matrix" aria-label="X12 capability matrix">
      <div className="ledger-title">
        <div>
          <h2>X12 Capability Matrix</h2>
          <p className="muted">Quick map of what ASHN can parse, generate, acknowledge, and teach today.</p>
        </div>
        <span className="muted">{x12CapabilityRows.length} lanes</span>
      </div>
      <div className="capability-table" role="table" aria-label="Supported X12 transaction capabilities">
        <div className="capability-row capability-header" role="row">
          <span role="columnheader">Type</span>
          <span role="columnheader">Flow</span>
          <span role="columnheader">Intake</span>
          <span role="columnheader">Output</span>
          <span role="columnheader">Ack</span>
          <span role="columnheader">Learning Focus</span>
        </div>
        {x12CapabilityRows.map((row) => (
          <div className="capability-row" role="row" key={row.type}>
            <strong role="cell">{row.type}</strong>
            <span role="cell">{row.direction}</span>
            <span role="cell">{row.parser}</span>
            <span role="cell">{row.generated}</span>
            <span role="cell">{row.acknowledgments}</span>
            <p role="cell">{row.learning}</p>
          </div>
        ))}
      </div>
    </section>
  );
}

function RawSampleGuide() {
  return (
    <section className="raw-sample-guide" aria-label="Raw X12 sample guide">
      <div>
        <strong>Accepted Sample Guide</strong>
        <span>Use these scrolls when you want a predictable green-path demo.</span>
      </div>
      <div className="raw-sample-guide-grid">
        {rawSampleGuides.map((sample) => (
          <article className="raw-sample-guide-card" key={sample.type}>
            <span>{sample.type}</span>
            <strong>{sample.result}</strong>
            <p>{sample.focus}</p>
          </article>
        ))}
      </div>
    </section>
  );
}

function GuideRule({ title, value }: { title: string; value: string }) {
  return (
    <div className="guide-rule">
      <span>{title}</span>
      <strong>{value}</strong>
    </div>
  );
}

function attachmentGuide(profile: NonNullable<TradingPartner["validationProfile"]>) {
  const pieces = [
    profile.attachmentTypes?.length ? `${profile.attachmentTypes.join("/")} attachments` : "standard attachments",
    profile.reportTypeCodes?.length ? `${profile.reportTypeCodes.join("/")} reports` : "",
    profile.contentTypes?.length ? profile.contentTypes.join(", ") : "",
    profile.allowedFileExtensions?.length ? `${profile.allowedFileExtensions.join("/")} files` : "",
    profile.maxAttachmentsPerPacket ? `${profile.maxAttachmentsPerPacket} docs/packet` : "",
    profile.unsolicitedAttachmentWindowDays === 0 ? "same-day unsolicited" : profile.unsolicitedAttachmentWindowDays ? `${profile.unsolicitedAttachmentWindowDays}d unsolicited window` : "",
    profile.maxEmbeddedContentBytes ? `${Math.round(profile.maxEmbeddedContentBytes / 1024)} KB embedded limit` : ""
  ].filter(Boolean);
  return pieces.join(" · ");
}

function authorizationGuide(profile: NonNullable<TradingPartner["validationProfile"]>) {
  const pieces = [
    profile.serviceTypes?.length ? `${profile.serviceTypes.join("/")} services` : "standard services",
    profile.incidentSeverities?.length ? `${profile.incidentSeverities.join("/")} severity` : "standard severity"
  ].filter(Boolean);
  return pieces.join(" · ");
}

function claimGuide(profile: NonNullable<TradingPartner["validationProfile"]>) {
  const pieces = [
    profile.diagnosisCodes?.length ? `${profile.diagnosisCodes.join("/")} diagnoses` : "standard diagnoses",
    profile.procedureCodePrefixes?.length ? `${profile.procedureCodePrefixes.join("/")} procedure prefixes` : "",
    profile.procedureCodes?.length ? `${profile.procedureCodes.join("/")} procedures` : ""
  ].filter(Boolean);
  return pieces.join(" · ");
}

function dentalGuide(profile: NonNullable<TradingPartner["validationProfile"]>) {
  const pieces = [
    profile.dentalCdtRanges?.length ? `${profile.dentalCdtRanges.join("/")} CDT` : "",
    profile.dentalRequiresToothNumber ? "tooth required" : "",
    profile.dentalAllowedSurfaces?.length ? `${profile.dentalAllowedSurfaces.join("/")} surfaces` : "",
    profile.dentalAllowedQuadrants?.length ? `${profile.dentalAllowedQuadrants.join("/")} quadrants` : "",
    profile.dentalRequiredAttachmentCodes?.length ? `${profile.dentalRequiredAttachmentCodes.join("/")} docs` : "",
    profile.dentalPredeterminationRules?.length ? profile.dentalPredeterminationRules.join(", ") : ""
  ].filter(Boolean);
  return pieces.length > 0 ? pieces.join(" · ") : "standard dental rules";
}

function Pager({ page, onPrevious, onNext }: { page: PageInfo; onPrevious: () => void; onNext: () => void }) {
  return (
    <div className="pager">
      <span>{pageSummary(page)}</span>
      <div>
        <button className="secondary" disabled={page.offset === 0} onClick={onPrevious}>Previous</button>
        <button className="secondary" disabled={!page.hasMore} onClick={onNext}>Next</button>
      </div>
    </div>
  );
}

function PayloadBlock({
  title,
  value,
  onCopy,
  onDownload,
  downloadLabel = "Download",
  canDownload = false
}: {
  title: string;
  value: string;
  onCopy: (value: string) => void;
  onDownload?: () => void;
  downloadLabel?: string;
  canDownload?: boolean;
}) {
  return (
    <div className="payload-block">
      <div className="payload-title">
        <h3>{title}</h3>
        <div>
          <button className="secondary" onClick={() => onCopy(value)}>Copy</button>
          {onDownload && (
            <button className="secondary" disabled={!canDownload} onClick={onDownload}>{downloadLabel}</button>
          )}
        </div>
      </div>
      <pre>{value}</pre>
    </div>
  );
}

function LedgerEvent({ event }: { event: Envelope }) {
  const transactions = event.transactions ?? (event.transaction ? [event.transaction] : []);
  return (
    <article className="event">
      <p>{event.lore ?? event.error ?? "Transaction event"}</p>
      <div className="chips">
        {transactions.map((transaction) => (
          <span key={transaction.id}>
            {transaction.type} · {transaction.status}
          </span>
        ))}
      </div>
      <details>
        <summary>Raw payload</summary>
        <pre>{JSON.stringify(event, null, 2)}</pre>
      </details>
    </article>
  );
}

function DetailItem({ label, value }: { label: string; value: string }) {
  return (
    <div className="detail-item">
      <span>{label}</span>
      <strong>{value || "—"}</strong>
    </div>
  );
}

function DetailLink({ label, value }: { label: string; value: string }) {
  return (
    <div className="detail-item">
      <span>{label}</span>
      {value ? <a href={value} target="_blank" rel="noreferrer">{value}</a> : <strong>—</strong>}
    </div>
  );
}

function ServiceLineBreakdown({ serviceLines }: { serviceLines: ClaimServiceLine[] }) {
  if (serviceLines.length === 0) {
    return null;
  }
  return (
    <section className="service-line-breakdown" aria-label="service-line adjudication">
      <div className="relationship-heading">
        <div>
          <h3>Service-Line Adjudication</h3>
          <p className="muted">Line-level billed, allowed, paid, patient responsibility, and adjustment details.</p>
        </div>
        <span>{serviceLines.length} lines</span>
      </div>
      <div className="service-line-grid">
        {serviceLines.map((line, index) => (
          <article key={`${line.lineNumber}-${line.procedureCode}-${index}`} className="service-line-card">
            <div>
              <span>Line {line.lineNumber || index + 1}</span>
              <strong>{line.procedureCode || "ASHN"}</strong>
              <small>{line.description || "ASHN service line"} · {line.units || 1} unit(s)</small>
            </div>
            {(line.cdtCode || line.toothNumber || line.surface || line.quadrant || line.orthodontic) && (
              <div className="chips">
                {line.cdtCode && <span>CDT {line.cdtCode}</span>}
                {line.toothNumber && <span>Tooth {line.toothNumber}</span>}
                {line.surface && <span>Surface {line.surface}</span>}
                {line.quadrant && <span>{line.quadrant}</span>}
                {line.orthodontic && <span>Orthodontic</span>}
              </div>
            )}
            <dl>
              <div><dt>Billed</dt><dd>{money(line.amountCents)}</dd></div>
              <div><dt>Allowed</dt><dd>{money(line.allowedAmountCents)}</dd></div>
              <div><dt>Paid</dt><dd>{money(line.paidAmountCents)}</dd></div>
              <div><dt>Patient</dt><dd>{money(line.patientResponsibilityCents)}</dd></div>
              <div><dt>Adjustment</dt><dd>{money(line.adjustmentAmountCents)}</dd></div>
            </dl>
            {(line.adjustmentReason || line.denialReason) && (
              <p className="muted">{line.denialReason ?? line.adjustmentReason}</p>
            )}
          </article>
        ))}
      </div>
    </section>
  );
}

function BenefitPlanSignals({ claim, explanation }: { claim: Claim; explanation: AdjudicationExplanation | null }) {
  const serviceLines = claim.serviceLines ?? [];
  const signals = [
    `Coverage: ${explanation?.coverageStatus ?? "claim-level"}`,
    `Provider tier: ${explanation?.providerTier ?? "standard"}`,
    `Adventurer rank: ${explanation?.adventurerRank ?? "standard"}`,
    `Premium signal: ${explanation?.premiumCurrent === undefined ? "unknown" : (explanation.premiumCurrent ? "current" : "not current")}`
  ];
  if (claim.authorizationStatus) {
    signals.push(`Prior auth: ${claim.authorizationStatus}`);
  }
  if (serviceLines.some(claimServiceLineIsDentalUI)) {
    signals.push(`Dental max signal: ${dentalMaximumSignal(serviceLines)}`);
  }
  if (serviceLines.length === 0) {
    return null;
  }
  return (
    <section className="adjudication-explanation benefit-plan-signals" aria-label="benefit plan signals">
      <div className="relationship-heading">
        <div>
          <h3>Benefit Plan Signals</h3>
          <p className="muted">Readable plan rules inferred from service-line category, coverage, premium, and authorization context.</p>
        </div>
        <span>{serviceLines.length} lines</span>
      </div>
      <div className="chips adjudication-signals">
        {signals.map((signal) => <span key={signal}>{signal}</span>)}
      </div>
      <div className="service-line-grid">
        {serviceLines.map((line, index) => (
          <article key={`${line.lineNumber}-${line.procedureCode}-${index}-benefit`} className="service-line-card benefit-signal-card">
            <div>
              <span>Plan category</span>
              <strong>{benefitCategoryLabel(line)}</strong>
              <small>{line.procedureCode || line.cdtCode || "ASHN"} · {line.adjustmentReason || "standard benefit rule"}</small>
            </div>
            <dl>
              <div><dt>Allowed Rate</dt><dd>{percentageLabel(line.allowedAmountCents, line.amountCents)}</dd></div>
              <div><dt>Paid Rate</dt><dd>{percentageLabel(line.paidAmountCents, line.allowedAmountCents)}</dd></div>
              <div><dt>Member Share</dt><dd>{percentageLabel(line.patientResponsibilityCents, line.allowedAmountCents)}</dd></div>
              <div><dt>Plan Savings</dt><dd>{money(line.adjustmentAmountCents)}</dd></div>
            </dl>
          </article>
        ))}
      </div>
    </section>
  );
}

function DiagnosisBreakdown({ diagnoses }: { diagnoses: ClaimDiagnosis[] }) {
  if (diagnoses.length === 0) {
    return null;
  }
  return (
    <section className="service-line-breakdown" aria-label="claim diagnoses">
      <div className="relationship-heading">
        <div>
          <h3>Claim Diagnoses</h3>
          <p className="muted">Primary and supporting diagnosis codes carried from XML, JSON, or raw X12 HI segments.</p>
        </div>
        <span>{diagnoses.length} codes</span>
      </div>
      <div className="service-line-grid">
        {diagnoses.map((diagnosis, index) => (
          <article key={`${diagnosis.qualifier}-${diagnosis.code}-${index}`} className="service-line-card">
            <div>
              <span>{diagnosis.primary ? "Primary" : "Supporting"} · {diagnosis.qualifier || "ABF"}</span>
              <strong>{diagnosis.code}</strong>
              <small>{diagnosis.description || "Diagnosis carried on claim"}</small>
            </div>
          </article>
        ))}
      </div>
    </section>
  );
}

function benefitCategoryLabel(line: ClaimServiceLine) {
  if (claimServiceLineIsDentalUI(line)) {
    const code = Number((line.cdtCode || line.procedureCode || "").replace(/\D/g, ""));
    if (line.orthodontic || (code >= 8000 && code <= 8999)) return "Dental orthodontic";
    if (code >= 1000 && code <= 1999) return "Dental preventive";
    if (code >= 2000 && code <= 4999) return "Dental basic";
    return "Dental major";
  }
  const code = (line.procedureCode || "").toUpperCase();
  if (code.startsWith("ASHN3")) return "Resurrection benefit";
  if (code.startsWith("ASHN2")) return "Supplies benefit";
  return "Clinical benefit";
}

function claimServiceLineIsDentalUI(line: ClaimServiceLine) {
  return Boolean(line.cdtCode || line.toothNumber || line.surface || line.quadrant || line.orthodontic || (line.procedureCode || "").toUpperCase().startsWith("D"));
}

function percentageLabel(numerator?: number, denominator?: number) {
  if (numerator === undefined || denominator === undefined || denominator <= 0) {
    return "—";
  }
  return `${Math.round((numerator / denominator) * 100)}%`;
}

function dentalMaximumSignal(serviceLines: ClaimServiceLine[]) {
  if (serviceLines.some((line) => (line.adjustmentReason || "").toLowerCase().includes("annual maximum"))) {
    return "applied";
  }
  return "available";
}

function AdjudicationExplanationPanel({ claim, explanation }: { claim: Claim; explanation: AdjudicationExplanation | null }) {
  if (!explanation && !claim.adjustmentReason && !claim.denialReason) {
    return null;
  }

  const signals = [
    { label: "Engine", value: explanation?.engine ?? "payer-core" },
    { label: "Coverage", value: explanation?.coverageStatus ?? "—" },
    { label: "Provider Tier", value: explanation?.providerTier ?? "—" },
    { label: "Adventurer Rank", value: explanation?.adventurerRank ?? "—" },
    {
      label: "Premium Current",
      value: explanation?.premiumCurrent === undefined ? "—" : (explanation.premiumCurrent ? "Yes" : "No")
    },
    {
      label: "Premium Paid",
      value: explanation?.premiumPaidAmountCents === undefined ? "—" : money(explanation.premiumPaidAmountCents)
    }
  ];

  const reason = explanation?.denialReason || explanation?.adjustmentReason || claim.denialReason || claim.adjustmentReason || "No adjudication reason recorded yet.";

  return (
    <section className="adjudication-explanation" aria-label="adjudication explanation">
      <div className="relationship-heading">
        <div>
          <h3>Adjudication Explanation</h3>
          <p className="muted">Readable benefit signals from the latest related 277 status response.</p>
        </div>
        <span>Latest 277</span>
      </div>
      <div className="adjudication-money-grid">
        <DetailItem label="Allowed" value={money(explanation?.allowedAmountCents ?? claim.allowedAmountCents)} />
        <DetailItem label="Paid" value={money(explanation?.paidAmountCents ?? claim.paidAmountCents)} />
        <DetailItem label="Patient Resp." value={money(explanation?.patientResponsibilityCents ?? claim.patientResponsibilityCents)} />
        <DetailItem label="Adjustment" value={money(explanation?.adjustmentAmountCents ?? claim.adjustmentAmountCents)} />
      </div>
      <div className="chips adjudication-signals">
        {signals.map((signal) => (
          <span key={signal.label}>{signal.label}: {signal.value}</span>
        ))}
      </div>
      <p className="muted">{reason}</p>
    </section>
  );
}

function DocumentationWorkbench({
  claim,
  checklist,
  attachmentTransactions,
  busy,
  onReview,
  onResubmit
}: {
  claim: Claim;
  checklist: DocumentationChecklistItem[];
  attachmentTransactions: Transaction[];
  busy: boolean;
  onReview: (transactionId: string, status: "Accepted" | "Rejected") => void;
  onResubmit: (transaction: Transaction) => void;
}) {
  const openCount = claim.status === "Pending Documentation" ? checklist.filter((item) => item.required).length : 0;
  const receivedCount = attachmentTransactions.length;
  const statusLabel = receivedCount > 0 ? `${receivedCount} docs received` : claim.status === "Pending Documentation" ? `${openCount} required docs open` : "Ready for packet";

  return (
    <section className="documentation-workbench" aria-label="275 documentation workbench">
      <div className="relationship-heading">
        <div>
          <h3>275 Documentation Workbench</h3>
          <p className="muted">Request, package, and review supporting patient information for this claim.</p>
        </div>
        <span>{statusLabel}</span>
      </div>
      <div className="documentation-checklist">
        {checklist.map((item) => (
          <article key={item.code} className="documentation-item">
            <span>{item.required ? "Required" : "Optional"}</span>
            <strong>{item.label}</strong>
            <small>{item.attachmentType}/{item.reportTypeCode} · {item.contentType}</small>
          </article>
        ))}
      </div>
      <p className="muted">
        A submitted packet creates one related 275 transaction per checklist document, sharing the same packet id for downstream review.
      </p>
      {attachmentTransactions.length > 0 && (
        <div className="document-review-list">
          <h4>Document Review</h4>
          {attachmentTransactions.map((transaction) => {
            const reviewStatus = payloadString(transaction, "attachmentReviewStatus") ?? "Received";
            return (
              <article key={transaction.id} className="document-review-row">
                <div>
                  <span>{reviewStatus}</span>
                  <strong>{payloadString(transaction, "description") ?? payloadString(transaction, "attachmentControlNumber") ?? transaction.id}</strong>
                  <small>{payloadString(transaction, "attachmentType")}/{payloadString(transaction, "reportTypeCode")} · {payloadString(transaction, "documentReferenceId") ?? transaction.id}</small>
                </div>
                <div className="review-actions">
                  <button disabled={busy || reviewStatus === "Accepted"} onClick={() => onReview(transaction.id, "Accepted")}>Accept Doc</button>
                  <button className="danger" disabled={busy || reviewStatus === "Rejected"} onClick={() => onReview(transaction.id, "Rejected")}>Reject Doc</button>
                  {reviewStatus === "Rejected" && (
                    <button disabled={busy} onClick={() => onResubmit(transaction)}>Request + Resubmit</button>
                  )}
                </div>
              </article>
            );
          })}
        </div>
      )}
    </section>
  );
}

function AuthorizationDocumentationWorkbench({
  authorizationTransaction,
  checklist,
  attachmentTransactions,
  busy,
  onReview
}: {
  authorizationTransaction: Transaction;
  checklist: DocumentationChecklistItem[];
  attachmentTransactions: Transaction[];
  busy: boolean;
  onReview: (transactionId: string, status: "Accepted" | "Rejected") => void;
}) {
  const receivedCount = attachmentTransactions.length;
  const statusLabel = receivedCount > 0 ? `${receivedCount} auth docs received` : `${checklist.length} requested doc types`;

  return (
    <section className="documentation-workbench" aria-label="278 authorization documentation workbench">
      <div className="relationship-heading">
        <div>
          <h3>278 Authorization Documentation</h3>
          <p className="muted">Collect and review 275 support before approving or denying this authorization.</p>
        </div>
        <span>{statusLabel}</span>
      </div>
      <div className="documentation-checklist">
        {checklist.map((item) => (
          <article key={item.code} className="documentation-item">
            <span>{item.required ? "Expected" : "Optional"}</span>
            <strong>{item.label}</strong>
            <small>{item.attachmentType}/{item.reportTypeCode} · {item.contentType}</small>
          </article>
        ))}
      </div>
      {authorizationReviewPrompts(authorizationTransaction).length > 0 && (
        <div className="review-prompt-list">
          <h4>Manual Review Prompts</h4>
          {authorizationReviewPrompts(authorizationTransaction).map((prompt) => (
            <p key={prompt}>{prompt}</p>
          ))}
        </div>
      )}
      {attachmentTransactions.length === 0 ? (
        <p className="muted">No 275 support is linked to {authorizationTransaction.id} yet.</p>
      ) : (
        <div className="document-review-list">
          <h4>Authorization Document Review</h4>
          {attachmentTransactions.map((transaction) => {
            const reviewStatus = payloadString(transaction, "attachmentReviewStatus") ?? "Received";
            return (
              <article key={transaction.id} className="document-review-row">
                <div>
                  <span>{reviewStatus}</span>
                  <strong>{payloadString(transaction, "description") ?? payloadString(transaction, "attachmentControlNumber") ?? transaction.id}</strong>
                  <small>{payloadString(transaction, "attachmentType")}/{payloadString(transaction, "reportTypeCode")} · {payloadString(transaction, "documentReferenceId") ?? transaction.id}</small>
                </div>
                <div className="review-actions">
                  <button disabled={busy || reviewStatus === "Accepted"} onClick={() => onReview(transaction.id, "Accepted")}>Accept Doc</button>
                  <button className="danger" disabled={busy || reviewStatus === "Rejected"} onClick={() => onReview(transaction.id, "Rejected")}>Reject Doc</button>
                </div>
              </article>
            );
          })}
        </div>
      )}
    </section>
  );
}

function money(value?: number) {
  return `$${((value ?? 0) / 100).toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`;
}

function buildIntakeRejectionSummary(messages: InboundMessage[]): IntakeRejectionSummary {
  const rejected = messages.filter((message) => message.status === "rejected");
  return {
    messages: rejected,
    byPartner: topCounts(rejected.map((message) => message.partnerId || "Unknown partner")),
    byType: topCounts(rejected.map((message) => message.transactionType || "Unknown type")),
    byReason: topCounts(rejected.map((message) => rejectionReasonLabel(message.error)))
  };
}

function topCounts(values: string[], limit = 3) {
  const counts = new Map<string, number>();
  for (const value of values) {
    counts.set(value, (counts.get(value) ?? 0) + 1);
  }
  return Array.from(counts.entries())
    .map(([label, count]) => ({ label, count }))
    .sort((left, right) => right.count - left.count || left.label.localeCompare(right.label))
    .slice(0, limit);
}

function rejectionReasonLabel(error?: string) {
  const text = (error || "Unknown rejection").trim();
  if (text.includes("diagnosis code")) return "Diagnosis code profile";
  if (text.includes("diagnosis qualifier")) return "Diagnosis qualifier profile";
  if (text.includes("procedure code")) return "Procedure profile";
  if (text.includes("attachment type")) return "Attachment type profile";
  if (text.includes("attachment purpose")) return "Attachment purpose profile";
  if (text.includes("attachment format")) return "Attachment format profile";
  if (text.includes("report type")) return "Report type profile";
  if (text.includes("base64") || text.includes("mime")) return "Attachment payload encoding";
  if (text.includes("lx loops") || text.includes("packet contains")) return "Attachment packet limit";
  if (text.includes("solicited attachment") || text.includes("trace")) return "Solicited trace matching";
  if (text.includes("claim not found")) return "Missing related claim";
  if (text.includes("same day") || (text.includes("within") && text.includes("unsolicited"))) return "Late unsolicited attachment";
  if (text.includes("trading partner")) return "Trading partner routing";
  if (text.includes("transaction type")) return "Transaction set profile";
  return text;
}

function IntakeRejectionPanel({
  summary,
  metrics,
  busy,
  onShowRejected,
  onShowRejected837,
  onDrilldown,
  onSelect,
  onReplay
}: {
  summary: IntakeRejectionSummary;
  metrics: IntakeRejectionMetrics | null;
  busy: boolean;
  onShowRejected: () => void;
  onShowRejected837: () => void;
  onDrilldown: (item: IntakeRejectionCount) => void;
  onSelect: (message: InboundMessage) => void;
  onReplay: (messageId: string) => void;
}) {
  const total = metrics?.total ?? summary.messages.length;
  const latest = (metrics?.latest.length ? metrics.latest : summary.messages).slice(0, 5);
  const byPartner = metrics?.byPartner.length ? metrics.byPartner : summary.byPartner;
  const byType = metrics?.byType.length ? metrics.byType : summary.byType;
  const byReason = metrics?.byReason.length ? metrics.byReason : summary.byReason;
  return (
    <section className="intake-rejection-panel" aria-label="intake rejections">
      <div className="relationship-heading">
        <div>
          <h3>Partner Rejection Ops</h3>
          <p className="muted">Trend, drilldown, and replay view for partner profile failures.</p>
        </div>
        <span>{total} rejected</span>
      </div>
      <div className="rejection-actions">
        <button type="button" className="secondary" onClick={onShowRejected}>Show Rejected</button>
        <button type="button" className="secondary" onClick={onShowRejected837}>Rejected 837s</button>
      </div>
      {total === 0 ? (
        <p className="muted">No rejected intake messages match the current filters.</p>
      ) : (
        <>
          <RejectionTrendChart trend={metrics?.trend ?? []} />
          <div className="rejection-stats">
            <RejectionCountList title="Partners" items={byPartner} onSelect={onDrilldown} />
            <RejectionCountList title="Types" items={byType} onSelect={onDrilldown} />
            <RejectionCountList title="Reasons" items={byReason} onSelect={onDrilldown} />
          </div>
          <div className="document-review-list">
            <h4>Latest Rejections</h4>
            {latest.map((message) => (
              <article key={message.id} className="document-review-row">
                <div>
                  <span>{message.transactionType || "unknown"} · {message.partnerId || "unknown partner"}</span>
                  <strong>{rejectionReasonLabel(message.error)}</strong>
                  <small>{message.error || "No error detail"} · {new Date(message.createdAt).toLocaleString()}</small>
                </div>
                <div className="review-actions">
                  <button className="secondary" type="button" onClick={() => onSelect(message)}>Inspect</button>
                  <button type="button" disabled={busy} onClick={() => onReplay(message.id)}>Replay</button>
                </div>
              </article>
            ))}
          </div>
        </>
      )}
    </section>
  );
}

function RejectionTrendChart({ trend }: { trend: IntakeRejectionTrend[] }) {
  const maxCount = Math.max(1, ...trend.map((item) => item.count));
  return (
    <div className="rejection-trend" aria-label="rejection trend">
      <div className="relationship-heading compact">
        <h4>Rejection Trend</h4>
        <span>{trend.length} day window</span>
      </div>
      {trend.length === 0 ? (
        <p className="muted">No trend buckets yet.</p>
      ) : (
        <div className="trend-bars">
          {trend.map((item) => (
            <div key={item.date} className="trend-bar-item">
              <span>{item.count}</span>
              <div className="trend-bar-track">
                <div className="trend-bar-fill" style={{ height: `${Math.max(12, (item.count / maxCount) * 100)}%` }} />
              </div>
              <small>{shortDate(item.date)}</small>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

function RejectionCountList({ title, items, onSelect }: { title: string; items: Array<IntakeRejectionCount | { label: string; count: number }>; onSelect?: (item: IntakeRejectionCount) => void }) {
  return (
    <article>
      <span>{title}</span>
      {items.length === 0 ? (
        <strong>—</strong>
      ) : (
        items.map((item) => (
          <button key={item.label} type="button" onClick={() => onSelect?.(item as IntakeRejectionCount)}>
            {item.label} · {item.count}
          </button>
        ))
      )}
    </article>
  );
}

function shortDate(value: string) {
  const date = new Date(`${value}T00:00:00`);
  if (Number.isNaN(date.getTime())) return value;
  return date.toLocaleDateString(undefined, { month: "short", day: "numeric" });
}

function TimelineGroupCard({ group, onSelect }: { group: TimelineGroup; onSelect: (transactionId: string) => void }) {
  const packetSummaries = attachmentPacketSummaries(group.transactions);

  return (
    <article className="timeline-card">
      <div className="timeline-heading">
        <div>
          <strong>{group.title}</strong>
          <span>{group.subtitle}</span>
        </div>
        <span className="timeline-count">{group.transactions.length} events</span>
      </div>
      <div className="timeline-chain">
        {group.transactions.map((transaction) => (
          <button className="timeline-step" key={transaction.id} onClick={() => onSelect(transaction.id)}>
            <span className="timeline-dot" />
            <strong>{transaction.type}</strong>
            <small>{timelineStepDetail(transaction)}</small>
            <em>{new Date(transaction.createdAt).toLocaleTimeString()}</em>
          </button>
        ))}
      </div>
      {packetSummaries.length > 0 && (
        <div className="timeline-packets" aria-label="Attachment packet summary">
          <strong>275 Packet Summary</strong>
          {packetSummaries.map((packet) => (
            <button type="button" key={packet.packetId} onClick={() => onSelect(firstPacketTransactionID(group.transactions, packet.packetId))}>
              <span>{packet.packetId}</span>
              <small>{packet.observedCount}/{packet.declaredCount ?? packet.observedCount} docs observed · {packet.reviewStatuses.join(", ")}</small>
              <em>{packet.labels.join(" · ")}</em>
            </button>
          ))}
        </div>
      )}
    </article>
  );
}

function TransactionRelationshipGraph({
  relationshipMap,
  onSelect
}: {
  relationshipMap: TransactionRelationshipMap;
  onSelect: (transactionId: string) => void;
}) {
  const hasParent = Boolean(relationshipMap.parent);
  const hasChildren = relationshipMap.children.length > 0;

  return (
    <section className="relationship-map" aria-label="Transaction relationship map">
      <div className="relationship-heading">
        <div>
          <h3>Request / Response Links</h3>
          <p className="muted">Follow acknowledgments, attachments, and paired transactions without leaving the detail drawer.</p>
        </div>
        <span>{hasParent || hasChildren ? "Linked" : "Standalone"}</span>
      </div>
      <div className="relationship-chain">
        {relationshipMap.parent ? (
          <RelationshipNode transaction={relationshipMap.parent} label="Source" onSelect={onSelect} />
        ) : (
          <div className="relationship-empty">No parent</div>
        )}
        <RelationshipNode transaction={relationshipMap.current} label="Current" active onSelect={onSelect} />
        {hasChildren ? (
          <div className="relationship-children">
            {relationshipMap.children.map((transaction) => (
              <RelationshipNode key={transaction.id} transaction={transaction} label="Response" onSelect={onSelect} />
            ))}
          </div>
        ) : (
          <div className="relationship-empty">No responses yet</div>
        )}
      </div>
    </section>
  );
}

function RelationshipNode({
  transaction,
  label,
  active = false,
  onSelect
}: {
  transaction: Transaction;
  label: string;
  active?: boolean;
  onSelect: (transactionId: string) => void;
}) {
  return (
    <button className={`relationship-node ${active ? "active" : ""}`} type="button" onClick={() => onSelect(transaction.id)}>
      <span>{label}</span>
      <strong>{transaction.type} · {transaction.status}</strong>
      <small>{transaction.id}</small>
    </button>
  );
}

function buildTimelineGroups(transactions: Transaction[]) {
  const transactionsByID = new Map(transactions.map((transaction) => [transaction.id, transaction]));
  const groups = new Map<string, TimelineGroup>();

  transactions.forEach((transaction) => {
    const parent = transaction.relatedId ? transactionsByID.get(transaction.relatedId) : undefined;
    const claimId = transactionClaimId(transaction) ?? (parent ? transactionClaimId(parent) : undefined);
    const adventurerId = transactionAdventurerId(transaction) ?? (parent ? transactionAdventurerId(parent) : undefined);
    const groupKey = claimId ? `claim:${claimId}` : adventurerId ? `adventurer:${adventurerId}` : transaction.relatedId ? `related:${transaction.relatedId}` : `transaction:${transaction.id}`;
    const existing = groups.get(groupKey);

    if (existing) {
      existing.transactions.push(transaction);
      existing.latestAt = Math.max(existing.latestAt, Date.parse(transaction.createdAt));
      return;
    }

    groups.set(groupKey, {
      id: groupKey,
      title: timelineTitle(transaction, claimId, adventurerId),
      subtitle: timelineSubtitle(transaction, claimId, adventurerId),
      transactions: [transaction],
      latestAt: Date.parse(transaction.createdAt)
    });
  });

  return Array.from(groups.values())
    .map((group) => ({
      ...group,
      transactions: group.transactions.sort((left, right) => Date.parse(left.createdAt) - Date.parse(right.createdAt))
    }))
    .sort((left, right) => right.latestAt - left.latestAt);
}

function buildTransactionRelationshipMap(current: Transaction, transactions: Transaction[]): TransactionRelationshipMap {
  const transactionsByID = new Map(transactions.map((transaction) => [transaction.id, transaction]));
  const parent = current.relatedId ? transactionsByID.get(current.relatedId) : undefined;
  const children = transactions
    .filter((transaction) => transaction.relatedId === current.id)
    .sort((left, right) => Date.parse(left.createdAt) - Date.parse(right.createdAt));

  return {
    parent,
    current,
    children
  };
}

function buildAcknowledgmentDrilldowns(transactions: Transaction[]): AcknowledgmentDrilldown[] {
  const byType = (type: string) => transactions.filter((transaction) => transaction.type === type);
  const businessReviews = transactions.filter(isBusinessReviewTransaction);

  return [
    {
      id: "interchange",
      label: "TA1 Interchange",
      typeFilter: "TA1",
      count: byType("TA1").length,
      detail: "Envelope or interchange pre-screen failures before transaction translation.",
      examples: byType("TA1")
    },
    {
      id: "syntax",
      label: "999 Syntax",
      typeFilter: "999",
      count: byType("999").length,
      detail: "Implementation acknowledgments for accepted or rejected transaction syntax.",
      examples: byType("999")
    },
    {
      id: "application",
      label: "824 Application",
      typeFilter: "824",
      count: byType("824").length,
      detail: "Application advice for companion-guide or attachment-validation failures.",
      examples: byType("824")
    },
    {
      id: "business-review",
      label: "Business Review",
      typeFilter: "275",
      count: businessReviews.length,
      detail: "Manual authorization or attachment review outcomes after EDI acceptance.",
      examples: businessReviews
    }
  ];
}

function isBusinessReviewTransaction(transaction: Transaction) {
  if (transaction.type === "275" && payloadString(transaction, "attachmentReviewStatus")) {
    return true;
  }
  if (transaction.type === "278" && ["Pending", "Approved", "Denied"].includes(transaction.status)) {
    return true;
  }
  return Boolean(payloadString(transaction, "authorizationStatus") || payloadString(transaction, "attachmentReviewReason"));
}

function claimAttachmentTransactions(claim: Claim, transactions: Transaction[]) {
  return transactions
    .filter((transaction) => transaction.type === "275")
    .filter((transaction) => payloadString(transaction, "claimId") === claim.id || transaction.relatedId === claim.transactionId)
    .sort((left, right) => {
      const leftSequence = Number(payloadString(left, "packetSequence") ?? "0");
      const rightSequence = Number(payloadString(right, "packetSequence") ?? "0");
      if (leftSequence !== rightSequence) return leftSequence - rightSequence;
      return Date.parse(left.createdAt) - Date.parse(right.createdAt);
    });
}

function latestClaimAdjudication(claim: Claim, transactions: Transaction[]): AdjudicationExplanation | null {
  const matching = transactions
    .filter((transaction) => transaction.type === "277")
    .filter((transaction) => payloadString(transaction, "claimId") === claim.id || transaction.relatedId === claim.transactionId)
    .filter((transaction) => Boolean(payloadRecord(payloadRecord(transaction.payload)?.adjudication)))
    .sort((left, right) => Date.parse(right.createdAt) - Date.parse(left.createdAt));

  const adjudication = payloadRecord(payloadRecord(matching[0]?.payload)?.adjudication);
  if (!adjudication) {
    return null;
  }

  return {
    engine: valueString(adjudication.engine),
    allowedAmountCents: valueNumber(adjudication.allowedAmountCents),
    paidAmountCents: valueNumber(adjudication.paidAmountCents),
    patientResponsibilityCents: valueNumber(adjudication.patientResponsibilityCents),
    adjustmentAmountCents: valueNumber(adjudication.adjustmentAmountCents),
    adjustmentReason: valueString(adjudication.adjustmentReason),
    denialReason: valueString(adjudication.denialReason),
    coverageStatus: valueString(adjudication.coverageStatus),
    providerTier: valueString(adjudication.providerTier),
    adventurerRank: valueString(adjudication.adventurerRank),
    premiumCurrent: valueBool(adjudication.premiumCurrent),
    premiumPaidAmountCents: valueNumber(adjudication.premiumPaidAmountCents)
  };
}

function authorizationDocumentationTransactions(authorizationTransaction: Transaction, transactions: Transaction[]) {
  return transactions
    .filter((transaction) => transaction.type === "275")
    .filter((transaction) => payloadString(transaction, "authorizationTransactionId") === authorizationTransaction.id || transaction.relatedId === authorizationTransaction.id)
    .sort((left, right) => {
      const leftSequence = Number(payloadString(left, "packetSequence") ?? "0");
      const rightSequence = Number(payloadString(right, "packetSequence") ?? "0");
      if (leftSequence !== rightSequence) return leftSequence - rightSequence;
      return Date.parse(left.createdAt) - Date.parse(right.createdAt);
    });
}

function checklistItemForTransaction(transaction: Transaction) {
  const description = payloadString(transaction, "description");
  const documentReferenceId = payloadString(transaction, "documentReferenceId") ?? "";
  const attachmentControlNumber = payloadString(transaction, "attachmentControlNumber") ?? "";
  return documentationChecklist.find((item) => (
    item.label === description ||
    documentReferenceId.includes(item.code.toLowerCase()) ||
    attachmentControlNumber.includes(item.code)
  )) ?? documentationChecklist[0];
}

function buildAttachmentDraft(claim: Claim, item: DocumentationChecklistItem, packetId: string, sequence: number, count: number, mode = "initial"): AttachmentDraft {
  const claimToken = claim.id.slice(0, 8);
  const modeSuffix = mode === "resubmission" ? "-RESUB" : "";
  return {
    packetId,
    attachmentType: item.attachmentType,
    attachmentControlNumber: `ATTACH-${item.code}-${claimToken.toUpperCase()}${modeSuffix}`,
    reportTypeCode: item.reportTypeCode,
    transmissionCode: "EL",
    contentType: item.contentType,
    description: mode === "resubmission" ? `${item.label} resubmission` : item.label,
    documentReferenceId: `${item.code.toLowerCase()}-${claimToken}${mode === "resubmission" ? "-resub" : ""}`,
    documentReferenceUrl: `https://docs.example.test/${claim.id}/${item.code.toLowerCase()}${mode === "resubmission" ? "-resub" : ""}.txt`,
    content: `${item.label} ${mode === "resubmission" ? "corrected resubmission" : "supporting document"} for claim ${claim.id}. Packet document ${sequence} of ${count}.`
  };
}

function buildAuthorizationAttachmentDraft(transaction: Transaction, item: DocumentationChecklistItem, packetId: string, sequence: number, count: number): AttachmentDraft {
  const token = transaction.id.slice(0, 8);
  return {
    packetId,
    packetSequence: sequence,
    packetCount: count,
    attachmentType: item.attachmentType,
    attachmentControlNumber: `ATTACH-AUTH-${item.code}-${token.toUpperCase()}`,
    reportTypeCode: item.reportTypeCode,
    transmissionCode: "EL",
    contentType: item.contentType,
    description: `${item.label} for authorization`,
    documentReferenceId: `auth-${item.code.toLowerCase()}-${token}`,
    documentReferenceUrl: `https://docs.example.test/auth/${transaction.id}/${item.code.toLowerCase()}.txt`,
    content: `${item.label} supporting document ${sequence} of ${count} for authorization ${transaction.id}.`
  };
}

function checklistForAuthorization(transaction: Transaction): DocumentationChecklistItem[] {
  const payload = payloadRecord(transaction.payload);
  const rawChecklist = Array.isArray(payload?.requiredDocuments) ? payload.requiredDocuments : undefined;
  const checklist = rawChecklist
    ?.map((item) => documentationChecklistItemFromPayload(item))
    .filter((item): item is DocumentationChecklistItem => Boolean(item));
  return checklist && checklist.length > 0 ? checklist : documentationChecklist.slice(0, 2);
}

function documentationChecklistItemFromPayload(value: unknown): DocumentationChecklistItem | undefined {
  const item = payloadRecord(value);
  if (!item) return undefined;
  const code = stringFromUnknown(item.code);
  const label = stringFromUnknown(item.label);
  const attachmentType = stringFromUnknown(item.attachmentType);
  const reportTypeCode = stringFromUnknown(item.reportTypeCode);
  const contentType = stringFromUnknown(item.contentType);
  if (!code || !label || !attachmentType || !reportTypeCode || !contentType) return undefined;
  return { code, label, attachmentType, reportTypeCode, contentType, required: item.required !== false };
}

function authorizationReviewPrompts(transaction: Transaction): string[] {
  const payload = payloadRecord(transaction.payload);
  const prompts = Array.isArray(payload?.manualReviewPrompts) ? payload.manualReviewPrompts : [];
  return prompts.map((prompt) => stringFromUnknown(prompt)).filter((prompt): prompt is string => Boolean(prompt));
}

function timelineTitle(transaction: Transaction, claimId?: string, adventurerId?: string) {
  if (claimId) return "Claim lifecycle";
  const adventurerName = payloadNestedString(transaction, "adventurer", "name");
  if (adventurerName) return `Adventurer lifecycle: ${adventurerName}`;
  if (adventurerId) return "Adventurer lifecycle";
  if (transaction.type === "999") return "Implementation acknowledgment";
  if (transaction.type === "824") return "Application advice";
  if (transaction.type === "TA1") return "Interchange acknowledgment";
  return "Standalone transaction";
}

function timelineSubtitle(transaction: Transaction, claimId?: string, adventurerId?: string) {
  if (claimId) return `Claim ${claimId}`;
  if (adventurerId) return `Adventurer ${adventurerId}`;
  if (transaction.relatedId) return `Related to ${transaction.relatedId}`;
  return transaction.id;
}

function timelineStepDetail(transaction: Transaction) {
  if (transaction.type === "275") {
    const attachmentType = payloadString(transaction, "attachmentType");
    const reportTypeCode = payloadString(transaction, "reportTypeCode");
    const reviewStatus = payloadString(transaction, "attachmentReviewStatus") ?? "Received";
    const attachmentLabel = [attachmentType, reportTypeCode].filter(Boolean).join("/");
    const packetLabel = attachmentPacketLabel(transaction);
    const suffix = packetLabel ? ` · ${packetLabel}` : "";
    return attachmentLabel ? `${attachmentLabel} attachment · Review ${reviewStatus}${suffix}` : `Attachment · Review ${reviewStatus}${suffix}`;
  }
  return transaction.status;
}

function attachmentPacketLabel(transaction: Transaction) {
  const packetId = payloadString(transaction, "packetId");
  const packetSequence = payloadScalarString(transaction, "packetSequence");
  const packetCount = payloadScalarString(transaction, "packetCount");
  if (!packetId) return undefined;
  if (packetSequence && packetCount) return `${packetId} (${packetSequence}/${packetCount})`;
  return packetId;
}

function attachmentPacketSummaries(transactions: Transaction[]): AttachmentPacketSummary[] {
  const packets = new Map<string, AttachmentPacketSummary>();

  transactions
    .filter((transaction) => transaction.type === "275")
    .forEach((transaction) => {
      const packetId = payloadString(transaction, "packetId");
      if (!packetId) return;

      const declaredCount = payloadNumber(transaction, "packetCount");
      const reviewStatus = payloadString(transaction, "attachmentReviewStatus") ?? "Received";
      const attachmentType = payloadString(transaction, "attachmentType");
      const reportTypeCode = payloadString(transaction, "reportTypeCode");
      const label = [attachmentType, reportTypeCode].filter(Boolean).join("/") || "Attachment";
      const packet = packets.get(packetId) ?? {
        packetId,
        observedCount: 0,
        declaredCount,
        acceptedCount: 0,
        rejectedCount: 0,
        reviewStatuses: [],
        labels: []
      };

      packet.observedCount += 1;
      packet.declaredCount = Math.max(packet.declaredCount ?? 0, declaredCount ?? 0) || packet.declaredCount;
      if (transaction.status === "Accepted") packet.acceptedCount += 1;
      if (reviewStatus === "Rejected") packet.rejectedCount += 1;
      if (!packet.reviewStatuses.includes(reviewStatus)) packet.reviewStatuses.push(reviewStatus);
      if (!packet.labels.includes(label)) packet.labels.push(label);
      packets.set(packetId, packet);
    });

  return Array.from(packets.values()).sort((left, right) => left.packetId.localeCompare(right.packetId));
}

function firstPacketTransactionID(transactions: Transaction[], packetId: string) {
  return transactions.find((transaction) => transaction.type === "275" && payloadString(transaction, "packetId") === packetId)?.id ?? transactions[0]?.id;
}

function transactionClaimId(transaction: Transaction) {
  return payloadString(transaction, "claimId") ?? payloadNestedString(transaction, "claim", "id");
}

function transactionAdventurerId(transaction: Transaction) {
  return payloadString(transaction, "adventurerId") ?? payloadNestedString(transaction, "claim", "adventurerId") ?? payloadNestedString(transaction, "adventurer", "id");
}

function authorizationReviewSummary(transaction: Transaction) {
  const serviceType = payloadString(transaction, "serviceType");
  if (serviceType === "dental-predetermination") {
    const cdtCode = payloadNestedString(transaction, "dentalService", "cdtCode");
    const toothNumber = payloadNestedString(transaction, "dentalService", "toothNumber");
    const parts = [cdtCode && `CDT ${cdtCode}`, toothNumber && `tooth ${toothNumber}`].filter(Boolean).join(" · ");
    return `Manual council review can approve or deny this dental predetermination${parts ? ` for ${parts}` : ""} before the worker decides.`;
  }
  return "Manual council review can approve or deny the pending resurrection authorization before the worker decides.";
}

function raw275Fixture(options: {
  control: string;
  bgn01: string;
  trace: string;
  claimId: string;
  attachmentControl: string;
  cat02: string;
  bds01: string;
  content: string;
  serviceDate?: string;
  attachmentCount?: number;
}) {
  const attachmentCount = options.attachmentCount ?? 1;
  const attachmentLoops = Array.from({ length: attachmentCount }, (_, index) => {
    const sequence = index + 1;
    const suffix = attachmentCount > 1 ? `-${sequence}` : "";
    return [
      `LX*${sequence}~`,
      `TRN*1*${options.trace || `missing-trace-${sequence}`}*provider-vitesse-temple~`,
      `REF*1K*${options.claimId}~`,
      `REF*6R*${options.attachmentControl}${suffix}~`,
      `DTP*472*D8*${options.serviceDate ?? "20260708"}~`,
      "LQ*AT*OZ~",
      `CAT*B4*${options.cat02}~`,
      "OOI*DOC~",
      `BDS*${options.bds01}**Content-Type: text/plain~`,
      "NTE*ADD*275 rejection fixture~",
      `BIN*${options.content.length}*${options.content}~`
    ].join("\n");
  }).join("\n");

  return [
    `ISA*00*          *00*          *ZZ*provider-vitesse-temple*ZZ*Adventure Society*260708*1200*^*00501*${options.control}*0*T*:~`,
    `GS*HC*provider-vitesse-temple*Adventure Society*20260708*1200*${options.control}*X*005010X275A1~`,
    `ST*275*${options.control}~`,
    `BGN*${options.bgn01}*${options.trace}*20260708~`,
    "NM1*41*2*provider-vitesse-temple*****46*provider-vitesse-temple~",
    "NM1*1P*2*provider-vitesse-temple*****XX*provider-vitesse-temple~",
    "NM1*PR*2*Adventure Society*****PI*adventure-society~",
    attachmentLoops,
    `SE*${8 + attachmentCount * 11}*${options.control}~`,
    `GE*1*${options.control}~`,
    `IEA*1*${options.control}~`
  ].join("\n");
}

function payloadString(transaction: Transaction, key: string) {
  const payload = payloadRecord(transaction.payload);
  const value = payload?.[key];
  return typeof value === "string" && value ? value : undefined;
}

function payloadScalarString(transaction: Transaction, key: string) {
  const payload = payloadRecord(transaction.payload);
  const value = payload?.[key];
  if (typeof value === "string" && value) return value;
  if (typeof value === "number" && Number.isFinite(value)) return String(value);
  return undefined;
}

function payloadNumber(transaction: Transaction, key: string) {
  const value = payloadScalarString(transaction, key);
  if (!value) return undefined;
  const parsed = Number(value);
  return Number.isFinite(parsed) ? parsed : undefined;
}

function hasDocumentReference(transaction: Transaction) {
  return Boolean(
    payloadString(transaction, "documentReferenceId") ||
    payloadString(transaction, "documentReferenceUrl") ||
    payloadString(transaction, "content")
  );
}

function payloadNestedString(transaction: Transaction, parentKey: string, childKey: string) {
  const payload = payloadRecord(transaction.payload);
  const parent = payloadRecord(payload?.[parentKey]);
  const value = parent?.[childKey];
  return typeof value === "string" && value ? value : undefined;
}

function payloadRecord(value: unknown): Record<string, unknown> | undefined {
  if (!value || typeof value !== "object" || Array.isArray(value)) return undefined;
  return value as Record<string, unknown>;
}

function stringFromUnknown(value: unknown) {
  return typeof value === "string" && value ? value : undefined;
}

function valueString(value: unknown) {
  return typeof value === "string" && value ? value : undefined;
}

function valueNumber(value: unknown) {
  return typeof value === "number" && Number.isFinite(value) ? value : undefined;
}

function valueBool(value: unknown) {
  return typeof value === "boolean" ? value : undefined;
}

function isNonEmptyString(value: unknown): value is string {
  return typeof value === "string" && value.length > 0;
}

function requireScenarioData<T>(value: T | undefined, message: string): T {
  if (value === undefined || value === null) {
    throw new Error(message);
  }
  return value;
}

function demoScenarioExport(scenario: DemoScenario) {
  return {
    schema: "ashn.demo-scenario.v1",
    exportedAt: new Date().toISOString(),
    scenario,
    runbook: {
      prerequisites: [
        "Start the local stack with `make dev-stack` or open the deployed dashboard.",
        "Use `make demo-reset` before formal demos when a clean database is preferred.",
        "Keep the Ledger and Timeline tabs available for transaction evidence."
      ],
      evidenceToExport: scenario.exports,
      followUpQuestions: [
        "Which transaction proves the request was accepted?",
        "Which related transaction explains the business outcome?",
        "Which payload would an external partner need to debug or replay?"
      ]
    }
  };
}

function demoScenarioEvidenceBundle(scenario: DemoScenario, runState: ScenarioRunState) {
  const transactionIds = Array.from(new Set((runState.evidence ?? []).flatMap((step) => step.transactionIds)));
  const claimIds = Array.from(new Set((runState.evidence ?? []).map((step) => step.claimId).filter(isNonEmptyString)));
  const adventurerIds = Array.from(new Set((runState.evidence ?? []).map((step) => step.adventurerId).filter(isNonEmptyString)));
  return {
    schema: "ashn.demo-evidence.v1",
    exportedAt: new Date().toISOString(),
    run: {
      id: runState.runId,
      scenarioId: scenario.id,
      startedAt: runState.startedAt,
      completedAt: runState.completedAt,
      completedSteps: runState.completedSteps,
      totalSteps: scenario.steps.length,
      status: runState.error ? "failed" : "completed",
      error: runState.error
    },
    scenario: demoScenarioExport(scenario),
    evidence: {
      steps: runState.evidence ?? [],
      transactionIds,
      claimIds,
      adventurerIds,
      suggestedExports: scenario.exports,
      artifactHints: transactionIds.map((id) => ({
        transactionId: id,
        json: `/v1/transactions/${id}/export?format=json`,
        xml: `/v1/transactions/${id}/export?format=xml`,
        x12: `/v1/transactions/${id}/export?format=x12`
      }))
    }
  };
}

function scenarioStepEvidence(step: DemoScenario["steps"][number], result: Envelope): ScenarioStepEvidence {
  const transactions = result.transactions ?? (result.transaction ? [result.transaction] : []);
  const dataRecord = payloadRecord(result.data);
  const transactionTypes = transactions.map((transaction) => transaction.type).filter(Boolean);
  return {
    label: step.label,
    action: step.action,
    expected: step.expected,
    completedAt: new Date().toISOString(),
    transactionIds: transactions.map((transaction) => transaction.id).filter(isNonEmptyString),
    transactionTypes,
    relatedIds: transactions.map((transaction) => transaction.relatedId).filter(isNonEmptyString),
    claimId: valueString(dataRecord?.claimId) ?? (transactionTypes.includes("837") ? valueString(dataRecord?.id) : undefined) ?? transactions.map(transactionClaimId).find(Boolean),
    adventurerId: valueString(dataRecord?.adventurerId) ?? (transactionTypes.includes("834") ? valueString(dataRecord?.id) : undefined) ?? transactions.map(transactionAdventurerId).find(Boolean),
    lore: result.lore,
    error: result.error
  };
}

function transactionPayloadView(transaction: Transaction, tab: PayloadTab) {
  if (tab === "x12") {
    const value = transaction.rawX12 ?? "No raw X12 was generated for this transaction.";
    return {
      label: "X12",
      value,
      extension: "x12",
      filename: `ashn-${transaction.type}-${transaction.id}.x12`,
      canDownload: Boolean(transaction.rawX12)
    };
  }

  if (tab === "xml") {
    const value = transactionXMLPreview(transaction);
    return {
      label: "XML",
      value,
      extension: "xml",
      filename: `ashn-${transaction.type}-${transaction.id}.xml`,
      canDownload: true
    };
  }

  return {
    label: "JSON",
    value: JSON.stringify(transaction.payload ?? null, null, 2),
    extension: "json",
    filename: `ashn-${transaction.type}-${transaction.id}.json`,
    canDownload: true
  };
}

function downloadFilename(response: Response, path: string) {
  const disposition = response.headers.get("Content-Disposition") ?? "";
  const match = disposition.match(/filename="?([^";]+)"?/i);
  if (match?.[1]) {
    return match[1];
  }
  const format = new URL(path, "http://ashn.local").searchParams.get("format") ?? "json";
  return `ashn-export.${format}`;
}

function transactionsToCSV(transactions: Transaction[]) {
  const headers = ["id", "type", "status", "senderId", "receiverId", "relatedId", "createdAt", "payload"];
  const rows = transactions.map((transaction) => [
    transaction.id,
    transaction.type,
    transaction.status,
    transaction.senderId,
    transaction.receiverId,
    transaction.relatedId ?? "",
    transaction.createdAt,
    JSON.stringify(transaction.payload ?? null)
  ]);

  return [headers, ...rows]
    .map((row) => row.map(csvCell).join(","))
    .join("\n");
}

function csvCell(value: string) {
  if (/[",\n\r]/.test(value)) return `"${value.replace(/"/g, "\"\"")}"`;
  return value;
}

function transactionXMLPreview(transaction: Transaction) {
  return [
    `<AshnTransaction id="${escapeXML(transaction.id)}" type="${escapeXML(transaction.type)}" status="${escapeXML(transaction.status)}">`,
    `  <SenderId>${escapeXML(transaction.senderId)}</SenderId>`,
    `  <ReceiverId>${escapeXML(transaction.receiverId)}</ReceiverId>`,
    `  <RelatedId>${escapeXML(transaction.relatedId ?? "")}</RelatedId>`,
    `  <CreatedAt>${escapeXML(transaction.createdAt)}</CreatedAt>`,
    `  <PayloadJson>${escapeXML(JSON.stringify(transaction.payload ?? null, null, 2))}</PayloadJson>`,
    `</AshnTransaction>`
  ].join("\n");
}

function escapeXML(value: string) {
  return value
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;")
    .replace(/'/g, "&apos;");
}

function TransactionRow({ transaction, onSelect }: { transaction: Transaction; onSelect: (transactionId: string) => void }) {
  return (
    <article className="compact-row clickable" onClick={() => onSelect(transaction.id)}>
      <div>
        <strong>{transaction.type} · {transaction.status}</strong>
        <span>{new Date(transaction.createdAt).toLocaleString()}</span>
      </div>
      <code>{transaction.id}</code>
    </article>
  );
}

function InboundMessageRow({
  message,
  busy,
  onSelect,
  onReplay
}: {
  message: InboundMessage;
  busy: boolean;
  onSelect: (message: InboundMessage) => void;
  onReplay: (messageId: string) => void;
}) {
  return (
    <article className="compact-row">
      <div>
        <strong>{message.transactionType || "unknown"} · {message.status}</strong>
        <span>{message.downstreamStatus ? `${message.downstreamStatus} · ` : ""}{new Date(message.createdAt).toLocaleString()}</span>
        {message.error && <span>{rejectionReasonLabel(message.error)} · {message.error}</span>}
      </div>
      <code>{message.id}</code>
      <div className="row-actions">
        <button className="secondary" type="button" onClick={() => onSelect(message)}>Inspect</button>
        {message.status === "rejected" && <button type="button" disabled={busy} onClick={() => onReplay(message.id)}>Replay</button>}
      </div>
    </article>
  );
}

function JobRow({ job, busy, onReplay }: { job: TransactionJob; busy: boolean; onReplay: (jobId: string) => void }) {
  return (
    <article className="compact-row">
      <div>
        <strong>{job.type} · {job.status}{job.deadLetter ? " · Dead Letter" : ""}</strong>
        <span>{job.attempts} attempt{job.attempts === 1 ? "" : "s"} · runs {new Date(job.runAfter).toLocaleTimeString()} · entity {job.entityId}</span>
        {job.lastError && <span>{job.lastError}</span>}
      </div>
      <div className="row-actions">
        <code>{job.id}</code>
        {job.deadLetter && <button className="secondary" disabled={busy} onClick={() => onReplay(job.id)}>Replay</button>}
      </div>
    </article>
  );
}

function ClaimRow({ claim, onSelect }: { claim: Claim; onSelect: (claimId: string) => void }) {
  return (
    <article className="compact-row clickable" onClick={() => onSelect(claim.id)}>
      <div>
        <strong>{claim.status} · {claim.incidentSeverity}</strong>
        <span>{money(claim.amountCents)} billed · {money(claim.paidAmountCents)} paid · provider {claim.providerId}</span>
      </div>
      <code>{claim.id}</code>
    </article>
  );
}

function PremiumPaymentRow({ payment, onSelect }: { payment: PremiumPayment; onSelect: (paymentId: string) => void }) {
  const benefitLabel = payment.currentForBenefits ? "Benefit-current" : "Historical";
  const reconciliationLabel = payment.reconciled ? "Reconciled" : "Needs review";
  return (
    <article className="compact-row clickable" onClick={() => onSelect(payment.id)}>
      <div>
        <strong>{money(payment.amountCents)} · {reconciliationLabel}</strong>
        <span>{benefitLabel} · {payment.status} · {new Date(payment.createdAt).toLocaleString()}</span>
        <span>Adventurer {payment.adventurerId}</span>
      </div>
      <code>{payment.transactionId}</code>
    </article>
  );
}

function premiumPaymentToCSV(payment: PremiumPayment) {
  const headers = ["id", "adventurerId", "transactionId", "amountCents", "status", "createdAt", "reconciled", "currentForBenefits"];
  const row = [
    payment.id,
    payment.adventurerId,
    payment.transactionId,
    String(payment.amountCents),
    payment.status,
    payment.createdAt,
    String(payment.reconciled),
    String(payment.currentForBenefits)
  ];
  return [headers, row].map((items) => items.map(csvCell).join(",")).join("\n");
}

function AdventurerRow({ adventurer }: { adventurer: Adventurer }) {
  return (
    <article className="compact-row">
      <div>
        <strong>{adventurer.name}</strong>
        <span>{adventurer.rank} · {adventurer.region} · {adventurer.coverageStatus}</span>
      </div>
      <code>{adventurer.id}</code>
    </article>
  );
}

createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>
);
