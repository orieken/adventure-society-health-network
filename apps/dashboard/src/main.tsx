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
  amountCents: number;
  allowedAmountCents?: number;
  paidAmountCents?: number;
  patientResponsibilityCents?: number;
  adjustmentAmountCents?: number;
  adjustmentReason?: string;
  denialReason?: string;
  status: string;
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

const apiUrl = import.meta.env.VITE_ASHN_API_URL ?? "http://localhost:8080";
const adventurerPageSize = 10;
const claimPageSize = 10;
const transactionPageSize = 25;
const auditPageSize = 10;
const dashboardRefreshMs = 3000;
const transactionTypes = ["All", "834", "820", "270", "271", "278", "837", "835", "276", "277", "269", "999", "277CA"];
const transactionStatuses = ["All", "Created", "Dispatched", "Accepted", "Pending", "Approved", "Denied", "Paid", "Failed"];
const claimStatuses = ["All", "Submitted", "Pending", "Approved", "Denied", "Paid"];
const auditStatuses = ["All", "accepted", "rejected"];

function providerLabel(providerId: string, providers: Provider[]) {
  if (providerId === "All") return "All";
  return providers.find((provider) => provider.id === providerId)?.name ?? providerId;
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
  const [events, setEvents] = useState<Envelope[]>([]);
  const [busy, setBusy] = useState(false);
  const [searchTerm, setSearchTerm] = useState("");
  const [transactionTypeFilter, setTransactionTypeFilter] = useState("All");
  const [transactionStatusFilter, setTransactionStatusFilter] = useState("All");
  const [claimStatusFilter, setClaimStatusFilter] = useState("All");
  const [providerFilter, setProviderFilter] = useState("All");
  const [auditStatusFilter, setAuditStatusFilter] = useState("All");
  const [auditTypeFilter, setAuditTypeFilter] = useState("All");

  const selectedProvider = useMemo(
    () => providers.find((provider) => provider.id === selectedProviderId),
    [providers, selectedProviderId]
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
    const [healthResult, providersResult, partnersResult, adventurersResult, claimsResult, transactionsResult, auditResult] = await Promise.all([
      request<Record<string, string>>("/v1/health"),
      request<Provider[]>("/v1/providers"),
      request<TradingPartner[]>("/v1/x12/trading-partners"),
      request<Adventurer[]>(`/v1/adventurers?${adventurerQuery}`),
      request<Claim[]>(`/v1/claims?${claimQuery}`),
      request<Transaction[]>(`/v1/transactions?${transactionQuery}`),
      request<InboundMessage[]>(`/v1/x12/messages?${auditQuery}`)
    ]);
    setHealth(healthResult);
    setProviders(providersResult.data ?? []);
    setTradingPartners(partnersResult.data ?? []);
    setRecentAdventurers(adventurersResult.data ?? []);
    setRecentClaims(claimsResult.data ?? []);
    setRecentTransactions(transactionsResult.data ?? []);
    setInboundMessages(auditResult.data ?? []);
    setAdventurerPage(adventurersResult.page ?? { limit: adventurerPageSize, offset: adventurerOffset, count: adventurersResult.data?.length ?? 0, hasMore: false });
    setClaimPage(claimsResult.page ?? { limit: claimPageSize, offset: claimOffset, count: claimsResult.data?.length ?? 0, hasMore: false });
    setTransactionPage(transactionsResult.page ?? { limit: transactionPageSize, offset: transactionOffset, count: transactionsResult.data?.length ?? 0, hasMore: false });
    setAuditPage(auditResult.page ?? { limit: auditPageSize, offset: auditOffset, count: auditResult.data?.length ?? 0, hasMore: false });
    if (pushProviderEvent && providersResult.lore) {
      pushEvent(providersResult);
    }
  }

  function resetLedgerOffsets() {
    setAdventurerOffset(0);
    setClaimOffset(0);
    setTransactionOffset(0);
    setAuditOffset(0);
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

  function downloadFromPath(path: string) {
    const anchor = document.createElement("a");
    anchor.href = `${apiUrl}${path}`;
    document.body.appendChild(anchor);
    anchor.click();
    anchor.remove();
  }

  async function replayTransaction(transactionId: string) {
    setBusy(true);
    const result = await request<Transaction>(`/v1/transactions/${transactionId}/replay`, { method: "POST" });
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
    const result = await request("/v1/auth-requests", {
      method: "POST",
      body: JSON.stringify({
        adventurerId: adventurer.id,
        providerId: selectedProviderId,
        serviceType: "resurrection",
        incidentSeverity: "Diamond"
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
        amountCents: 125000
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
      setSelectedClaim(null);
      setSelectedInboundMessage(null);
    }
    setBusy(false);
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
          <h2>Gateway</h2>
          <p>{apiUrl}</p>
          <div className="health-grid">
            {Object.entries(health?.data ?? {}).map(([service, status]) => (
              <span key={service} className={status === "ok" ? "ok" : "bad"}>
                {service}: {status}
              </span>
            ))}
          </div>
        </div>
      </section>

      <section className="stats-grid">
        <MetricCard label="Adventurers" value={adventurerPage.count} detail={pageSummary(adventurerPage)} />
        <MetricCard label="Claims" value={claimPage.count} detail={`${recentClaims.filter((item) => item.status === "Paid").length} paid on this page`} />
        <MetricCard label="Transactions" value={transactionPage.count} detail={`${recentTransactions.length} ledger entries loaded`} />
      </section>

      <section className="panel trading-panel">
        <div className="ledger-title">
          <div>
            <h2>Trading Partners</h2>
            <p className="muted">Sender/receiver IDs, allowed X12 types, and current routing targets.</p>
          </div>
          <span className="muted">{tradingPartners.length} profiles</span>
        </div>
        <div className="partner-grid">
          {tradingPartners.length === 0 ? (
            <p className="muted">No trading partner profiles are loaded.</p>
          ) : (
            tradingPartners.map((partner) => <TradingPartnerCard key={partner.id} partner={partner} />)
          )}
        </div>
      </section>

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

      <section className="panel filters-panel">
        <div className="ledger-title">
          <h2>Search & Filters</h2>
          <button
            className="secondary"
            onClick={() => {
              setSearchTerm("");
              setTransactionTypeFilter("All");
              setTransactionStatusFilter("All");
              setClaimStatusFilter("All");
              setProviderFilter("All");
              setAuditStatusFilter("All");
              setAuditTypeFilter("All");
              resetLedgerOffsets();
            }}
          >
            Clear
          </button>
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

      <section className="history-grid">
        <div className="panel ledger">
          <div className="ledger-title">
            <h2>Persisted Transactions</h2>
            <span className="muted">from Postgres</span>
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

      <section className="panel ledger">
        <div className="ledger-title">
          <h2>XML Intake Audits</h2>
          <span className="muted">from edi-intake</span>
        </div>
        {inboundMessages.length === 0 ? (
          <p className="muted">No XML intake messages match the current filters.</p>
        ) : (
          inboundMessages.map((message) => (
            <InboundMessageRow key={message.id} message={message} onSelect={(item) => {
              setSelectedInboundMessage(item);
              setSelectedClaim(null);
              setSelectedTransaction(null);
            }} />
          ))
        )}
        <Pager page={auditPage} onPrevious={() => setAuditOffset(Math.max(0, auditPage.offset - auditPage.limit))} onNext={() => setAuditOffset(auditPage.offset + auditPage.limit)} />
      </section>

      {(selectedClaim || selectedTransaction || selectedInboundMessage) && (
        <section className="panel detail-panel">
          <div className="ledger-title">
            <h2>{selectedTransaction ? "Transaction Detail" : selectedInboundMessage ? "XML Intake Detail" : "Claim Detail"}</h2>
            <button className="secondary" onClick={() => {
              setSelectedClaim(null);
              setSelectedTransaction(null);
              setSelectedInboundMessage(null);
            }}>
              Close
            </button>
          </div>
          {selectedTransaction && (
            <div className="detail-grid">
              <div className="detail-actions">
                <button className="secondary" onClick={() => downloadFromPath(`/v1/transactions/${selectedTransaction.id}/export?format=json`)}>Export JSON</button>
                <button className="secondary" onClick={() => downloadFromPath(`/v1/transactions/${selectedTransaction.id}/export?format=xml`)}>Export XML</button>
                <button className="secondary" disabled={!selectedTransaction.rawX12} onClick={() => downloadFromPath(`/v1/transactions/${selectedTransaction.id}/export?format=x12`)}>Export X12</button>
                <button disabled={busy} onClick={() => replayTransaction(selectedTransaction.id)}>Replay Transaction</button>
              </div>
              <DetailItem label="Type" value={selectedTransaction.type} />
              <DetailItem label="Status" value={selectedTransaction.status} />
              <DetailItem label="Sender" value={selectedTransaction.senderId} />
              <DetailItem label="Receiver" value={selectedTransaction.receiverId} />
              <DetailItem label="Created" value={new Date(selectedTransaction.createdAt).toLocaleString()} />
              <DetailItem label="ID" value={selectedTransaction.id} />
              <DetailItem label="Related" value={selectedTransaction.relatedId ?? "—"} />
              <PayloadBlock
                title="Raw X12"
                value={selectedTransaction.rawX12 ?? "No raw X12 was generated for this transaction."}
                onCopy={copyText}
                downloadLabel="Download .x12"
                onDownload={() => downloadText(`ashn-${selectedTransaction.type}-${selectedTransaction.id}.x12`, selectedTransaction.rawX12 ?? "")}
                canDownload={Boolean(selectedTransaction.rawX12)}
              />
              <PayloadBlock
                title="JSON Payload"
                value={JSON.stringify(selectedTransaction.payload, null, 2)}
                onCopy={copyText}
              />
            </div>
          )}
          {selectedClaim && (
            <div className="detail-grid">
              <DetailItem label="Status" value={selectedClaim.status} />
              <DetailItem label="Severity" value={selectedClaim.incidentSeverity} />
              <DetailItem label="Billed" value={money(selectedClaim.amountCents)} />
              <DetailItem label="Allowed" value={money(selectedClaim.allowedAmountCents)} />
              <DetailItem label="Paid" value={money(selectedClaim.paidAmountCents)} />
              <DetailItem label="Patient Resp." value={money(selectedClaim.patientResponsibilityCents)} />
              <DetailItem label="Adjustment" value={money(selectedClaim.adjustmentAmountCents)} />
              <DetailItem label="Adjustment Reason" value={selectedClaim.adjustmentReason ?? "—"} />
              <DetailItem label="Denial Reason" value={selectedClaim.denialReason ?? "—"} />
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
        </section>
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

function TradingPartnerCard({ partner }: { partner: TradingPartner }) {
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
            <small>{transaction.status}</small>
            <em>{new Date(transaction.createdAt).toLocaleTimeString()}</em>
          </button>
        ))}
      </div>
    </article>
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
