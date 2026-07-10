import React, { FormEvent, useEffect, useMemo, useState } from "react";
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
    maxEmbeddedContentBytes?: number;
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
};

type DocumentationChecklistItem = {
  code: string;
  label: string;
  attachmentType: string;
  reportTypeCode: string;
  contentType: string;
  required: boolean;
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

type TimelineGroup = {
  id: string;
  title: string;
  subtitle: string;
  transactions: Transaction[];
  latestAt: number;
};

type TransactionRelationshipMap = {
  parent?: Transaction;
  current: Transaction;
  children: Transaction[];
};

type DashboardTab = "workflow" | "timeline" | "ledger" | "xml" | "partners";
type PayloadTab = "json" | "xml" | "x12";

type InboundMessage = {
  id: string;
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

const apiUrl = import.meta.env.VITE_ASHN_API_URL ?? "http://localhost:8080";
const adventurerPageSize = 10;
const claimPageSize = 10;
const transactionPageSize = 25;
const auditPageSize = 10;
const dashboardRefreshMs = 3000;
const transactionTypes = ["All", "834", "820", "270", "271", "275", "278", "837", "835", "276", "277", "269", "999", "277CA"];
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
const savedFiltersStorageKey = "ashn.savedFilters.v1";
const initialPartnerForm: PartnerFormState = {
  id: "",
  name: "",
  senderId: "",
  receiverId: "Adventure Society",
  allowedTransactionTypes: "270,275,276,278,837",
  routeTarget: "payer-core",
  status: "active"
};
const dashboardTabs: { id: DashboardTab; label: string; detail: string }[] = [
  { id: "workflow", label: "Workflow", detail: "Run the demo flow" },
  { id: "timeline", label: "Timeline", detail: "Follow transaction chains" },
  { id: "ledger", label: "Ledger", detail: "Browse DB records" },
  { id: "xml", label: "XML Intake", detail: "Inspect inbound audits" },
  { id: "partners", label: "Partners", detail: "Review routing profiles" }
];
const filterTabs: DashboardTab[] = ["timeline", "ledger", "xml"];

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

function storeSavedFilters(filters: SavedFilter[]) {
  window.localStorage.setItem(savedFiltersStorageKey, JSON.stringify(filters));
}

function App() {
  const [health, setHealth] = useState<Envelope<Record<string, string>> | null>(null);
  const [providers, setProviders] = useState<Provider[]>([]);
  const [tradingPartners, setTradingPartners] = useState<TradingPartner[]>([]);
  const [selectedProviderId, setSelectedProviderId] = useState("provider-vitesse-temple");
  const [adventurer, setAdventurer] = useState<Adventurer | null>(null);
  const [claim, setClaim] = useState<Claim | null>(null);
  const [recentAdventurers, setRecentAdventurers] = useState<Adventurer[]>([]);
  const [recentClaims, setRecentClaims] = useState<Claim[]>([]);
  const [recentTransactions, setRecentTransactions] = useState<Transaction[]>([]);
  const [inboundMessages, setInboundMessages] = useState<InboundMessage[]>([]);
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

  const providerFilters = useMemo(
    () => ["All", ...providers.map((provider) => provider.id)],
    [providers]
  );

  const timelineGroups = useMemo(
    () => buildTimelineGroups(recentTransactions),
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
    const [healthResult, providersResult, partnersResult, adventurersResult, claimsResult, transactionsResult, auditResult, jobsResult] = await Promise.allSettled([
      request<Record<string, string>>("/v1/health"),
      request<Provider[]>("/v1/providers"),
      request<TradingPartner[]>("/v1/x12/trading-partners"),
      request<Adventurer[]>(`/v1/adventurers?${adventurerQuery}`),
      request<Claim[]>(`/v1/claims?${claimQuery}`),
      request<Transaction[]>(`/v1/transactions?${transactionQuery}`),
      request<InboundMessage[]>(`/v1/x12/messages?${auditQuery}`),
      request<TransactionJob[]>("/v1/jobs?limit=8")
    ]);
    const healthEnvelope = settledValue(healthResult);
    const providersEnvelope = settledValue(providersResult);
    const partnersEnvelope = settledValue(partnersResult);
    const adventurersEnvelope = settledValue(adventurersResult);
    const claimsEnvelope = settledValue(claimsResult);
    const transactionsEnvelope = settledValue(transactionsResult);
    const auditEnvelope = settledValue(auditResult);
    const jobsEnvelope = settledValue(jobsResult);
    if (healthEnvelope) setHealth(healthEnvelope);
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
    if (transactionsEnvelope) {
      setRecentTransactions(transactionsEnvelope.data ?? []);
      setTransactionPage(transactionsEnvelope.page ?? { limit: transactionPageSize, offset: transactionOffset, count: transactionsEnvelope.data?.length ?? 0, hasMore: false });
    }
    if (auditEnvelope) {
      setInboundMessages(auditEnvelope.data ?? []);
      setAuditPage(auditEnvelope.page ?? { limit: auditPageSize, offset: auditOffset, count: auditEnvelope.data?.length ?? 0, hasMore: false });
    }
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
    const response = await fetch(`${apiUrl}${path}`, {
      ...init,
      headers: {
        "Content-Type": "application/json",
        ...(init?.headers ?? {})
      }
    });
    return (await response.json()) as Envelope<T>;
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

  function downloadFromPath(path: string) {
    const anchor = document.createElement("a");
    anchor.href = `${apiUrl}${path}`;
    document.body.appendChild(anchor);
    anchor.click();
    anchor.remove();
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
    if (!adventurer) return;
    setBusy(true);
    const result = await request("/v1/eligibility", {
      method: "POST",
      body: JSON.stringify({ adventurerId: adventurer.id, providerId: selectedProviderId })
    });
    pushEvent(result);
    await refresh();
    setBusy(false);
  }

  async function requestAuth() {
    if (!adventurer) return;
    setBusy(true);
    const result = await request<Record<string, string>>("/v1/auth-requests", {
      method: "POST",
      body: JSON.stringify({
        adventurerId: adventurer.id,
        providerId: selectedProviderId,
        serviceType: "resurrection",
        incidentSeverity: "Diamond"
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
    setBusy(true);
    const result = await request<Record<string, string>>(`/v1/auth-requests/${authorizationTransaction.id}/attachments`, {
      method: "POST",
      body: JSON.stringify({
        attachmentType: "OZ",
        attachmentControlNumber: `ATTACH-${authorizationTransaction.id.slice(0, 8).toUpperCase()}`,
        reportTypeCode: "B4",
        transmissionCode: "EL",
        contentType: "text/plain",
        description: "Prior authorization medical necessity notes",
        content: "Resurrection authorization includes encounter notes, severity evidence, and healer attestation."
      })
    });
    pushEvent(result);
    await refresh();
    setBusy(false);
  }

  async function submitClaim() {
    if (!adventurer) return;
    setBusy(true);
    const result = await request<Claim>("/v1/claims", {
      method: "POST",
      body: JSON.stringify({
        adventurerId: adventurer.id,
        providerId: selectedProviderId,
        incidentSeverity: "Awakened",
        amountCents: 125000,
        authorizationTransactionId: authorizationTransaction?.id
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
        attachments: documentationChecklist.map((item, index) => ({
          packetId,
          attachmentType: item.attachmentType,
          attachmentControlNumber: `ATTACH-${item.code}-${selectedClaim.id.slice(0, 8).toUpperCase()}`,
          reportTypeCode: item.reportTypeCode,
          transmissionCode: "EL",
          contentType: item.contentType,
          description: item.label,
          documentReferenceId: `${item.code.toLowerCase()}-${selectedClaim.id.slice(0, 8)}`,
          documentReferenceUrl: `https://docs.example.test/${selectedClaim.id}/${item.code.toLowerCase()}.txt`,
          content: `${item.label} for claim ${selectedClaim.id}. Packet document ${index + 1} of ${documentationChecklist.length}.`
        }))
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

  async function openClaimDetail(claimId: string) {
    setBusy(true);
    const result = await request<Claim>(`/v1/claims/${claimId}`);
    if (result.data) {
      setSelectedClaim(result.data);
      setSelectedTransaction(null);
      setSelectedInboundMessage(null);
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
    }
    setBusy(false);
  }

  function openInboundMessageDetail(message: InboundMessage) {
    setSelectedInboundMessage(message);
    setSelectedClaim(null);
    setSelectedTransaction(null);
  }

  function closeDetail() {
    setSelectedClaim(null);
    setSelectedTransaction(null);
    setSelectedInboundMessage(null);
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
            <button disabled={!adventurer || busy} onClick={requestAuth}>278 Resurrection Auth</button>
            <button disabled={!adventurer || busy} onClick={submitClaim}>837 Submit Claim</button>
            <button disabled={!claim || busy} onClick={payClaim}>835 Pay Claim</button>
          </div>
          {authorizationTransaction && (
            <div className="auth-review-card">
              <div>
                <span className="eyebrow">Prior Auth Review</span>
                <strong>278 · {authorizationTransaction.status}</strong>
                <code>{authorizationTransaction.id}</code>
              </div>
              <p>Manual council review can approve or deny the pending resurrection authorization before the worker decides.</p>
              <div className="actions compact-actions">
                <button disabled={busy} onClick={attachAuthorizationDocumentation}>Send 275 Auth Docs</button>
                <button disabled={busy || authorizationTransaction.status !== "Pending"} onClick={() => decideAuthorization("Approved")}>Approve Auth</button>
                <button className="danger" disabled={busy || authorizationTransaction.status !== "Pending"} onClick={() => decideAuthorization("Denied")}>Deny Auth</button>
              </div>
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
      )}

      {activeTab === "xml" && (
      <section className="panel ledger">
        <div className="ledger-title">
          <h2>XML Intake Audits</h2>
          <span className="muted">from edi-intake</span>
        </div>
        {inboundMessages.length === 0 ? (
          <p className="muted">No XML intake messages match the current filters.</p>
        ) : (
          inboundMessages.map((message) => (
            <InboundMessageRow key={message.id} message={message} onSelect={openInboundMessageDetail} />
          ))
        )}
        <Pager page={auditPage} onPrevious={() => setAuditOffset(Math.max(0, auditPage.offset - auditPage.limit))} onNext={() => setAuditOffset(auditPage.offset + auditPage.limit)} />
      </section>
      )}

      {(selectedClaim || selectedTransaction || selectedInboundMessage) && (
        <div className="drawer-backdrop" onClick={closeDetail}>
        <aside className="detail-drawer" onClick={(event) => event.stopPropagation()} aria-label="Selected record details">
          <div className="ledger-title">
            <div>
              <p className="eyebrow">Selected Record</p>
              <h2>{selectedTransaction ? "Transaction Detail" : selectedInboundMessage ? "XML Intake Detail" : "Claim Detail"}</h2>
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
                  <DetailItem label="Document URL" value={payloadString(selectedTransaction, "documentReferenceUrl") ?? "—"} />
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
              />
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
            </div>
          )}
          {selectedInboundMessage && (
            <div className="detail-grid">
              <div className="detail-actions">
                <button className="secondary" onClick={() => downloadFromPath(`/v1/x12/messages/${selectedInboundMessage.id}/export?format=xml`)}>Export XML</button>
                <button className="secondary" onClick={() => downloadFromPath(`/v1/x12/messages/${selectedInboundMessage.id}/export?format=json`)}>Export JSON</button>
                <button disabled={busy} onClick={() => replayInboundMessage(selectedInboundMessage.id)}>Replay XML</button>
              </div>
              <DetailItem label="Status" value={selectedInboundMessage.status} />
              <DetailItem label="Type" value={selectedInboundMessage.transactionType ?? "—"} />
              <DetailItem label="Downstream" value={selectedInboundMessage.downstreamStatus ? String(selectedInboundMessage.downstreamStatus) : "—"} />
              <DetailItem label="Content Type" value={selectedInboundMessage.contentType} />
              <DetailItem label="Created" value={new Date(selectedInboundMessage.createdAt).toLocaleString()} />
              <DetailItem label="ID" value={selectedInboundMessage.id} />
              {selectedInboundMessage.error && <DetailItem label="Error" value={selectedInboundMessage.error} />}
              <PayloadBlock
                title="Raw XML"
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
      {profile && (profile.attachmentTypes?.length || profile.contentTypes?.length || profile.maxEmbeddedContentBytes) ? (
        <p>
          Guide: {profile.attachmentTypes?.length ? `${profile.attachmentTypes.join("/")} attachments` : "standard attachments"}
          {profile.contentTypes?.length ? ` · ${profile.contentTypes.join(", ")}` : ""}
          {profile.maxEmbeddedContentBytes ? ` · ${Math.round(profile.maxEmbeddedContentBytes / 1024)} KB embedded limit` : ""}
        </p>
      ) : null}
      <div className="actions compact-actions">
        <button className="secondary" disabled={busy} onClick={() => onEdit(partner)}>Edit</button>
        <button className="danger" disabled={busy} onClick={() => onDelete(partner.id)}>Delete</button>
      </div>
    </article>
  );
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

function DocumentationWorkbench({
  claim,
  checklist,
  attachmentTransactions,
  busy,
  onReview
}: {
  claim: Claim;
  checklist: DocumentationChecklistItem[];
  attachmentTransactions: Transaction[];
  busy: boolean;
  onReview: (transactionId: string, status: "Accepted" | "Rejected") => void;
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

function TimelineGroupCard({ group, onSelect }: { group: TimelineGroup; onSelect: (transactionId: string) => void }) {
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

function timelineTitle(transaction: Transaction, claimId?: string, adventurerId?: string) {
  if (claimId) return "Claim lifecycle";
  const adventurerName = payloadNestedString(transaction, "adventurer", "name");
  if (adventurerName) return `Adventurer lifecycle: ${adventurerName}`;
  if (adventurerId) return "Adventurer lifecycle";
  if (transaction.type === "999") return "Implementation acknowledgment";
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
  const packetSequence = payloadString(transaction, "packetSequence");
  const packetCount = payloadString(transaction, "packetCount");
  if (!packetId) return undefined;
  if (packetSequence && packetCount) return `${packetId} (${packetSequence}/${packetCount})`;
  return packetId;
}

function transactionClaimId(transaction: Transaction) {
  return payloadString(transaction, "claimId") ?? payloadNestedString(transaction, "claim", "id");
}

function transactionAdventurerId(transaction: Transaction) {
  return payloadString(transaction, "adventurerId") ?? payloadNestedString(transaction, "claim", "adventurerId") ?? payloadNestedString(transaction, "adventurer", "id");
}

function payloadString(transaction: Transaction, key: string) {
  const payload = payloadRecord(transaction.payload);
  const value = payload?.[key];
  return typeof value === "string" && value ? value : undefined;
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

function InboundMessageRow({ message, onSelect }: { message: InboundMessage; onSelect: (message: InboundMessage) => void }) {
  return (
    <article className="compact-row clickable" onClick={() => onSelect(message)}>
      <div>
        <strong>{message.transactionType || "unknown"} · {message.status}</strong>
        <span>{message.downstreamStatus ? `${message.downstreamStatus} · ` : ""}{new Date(message.createdAt).toLocaleString()}</span>
      </div>
      <code>{message.id}</code>
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
